package model

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

// ChannelScore tracks per-channel performance metrics for intelligent selection
// Ported from AIClient2API's provider pool scoring system
type ChannelScore struct {
	ChannelID      int
	ErrorCount     int       // recent errors
	SuccessCount   int       // recent successes
	TotalLatencyMs int64     // cumulative latency for averaging
	LastUsed       time.Time // LRU tracking
	SelectionSeq   int64     // monotonic sequence for concurrent selection
	CooldownUntil  time.Time // 429 cooldown expiry
	Consecutive429 int       // consecutive 429 errors for exponential backoff
}

var (
	channelScores   = make(map[int]*ChannelScore)
	channelScoresMu sync.RWMutex
	selectionSeq    int64
	selectionSeqMu  sync.Mutex
)

// GetChannelScore returns or creates a score record for a channel
func GetChannelScore(channelID int) *ChannelScore {
	channelScoresMu.RLock()
	score, ok := channelScores[channelID]
	channelScoresMu.RUnlock()
	if ok {
		return score
	}

	channelScoresMu.Lock()
	defer channelScoresMu.Unlock()

	// Double-check after acquiring write lock
	if score, ok = channelScores[channelID]; ok {
		return score
	}

	score = &ChannelScore{
		ChannelID: channelID,
		LastUsed:  time.Now().Add(-time.Hour), // old timestamp so unused channels are preferred
	}
	channelScores[channelID] = score
	return score
}

// CalculateScore computes a selection score for a channel
// Lower score = better candidate for selection
// Ported from AIClient2API's _calculateNodeScore
func (s *ChannelScore) CalculateScore(baseWeight int) float64 {
	now := time.Now()

	// If in cooldown, return very high score (avoid selection)
	if now.Before(s.CooldownUntil) {
		remaining := s.CooldownUntil.Sub(now).Seconds()
		return 100000 + remaining
	}

	score := 0.0

	// Factor 1: Error rate penalty (0-50 points)
	total := s.ErrorCount + s.SuccessCount
	if total > 0 {
		errorRate := float64(s.ErrorCount) / float64(total)
		score += errorRate * 50
	}

	// Factor 2: Average latency penalty (0-30 points)
	if s.SuccessCount > 0 && s.TotalLatencyMs > 0 {
		avgLatency := float64(s.TotalLatencyMs) / float64(s.SuccessCount)
		// Normalize: 0ms=0pts, 10000ms+=30pts
		score += math.Min(avgLatency/10000*30, 30)
	}

	// Factor 3: LRU bonus - recently used channels get slight penalty (0-10 points)
	timeSinceUse := now.Sub(s.LastUsed).Seconds()
	if timeSinceUse < 5 {
		score += 10 // heavily penalize very recently used (prefer other channels)
	} else if timeSinceUse < 30 {
		score += 5
	}

	// Factor 4: Base weight (inverted - higher weight = lower score)
	score -= float64(baseWeight) * 0.1

	return score
}

// RecordSuccess records a successful request
func (s *ChannelScore) RecordSuccess(latencyMs int64) {
	channelScoresMu.Lock()
	defer channelScoresMu.Unlock()

	s.SuccessCount++
	s.TotalLatencyMs += latencyMs
	s.LastUsed = time.Now()
	s.Consecutive429 = 0

	selectionSeqMu.Lock()
	selectionSeq++
	s.SelectionSeq = selectionSeq
	selectionSeqMu.Unlock()

	// Decay old error counts (keep last ~100 requests)
	if s.SuccessCount+s.ErrorCount > 100 {
		s.ErrorCount = s.ErrorCount * 9 / 10
		s.SuccessCount = s.SuccessCount * 9 / 10
		s.TotalLatencyMs = s.TotalLatencyMs * 9 / 10
	}
}

// RecordError records a failed request
func (s *ChannelScore) RecordError(statusCode int) {
	channelScoresMu.Lock()
	defer channelScoresMu.Unlock()

	s.ErrorCount++
	s.LastUsed = time.Now()

	selectionSeqMu.Lock()
	selectionSeq++
	s.SelectionSeq = selectionSeq
	selectionSeqMu.Unlock()

	// Handle 429 with exponential backoff + jitter
	// Ported from AIClient2API's RATE_LIMIT_COOLDOWN system
	if statusCode == 429 {
		s.Consecutive429++
		baseDelay := time.Duration(s.Consecutive429) * 5 * time.Second
		maxDelay := 5 * time.Minute
		if baseDelay > maxDelay {
			baseDelay = maxDelay
		}
		// Add random jitter (±25%)
		jitter := time.Duration(rand.Int63n(int64(baseDelay) / 2))
		if rand.Intn(2) == 0 {
			jitter = -jitter
		}
		cooldownDuration := baseDelay + jitter
		if cooldownDuration < time.Second {
			cooldownDuration = time.Second
		}
		s.CooldownUntil = time.Now().Add(cooldownDuration)
	} else if statusCode >= 500 {
		// Server errors: shorter cooldown
		s.CooldownUntil = time.Now().Add(10 * time.Second)
	}
}

// IsInCooldown checks if a channel is currently in cooldown
func (s *ChannelScore) IsInCooldown() bool {
	return time.Now().Before(s.CooldownUntil)
}

// CooldownRemaining returns remaining cooldown duration
func (s *ChannelScore) CooldownRemaining() time.Duration {
	remaining := time.Until(s.CooldownUntil)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// SelectBestChannel picks the channel with the lowest score from candidates
// Ported from AIClient2API's selection algorithm with mutex-protected atomic selection
func SelectBestChannel(candidates []struct {
	ID     int
	Weight int
}) int {
	if len(candidates) == 0 {
		return 0
	}
	if len(candidates) == 1 {
		return candidates[0].ID
	}

	bestID := candidates[0].ID
	bestScore := math.MaxFloat64

	for _, c := range candidates {
		score := GetChannelScore(c.ID)
		computed := score.CalculateScore(c.Weight)
		if computed < bestScore {
			bestScore = computed
			bestID = c.ID
		}
	}

	return bestID
}

// GetAllChannelScores returns all tracked scores (for monitoring)
func GetAllChannelScores() map[int]*ChannelScore {
	channelScoresMu.RLock()
	defer channelScoresMu.RUnlock()
	result := make(map[int]*ChannelScore, len(channelScores))
	for k, v := range channelScores {
		cp := *v
		result[k] = &cp
	}
	return result
}
