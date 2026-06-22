package model

import (
	"sync"
	"time"
)

// ChannelHealthRecord stores a single health check result
type ChannelHealthRecord struct {
	ID        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	ChannelID int    `json:"channel_id" gorm:"index;not null"`
	Model     string `json:"model" gorm:"type:varchar(128);index"`
	Status    string `json:"status" gorm:"type:varchar(32)"` // healthy, degraded, down
	StatusCode int   `json:"status_code"`
	LatencyMs  int   `json:"latency_ms"`
	Error      string `json:"error" gorm:"type:text"`
	CheckedAt  int64  `json:"checked_at" gorm:"autoCreateTime"`
}

func (ChannelHealthRecord) TableName() string {
	return "channel_health_records"
}

// ChannelHealthSummary is the aggregated health status for a channel
type ChannelHealthSummary struct {
	ChannelID      int     `json:"channel_id"`
	ChannelName    string  `json:"channel_name"`
	Status         string  `json:"status"`          // healthy, degraded, down
	SuccessRate    float64 `json:"success_rate"`     // 0.0 - 1.0
	AvgLatencyMs   int     `json:"avg_latency_ms"`
	LastCheckedAt  int64   `json:"last_checked_at"`
	RecentErrors   int     `json:"recent_errors"`    // errors in last hour
	TotalChecks    int     `json:"total_checks"`     // checks in last hour
}

// In-memory health cache for fast lookups
var (
	healthCache   = make(map[int]*ChannelHealthSummary) // channelID -> summary
	healthCacheMu sync.RWMutex
)

// GetChannelHealth returns the cached health summary for a channel
func GetChannelHealth(channelID int) *ChannelHealthSummary {
	healthCacheMu.RLock()
	defer healthCacheMu.RUnlock()
	if s, ok := healthCache[channelID]; ok {
		return s
	}
	return &ChannelHealthSummary{ChannelID: channelID, Status: "unknown"}
}

// GetAllChannelHealth returns all channel health summaries
func GetAllChannelHealth() []*ChannelHealthSummary {
	healthCacheMu.RLock()
	defer healthCacheMu.RUnlock()
	result := make([]*ChannelHealthSummary, 0, len(healthCache))
	for _, s := range healthCache {
		result = append(result, s)
	}
	return result
}

// RecordHealthCheck records a health check result and updates the cache
func RecordHealthCheck(channelID int, channelName string, model string, statusCode int, latencyMs int, errStr string) {
	// Save to DB (async-friendly)
	record := ChannelHealthRecord{
		ChannelID:  channelID,
		Model:      model,
		StatusCode: statusCode,
		LatencyMs:  latencyMs,
		Error:      errStr,
	}
	if statusCode >= 200 && statusCode < 400 {
		record.Status = "healthy"
	} else if statusCode >= 500 {
		record.Status = "down"
	} else {
		record.Status = "degraded"
	}
	DB.Create(&record)

	// Update in-memory cache
	healthCacheMu.Lock()
	defer healthCacheMu.Unlock()

	summary, ok := healthCache[channelID]
	if !ok {
		summary = &ChannelHealthSummary{ChannelID: channelID, ChannelName: channelName}
		healthCache[channelID] = summary
	}

	summary.LastCheckedAt = time.Now().Unix()
	summary.TotalChecks++
	if statusCode >= 400 {
		summary.RecentErrors++
	}

	// Calculate success rate
	if summary.TotalChecks > 0 {
		summary.SuccessRate = float64(summary.TotalChecks-summary.RecentErrors) / float64(summary.TotalChecks)
	}

	// Determine overall status
	if summary.SuccessRate >= 0.9 {
		summary.Status = "healthy"
	} else if summary.SuccessRate >= 0.5 {
		summary.Status = "degraded"
	} else {
		summary.Status = "down"
	}

	// Update average latency
	if summary.AvgLatencyMs == 0 {
		summary.AvgLatencyMs = latencyMs
	} else {
		summary.AvgLatencyMs = (summary.AvgLatencyMs + latencyMs) / 2
	}
}

// CleanupOldHealthRecords removes records older than 24 hours
func CleanupOldHealthRecords() {
	cutoff := time.Now().Add(-24 * time.Hour).Unix()
	DB.Where("checked_at < ?", cutoff).Delete(&ChannelHealthRecord{})

	// Reset counters in cache
	healthCacheMu.Lock()
	defer healthCacheMu.Unlock()
	for _, s := range healthCache {
		s.TotalChecks = 0
		s.RecentErrors = 0
		s.SuccessRate = 1.0
	}
}

// StartHealthCheckScheduler runs periodic health checks
func StartHealthCheckScheduler() {
	go func() {
		// Cleanup every hour
		cleanupTicker := time.NewTicker(1 * time.Hour)
		defer cleanupTicker.Stop()
		for range cleanupTicker.C {
			CleanupOldHealthRecords()
		}
	}()
}

func init() {
	StartHealthCheckScheduler()
}
