package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/bytedance/gopkg/util/gopool"
)

const (
	monitorCleanupDays = 30
)

// StartChannelMonitorScheduler starts the background scheduler for channel health checks.
// Uses gopool.Go() for goroutine management, consistent with the rest of the codebase.
func StartChannelMonitorScheduler() {
	go func() {
		common.SysLog("[ChannelMonitor] scheduler started")
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			runScheduledChecks()
			cleanupOldHistory()
		}
	}()
}

func runScheduledChecks() {
	monitors, err := model.ListEnabledMonitors()
	if err != nil {
		return
	}

	now := time.Now()
	for _, monitor := range monitors {
		// Skip if not due for check
		if monitor.LastCheckedAt != nil {
			elapsed := now.Sub(*monitor.LastCheckedAt)
			if elapsed < time.Duration(monitor.CheckInterval)*time.Second {
				continue
			}
		}

		m := monitor
		gopool.Go(func() {
			checkMonitor(m)
		})
	}
}

func checkMonitor(monitor model.ChannelMonitor) {
	models := monitor.ParseModels()
	if len(models) == 0 {
		return
	}

	var historyRows []model.ChannelMonitorHistory

	for _, modelName := range models {
		result := testChannelModel(monitor.ChannelId, modelName)
		historyRows = append(historyRows, model.ChannelMonitorHistory{
			MonitorId:  monitor.Id,
			Model:      modelName,
			Success:    result.success,
			LatencyMs:  result.latencyMs,
			StatusCode: result.statusCode,
			Message:    result.message,
			CheckedAt:  time.Now(),
		})
	}

	if len(historyRows) > 0 {
		model.InsertMonitorHistoryBatch(historyRows)
	}
	model.MarkMonitorChecked(monitor.Id)

	successCount := 0
	for _, r := range historyRows {
		if r.Success {
			successCount++
		}
	}
	common.SysLog(fmt.Sprintf("[ChannelMonitor] monitor=%d channel=%d: %d/%d models OK",
		monitor.Id, monitor.ChannelId, successCount, len(historyRows)))
}

type monitorTestResult struct {
	success    bool
	latencyMs  int
	statusCode int
	message    string
}

func testChannelModel(channelId int, modelName string) monitorTestResult {
	start := time.Now()

	channel, err := model.GetChannelById(channelId, false)
	if err != nil {
		return monitorTestResult{success: false, message: fmt.Sprintf("channel not found: %v", err)}
	}

	// Fix: use GetKeys() to handle multi-key channels, take the first key
	keys := channel.GetKeys()
	if len(keys) == 0 {
		return monitorTestResult{success: false, message: "channel has no API key"}
	}
	apiKey := keys[0]

	baseURL := channel.GetBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/v1/chat/completions",
		strings.NewReader(fmt.Sprintf(`{"model":"%s","messages":[{"role":"user","content":"hi"}],"max_tokens":5}`, modelName)))
	if err != nil {
		return monitorTestResult{success: false, message: fmt.Sprintf("build request: %v", err), latencyMs: int(time.Since(start).Milliseconds())}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	latencyMs := int(time.Since(start).Milliseconds())

	if err != nil {
		return monitorTestResult{success: false, statusCode: 0, latencyMs: latencyMs, message: fmt.Sprintf("request failed: %v", err)}
	}
	defer resp.Body.Close()

	// Fix: only 2xx counts as success; 429/4xx/5xx all fail
	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	message := fmt.Sprintf("status=%d", resp.StatusCode)
	switch {
	case resp.StatusCode == 429:
		message += " (rate limited)"
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		message += " (client error)"
	case resp.StatusCode >= 500:
		message += " (server error)"
	}

	return monitorTestResult{success: success, statusCode: resp.StatusCode, latencyMs: latencyMs, message: message}
}

func cleanupOldHistory() {
	before := time.Now().AddDate(0, 0, -monitorCleanupDays)
	deleted, err := model.DeleteMonitorHistoryBefore(before)
	if err == nil && deleted > 0 {
		common.SysLog(fmt.Sprintf("[ChannelMonitor] cleaned up %d old history entries", deleted))
	}
}
