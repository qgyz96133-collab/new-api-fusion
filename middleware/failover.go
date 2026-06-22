package middleware

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
)

// FailoverConfig holds failover settings
type FailoverConfig struct {
	MaxRetries      int           // max retry attempts on same channel
	MaxSwitches     int           // max channel switches
	RetryDelay      time.Duration // base delay between same-channel retries
	SwitchDelay     time.Duration // base delay between channel switches
	RetryableStatus map[int]bool  // HTTP status codes that trigger retry
	// Exponential backoff settings
	BackoffMultiplier float64       // delay multiplier per retry (e.g. 2.0)
	MaxBackoffDelay   time.Duration // cap on backoff delay
}

// DefaultFailoverConfig returns sensible defaults
func DefaultFailoverConfig() FailoverConfig {
	return FailoverConfig{
		MaxRetries:  2,
		MaxSwitches: 3,
		RetryDelay:  500 * time.Millisecond,
		SwitchDelay: 200 * time.Millisecond,
		RetryableStatus: map[int]bool{
			http.StatusServiceUnavailable: true, // 503
			http.StatusGatewayTimeout:     true, // 504
			http.StatusTooManyRequests:    true, // 429
			529:                           true, // Anthropic overloaded
			http.StatusBadGateway:         true, // 502
			http.StatusInternalServerError: true, // 500 (upstream transient)
		},
		BackoffMultiplier: 2.0,
		MaxBackoffDelay:   10 * time.Second,
	}
}

// FailoverAction represents the next action after an error
type FailoverAction int

const (
	FailoverRetry      FailoverAction = iota // retry same channel
	FailoverSwitch                           // switch to different channel
	FailoverExhausted                        // no more retries
	FailoverCanceled                         // context canceled
)

// FailoverState tracks retry state across iterations
type FailoverState struct {
	RetryCount     int
	SwitchCount    int
	FailedChannels map[int]struct{}
	LastError      error
	Config         FailoverConfig
	// Same-account retry support (ported from sub2api)
	SameAccountRetryCount map[int]int
	MaxSameAccountRetries int
	// Backoff state
	consecutiveFailures int
	// Cache billing flag (for sticky session failover)
	ForceCacheBilling bool
}

// NewFailoverState creates a new failover state
func NewFailoverState(cfg FailoverConfig) *FailoverState {
	return &FailoverState{
		FailedChannels:        make(map[int]struct{}),
		SameAccountRetryCount: make(map[int]int),
		Config:                cfg,
		MaxSameAccountRetries: 3,
	}
}

// backoffDelay calculates exponential backoff delay with jitter
func (s *FailoverState) backoffDelay(baseDelay time.Duration) time.Duration {
	delay := float64(baseDelay) * math.Pow(s.Config.BackoffMultiplier, float64(s.consecutiveFailures))
	if delay > float64(s.Config.MaxBackoffDelay) {
		delay = float64(s.Config.MaxBackoffDelay)
	}
	// Add 10% jitter
	jitter := delay * 0.1
	delay += jitter * float64(time.Now().UnixNano()%2-1)
	if delay < 0 {
		delay = float64(baseDelay)
	}
	return time.Duration(delay)
}

// isRetryableStatus checks if the status code should trigger failover
func (s *FailoverState) isRetryableStatus(statusCode int) bool {
	return s.Config.RetryableStatus[statusCode]
}

// isTransientServerError checks if a 500 error is likely transient
func isTransientServerError(statusCode int) bool {
	return statusCode == http.StatusInternalServerError ||
		statusCode == http.StatusBadGateway ||
		statusCode == http.StatusServiceUnavailable ||
		statusCode == http.StatusGatewayTimeout
}

// HandleError processes an upstream error and returns the next action
func (s *FailoverState) HandleError(ctx context.Context, channelID int, statusCode int, err error) FailoverAction {
	s.LastError = err
	s.consecutiveFailures++

	// Check if context is canceled
	select {
	case <-ctx.Done():
		return FailoverCanceled
	default:
	}

	// Check if this status code is retryable
	if !s.isRetryableStatus(statusCode) {
		return FailoverExhausted
	}

	// Same-channel retry (with exponential backoff)
	if s.RetryCount < s.Config.MaxRetries {
		s.RetryCount++
		delay := s.backoffDelay(s.Config.RetryDelay)
		common.SysLog(fmt.Sprintf("[Failover] retry %d/%d on channel %d (status=%d, backoff=%v)",
			s.RetryCount, s.Config.MaxRetries, channelID, statusCode, delay))

		if !sleepWithContext(ctx, delay) {
			return FailoverCanceled
		}
		return FailoverRetry
	}

	// Mark channel as failed
	s.FailedChannels[channelID] = struct{}{}

	// Channel switch (with exponential backoff)
	if s.SwitchCount < s.Config.MaxSwitches {
		s.SwitchCount++
		s.RetryCount = 0 // reset retry count for new channel
		delay := s.backoffDelay(s.Config.SwitchDelay)
		common.SysLog(fmt.Sprintf("[Failover] switch %d/%d away from channel %d (status=%d, backoff=%v)",
			s.SwitchCount, s.Config.MaxSwitches, channelID, statusCode, delay))

		if !sleepWithContext(ctx, delay) {
			return FailoverCanceled
		}
		return FailoverSwitch
	}

	return FailoverExhausted
}

// HandleSameAccountRetry attempts to retry on the same account before switching.
// Ported from sub2api's failover_loop.go: same-account retry for transient errors.
func (s *FailoverState) HandleSameAccountRetry(ctx context.Context, accountID int, statusCode int) FailoverAction {
	if !isTransientServerError(statusCode) {
		return FailoverExhausted
	}
	if s.SameAccountRetryCount[accountID] >= s.MaxSameAccountRetries {
		return FailoverExhausted
	}
	s.SameAccountRetryCount[accountID]++
	delay := time.Duration(s.SameAccountRetryCount[accountID]) * 500 * time.Millisecond
	common.SysLog(fmt.Sprintf("[Failover] same-account retry %d/%d on account %d (status=%d, delay=%v)",
		s.SameAccountRetryCount[accountID], s.MaxSameAccountRetries, accountID, statusCode, delay))

	if !sleepWithContext(ctx, delay) {
		return FailoverCanceled
	}
	return FailoverRetry
}

// ResetAfterSuccess resets failure counters after a successful request
func (s *FailoverState) ResetAfterSuccess() {
	s.consecutiveFailures = 0
}

// GetFailedChannels returns the set of failed channel IDs
func (s *FailoverState) GetFailedChannels() map[int]struct{} {
	return s.FailedChannels
}

// ClearFailedChannels resets the failed channels list (for single-account backoff retry)
func (s *FailoverState) ClearFailedChannels() {
	s.FailedChannels = make(map[int]struct{})
}

// IsRetryableError checks if an HTTP status code should trigger failover
func IsRetryableError(statusCode int) bool {
	cfg := DefaultFailoverConfig()
	return cfg.RetryableStatus[statusCode]
}

// sleepWithContext waits for the specified duration, returning false if context is canceled
func sleepWithContext(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// FailoverMiddleware adds failover context to requests
func FailoverMiddleware() gin.HandlerFunc {
	cfg := DefaultFailoverConfig()
	return func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyFailoverMaxRetries, cfg.MaxRetries)
		common.SetContextKey(c, constant.ContextKeyFailoverMaxSwitches, cfg.MaxSwitches)
		c.Next()
	}
}
