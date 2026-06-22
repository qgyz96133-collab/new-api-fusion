package controller

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// --- Channel Scoring Integration (ported from AIClient2API) ---

// RecordChannelSuccess records a successful relay for channel scoring
func RecordChannelSuccess(c *gin.Context, channelID int) {
	startTime := c.GetTime("relay_start_time")
	if startTime.IsZero() {
		startTime = time.Now()
	}
	latencyMs := time.Since(startTime).Milliseconds()

	score := model.GetChannelScore(channelID)
	score.RecordSuccess(latencyMs)

	// Update SmartRouter health
	relay.UpdateChannelHealth(channelID, true, time.Duration(latencyMs)*time.Millisecond)

	if common.DebugEnabled {
		common.SysLog(fmt.Sprintf("[Score] channel=%d success latency=%dms", channelID, latencyMs))
	}
}

// RecordChannelError records a failed relay for channel scoring
func RecordChannelError(c *gin.Context, channelID int, newAPIErr *types.NewAPIError) {
	statusCode := 500
	if newAPIErr != nil {
		statusCode = newAPIErr.StatusCode
	}

	score := model.GetChannelScore(channelID)
	score.RecordError(statusCode)

	// Update SmartRouter health
	startTime := c.GetTime("relay_start_time")
	latency := time.Duration(0)
	if !startTime.IsZero() {
		latency = time.Since(startTime)
	}
	relay.UpdateChannelHealth(channelID, false, latency)

	if common.DebugEnabled {
		cooldown := score.CooldownRemaining()
		common.SysLog(fmt.Sprintf("[Score] channel=%d error status=%d cooldown=%s", channelID, statusCode, cooldown))
	}
}

// MarkRelayStart sets the relay start time for latency tracking
func MarkRelayStart(c *gin.Context) {
	c.Set("relay_start_time", time.Now())
}

// --- Fallback Chain + Model Mapping Integration ---

// CheckModelMapping checks if the requested model has a fallback mapping
// Returns the target model name and whether a mapping was found
func CheckModelMapping(requestedModel string) (string, bool) {
	target, ok := model.GetModelMapping(requestedModel)
	if !ok {
		return requestedModel, false
	}

	if common.DebugEnabled {
		common.SysLog(fmt.Sprintf("[Fallback] model mapping: %s -> %s (channel_type=%d)",
			requestedModel, target.Model, target.ChannelType))
	}

	return target.Model, true
}

// GetFallbackChannelTypes returns fallback channel types for the current channel
func GetFallbackChannelTypes(channelType int) []int {
	typeName := model.ChannelTypeName(channelType)
	if typeName == "" {
		return nil
	}

	fallbackNames := model.GetFallbackChannels(typeName)
	if len(fallbackNames) == 0 {
		return nil
	}

	return model.ParseChannelTypeNames(strings.Join(fallbackNames, ","))
}

// --- System Prompt Replacement Rules ---

// PromptReplacementRule defines a text replacement rule for system prompts
type PromptReplacementRule struct {
	Old string `json:"old"`
	New string `json:"new"`
}

var (
	promptReplacementRules []PromptReplacementRule
	promptReplacementMode  string // "overwrite" or "append"
)

// SetPromptReplacementRules configures the prompt replacement rules
func SetPromptReplacementRules(rules []PromptReplacementRule, mode string) {
	promptReplacementRules = rules
	promptReplacementMode = mode
}

// ApplyPromptReplacements applies configured text replacements to a prompt
func ApplyPromptReplacements(prompt string) string {
	if len(promptReplacementRules) == 0 {
		return prompt
	}

	result := prompt
	for _, rule := range promptReplacementRules {
		if rule.Old != "" {
			result = strings.ReplaceAll(result, rule.Old, rule.New)
		}
	}

	if common.DebugEnabled && result != prompt {
		common.SysLog("[PromptReplace] applied text replacements to system prompt")
	}

	return result
}

// GetPromptReplacementRules returns current rules
func GetPromptReplacementRules() ([]PromptReplacementRule, string) {
	return promptReplacementRules, promptReplacementMode
}
