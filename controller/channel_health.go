package controller

import (
	"net/http"
	"io"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// GetChannelHealthStatus returns health status for all channels
func GetChannelHealthStatus(c *gin.Context) {
	summaries := model.GetAllChannelHealth()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": summaries})
}

// GetChannelHealthDetail returns detailed health for a specific channel
func GetChannelHealthDetail(c *gin.Context) {
	summary := model.GetChannelHealth(getPathParamInt(c, "id"))
	c.JSON(http.StatusOK, gin.H{"success": true, "data": summary})
}

// GetChannelHealthHistory returns recent health check records for a channel
func GetChannelHealthHistory(c *gin.Context) {
	channelID := getPathParamInt(c, "id")
	var records []model.ChannelHealthRecord
	model.DB.Where("channel_id = ?", channelID).
		Order("checked_at DESC").
		Limit(50).
		Find(&records)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": records})
}

// TriggerChannelHealthCheck manually triggers a health check for a channel
func TriggerChannelHealthCheck(c *gin.Context) {
	channelID := getPathParamInt(c, "id")

	channel, err := model.GetChannelById(channelID, false)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "channel not found"})
		return
	}

	// Run a test request
	statusCode, latencyMs, testErr := testChannelHealth(channel)
	errStr := ""
	if testErr != nil {
		errStr = testErr.Error()
	}

	testModel := ""
	if channel.TestModel != nil { testModel = *channel.TestModel }
	model.RecordHealthCheck(channelID, channel.Name, testModel, statusCode, latencyMs, errStr)

	summary := model.GetChannelHealth(channelID)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": summary})
}

// testChannelHealth performs a minimal health check on a channel
func testChannelHealth(channel *model.Channel) (int, int, error) {
	start := time.Now()

	baseURL := "https://api.openai.com"
	if channel.BaseURL != nil && *channel.BaseURL != "" {
		baseURL = *channel.BaseURL
	}

	// Simple connectivity test
	resp, err := http.Get(baseURL + "/v1/models")
	latencyMs := int(time.Since(start).Milliseconds())

	if err != nil {
		return 500, latencyMs, err
	}
	defer resp.Body.Close()

	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, latencyMs, nil
}

func getPathParamInt(c *gin.Context, param string) int {
	val, _ := strconv.Atoi(c.Param(param))
	return val
}
