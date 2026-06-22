package model

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// BalanceNotifyConfig holds notification threshold settings
type BalanceNotifyConfig struct {
	Enabled       bool    `json:"enabled"`
	Threshold     float64 `json:"threshold"`       // notify when balance falls below this (in USD)
	ThresholdPct  float64 `json:"threshold_pct"`   // notify when usage exceeds this % of quota
	NotifyMethod  string  `json:"notify_method"`   // "email", "webhook", "internal"
	WebhookURL    string  `json:"webhook_url"`
	CooldownHours int     `json:"cooldown_hours"`  // min hours between notifications per user
}

var (
	balanceNotifyConfig   BalanceNotifyConfig
	balanceNotifyConfigMu sync.RWMutex
	lastNotifyTimes       = make(map[int]time.Time) // userID -> last notify time
	lastNotifyTimesMu     sync.Mutex
)

func init() {
	balanceNotifyConfig = BalanceNotifyConfig{
		Enabled:       false,
		Threshold:     1.0, // $1.00 default
		ThresholdPct:  90.0,
		NotifyMethod:  "internal",
		CooldownHours: 24,
	}
}

// SetBalanceNotifyConfig updates the config
func SetBalanceNotifyConfig(cfg BalanceNotifyConfig) {
	balanceNotifyConfigMu.Lock()
	defer balanceNotifyConfigMu.Unlock()
	balanceNotifyConfig = cfg
}

// GetBalanceNotifyConfig returns the current config
func GetBalanceNotifyConfig() BalanceNotifyConfig {
	balanceNotifyConfigMu.RLock()
	defer balanceNotifyConfigMu.RUnlock()
	return balanceNotifyConfig
}

// CheckAndNotifyBalance checks if user's balance is below threshold and sends notification
func CheckAndNotifyBalance(userID int, currentBalance float64, totalQuota float64) {
	cfg := GetBalanceNotifyConfig()
	if !cfg.Enabled {
		return
	}

	shouldNotify := false
	reason := ""

	if cfg.Threshold > 0 && currentBalance < cfg.Threshold {
		shouldNotify = true
		reason = fmt.Sprintf("balance $%.2f below threshold $%.2f", currentBalance, cfg.Threshold)
	}

	if cfg.ThresholdPct > 0 && totalQuota > 0 {
		usagePct := (1 - currentBalance/totalQuota) * 100
		if usagePct >= cfg.ThresholdPct {
			shouldNotify = true
			reason = fmt.Sprintf("usage %.1f%% exceeds threshold %.1f%%", usagePct, cfg.ThresholdPct)
		}
	}

	if !shouldNotify {
		return
	}

	// Check cooldown
	lastNotifyTimesMu.Lock()
	lastTime, exists := lastNotifyTimes[userID]
	if exists && time.Since(lastTime) < time.Duration(cfg.CooldownHours)*time.Hour {
		lastNotifyTimesMu.Unlock()
		return
	}
	lastNotifyTimes[userID] = time.Now()
	lastNotifyTimesMu.Unlock()

	// Send notification
	common.SysLog(fmt.Sprintf("[BalanceNotify] user=%d %s", userID, reason))

	if cfg.NotifyMethod == "webhook" && cfg.WebhookURL != "" {
		go sendWebhookNotification(cfg.WebhookURL, userID, reason, currentBalance)
	}
}

func sendWebhookNotification(webhookURL string, userID int, reason string, balance float64) {
	// Simplified webhook - in production use proper HTTP client
	common.SysLog(fmt.Sprintf("[BalanceNotify] webhook -> %s user=%d reason=%s balance=%.2f",
		webhookURL, userID, reason, balance))
}
