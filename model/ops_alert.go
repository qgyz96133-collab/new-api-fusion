package model

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// OpsAlert represents an operational alert
type OpsAlert struct {
	ID        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Type      string `json:"type" gorm:"type:varchar(64)"`      // error_rate, latency, channel_down, quota_low
	Severity  string `json:"severity" gorm:"type:varchar(32)"`  // info, warning, critical
	ChannelID int    `json:"channel_id" gorm:"index"`
	Message   string `json:"message" gorm:"type:text"`
	Value     string `json:"value" gorm:"type:varchar(256)"`    // the metric value that triggered
	Threshold string `json:"threshold" gorm:"type:varchar(256)"` // the configured threshold
	Resolved  bool   `json:"resolved" gorm:"default:false"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
	ResolvedAt int64 `json:"resolved_at"`
}

func (OpsAlert) TableName() string { return "ops_alerts" }

// OpsTrendPoint represents a time-series data point
type OpsTrendPoint struct {
	Timestamp    int64   `json:"timestamp"`
	Window       string  `json:"window"` // "1m", "5m", "1h", "1d"
	Requests     int64   `json:"requests"`
	Errors       int64   `json:"errors"`
	ErrorRate    float64 `json:"error_rate"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	P95LatencyMs float64 `json:"p95_latency_ms"`
	TokensIn     int64   `json:"tokens_in"`
	TokensOut    int64   `json:"tokens_out"`
	Cost         float64 `json:"cost"`
}

// AlertEvaluator checks metrics against thresholds
// Ported from sub2api ops_alert_evaluator_service.go
type AlertEvaluator struct {
	mu         sync.Mutex
	thresholds AlertThresholds
}

type AlertThresholds struct {
	ErrorRateWarn    float64 `json:"error_rate_warn"`    // e.g., 0.05 (5%)
	ErrorRateCrit    float64 `json:"error_rate_crit"`    // e.g., 0.20 (20%)
	LatencyWarnMs    int     `json:"latency_warn_ms"`    // e.g., 5000
	LatencyCritMs    int     `json:"latency_crit_ms"`    // e.g., 30000
	ChannelDownMin   int     `json:"channel_down_min"`   // consecutive errors before alert
}

var globalAlertEvaluator = &AlertEvaluator{
	thresholds: AlertThresholds{
		ErrorRateWarn:  0.05,
		ErrorRateCrit:  0.20,
		LatencyWarnMs:  5000,
		LatencyCritMs:  30000,
		ChannelDownMin: 5,
	},
}

// GetAlertEvaluator returns the global evaluator
func GetAlertEvaluator() *AlertEvaluator {
	return globalAlertEvaluator
}

// SetThresholds updates alert thresholds
func (e *AlertEvaluator) SetThresholds(t AlertThresholds) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.thresholds = t
}

// Evaluate checks a channel's current state and creates alerts if needed
func (e *AlertEvaluator) Evaluate(channelID int, channelName string, errorRate float64, avgLatencyMs float64, consecutiveErrors int) {
	e.mu.Lock()
	t := e.thresholds
	e.mu.Unlock()

	// Error rate check
	if errorRate >= t.ErrorRateCrit {
		createAlert("error_rate", "critical", channelID,
			fmt.Sprintf("Channel %s error rate %.1f%% exceeds critical threshold %.1f%%",
				channelName, errorRate*100, t.ErrorRateCrit*100),
			fmt.Sprintf("%.2f", errorRate), fmt.Sprintf("%.2f", t.ErrorRateCrit))
	} else if errorRate >= t.ErrorRateWarn {
		createAlert("error_rate", "warning", channelID,
			fmt.Sprintf("Channel %s error rate %.1f%% exceeds warning threshold %.1f%%",
				channelName, errorRate*100, t.ErrorRateWarn*100),
			fmt.Sprintf("%.2f", errorRate), fmt.Sprintf("%.2f", t.ErrorRateWarn))
	}

	// Latency check
	if avgLatencyMs >= float64(t.LatencyCritMs) {
		createAlert("latency", "critical", channelID,
			fmt.Sprintf("Channel %s avg latency %.0fms exceeds %dms",
				channelName, avgLatencyMs, t.LatencyCritMs),
			fmt.Sprintf("%.0fms", avgLatencyMs), fmt.Sprintf("%dms", t.LatencyCritMs))
	} else if avgLatencyMs >= float64(t.LatencyWarnMs) {
		createAlert("latency", "warning", channelID,
			fmt.Sprintf("Channel %s avg latency %.0fms exceeds %dms",
				channelName, avgLatencyMs, t.LatencyWarnMs),
			fmt.Sprintf("%.0fms", avgLatencyMs), fmt.Sprintf("%dms", t.LatencyWarnMs))
	}

	// Channel down check
	if consecutiveErrors >= t.ChannelDownMin {
		createAlert("channel_down", "critical", channelID,
			fmt.Sprintf("Channel %s has %d consecutive errors", channelName, consecutiveErrors),
			fmt.Sprintf("%d", consecutiveErrors), fmt.Sprintf("%d", t.ChannelDownMin))
	}
}

func createAlert(alertType, severity string, channelID int, message, value, threshold string) {
	alert := OpsAlert{
		Type:      alertType,
		Severity:  severity,
		ChannelID: channelID,
		Message:   message,
		Value:     value,
		Threshold: threshold,
	}
	DB.Create(&alert)
	common.SysLog(fmt.Sprintf("[OpsAlert] %s/%s: %s", severity, alertType, message))
}

// GetRecentAlerts returns recent alerts
func GetRecentAlerts(limit int) []OpsAlert {
	var alerts []OpsAlert
	DB.Order("created_at DESC").Limit(limit).Find(&alerts)
	return alerts
}

// OpsTrendStore stores trend data in memory with periodic flush
var (
	trendStore   = make(map[string]*OpsTrendPoint) // window_key -> point
	trendStoreMu sync.Mutex
)

// RecordTrend records a request for trend aggregation
func RecordTrend(latencyMs int64, isError bool, tokensIn, tokensOut int64, cost float64) {
	trendStoreMu.Lock()
	defer trendStoreMu.Unlock()

	now := time.Now()
	key := now.Format("2006-01-02T15:04") // 1-minute windows

	point, ok := trendStore[key]
	if !ok {
		point = &OpsTrendPoint{Timestamp: now.Unix(), Window: "1m"}
		trendStore[key] = point
	}

	point.Requests++
	if isError {
		point.Errors++
	}
	point.AvgLatencyMs = (point.AvgLatencyMs*float64(point.Requests-1) + float64(latencyMs)) / float64(point.Requests)
	point.TokensIn += tokensIn
	point.TokensOut += tokensOut
	point.Cost += cost
	point.ErrorRate = float64(point.Errors) / float64(point.Requests)

	// Cleanup old entries (keep last 24h = 1440 minutes)
	if len(trendStore) > 1440 {
		cutoff := now.Add(-24 * time.Hour).Format("2006-01-02T15:04")
		for k := range trendStore {
			if k < cutoff {
				delete(trendStore, k)
			}
		}
	}
}

// GetTrends returns trend data for the specified time range
func GetTrends(minutes int) []OpsTrendPoint {
	trendStoreMu.Lock()
	defer trendStoreMu.Unlock()

	cutoff := time.Now().Add(-time.Duration(minutes) * time.Minute).Format("2006-01-02T15:04")
	var result []OpsTrendPoint

	for k, v := range trendStore {
		if k >= cutoff {
			result = append(result, *v)
		}
	}
	return result
}
