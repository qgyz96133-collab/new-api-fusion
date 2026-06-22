package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// GetFallbackChainConfig returns the current fallback chain configuration
func GetFallbackChainConfig(c *gin.Context) {
	cfg := model.GetFallbackConfig()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": cfg})
}

// UpdateFallbackChainConfig updates the fallback chain configuration
func UpdateFallbackChainConfig(c *gin.Context) {
	var cfg model.FallbackChainConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	if cfg.ProviderChains == nil {
		cfg.ProviderChains = make(map[string][]string)
	}
	if cfg.ModelMappings == nil {
		cfg.ModelMappings = make(map[string]model.ModelMappingTarget)
	}

	if err := model.SaveFallbackConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": cfg})
}

// GetChannelScores returns all channel performance scores
func GetChannelScores(c *gin.Context) {
	scores := model.GetAllChannelScores()

	type ScoreInfo struct {
		ChannelID      int     `json:"channel_id"`
		ErrorCount     int     `json:"error_count"`
		SuccessCount   int     `json:"success_count"`
		AvgLatencyMs   float64 `json:"avg_latency_ms"`
		InCooldown     bool    `json:"in_cooldown"`
		CooldownSecs   float64 `json:"cooldown_secs"`
		Consecutive429 int     `json:"consecutive_429"`
	}

	result := make([]ScoreInfo, 0, len(scores))
	for _, s := range scores {
		info := ScoreInfo{
			ChannelID:      s.ChannelID,
			ErrorCount:     s.ErrorCount,
			SuccessCount:   s.SuccessCount,
			InCooldown:     s.IsInCooldown(),
			CooldownSecs:   s.CooldownRemaining().Seconds(),
			Consecutive429: s.Consecutive429,
		}
		if s.SuccessCount > 0 {
			info.AvgLatencyMs = float64(s.TotalLatencyMs) / float64(s.SuccessCount)
		}
		result = append(result, info)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// UpdatePromptReplacements configures system prompt text replacement rules
func UpdatePromptReplacements(c *gin.Context) {
	var req struct {
		Rules []struct {
			Old string `json:"old" binding:"required"`
			New string `json:"new"`
		} `json:"rules"`
		Mode string `json:"mode"` // "overwrite" or "append"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	rules := make([]PromptReplacementRule, len(req.Rules))
	for i, r := range req.Rules {
		rules[i] = PromptReplacementRule{Old: r.Old, New: r.New}
	}
	if req.Mode == "" {
		req.Mode = "overwrite"
	}

	SetPromptReplacementRules(rules, req.Mode)

	// Persist to DB
	rulesJSON, _ := json.Marshal(map[string]interface{}{"rules": req.Rules, "mode": req.Mode})
	model.UpdateOption("prompt_replacement_rules", string(rulesJSON))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Updated %d replacement rules (mode: %s)", len(rules), req.Mode),
	})
}

// GetPromptReplacements returns current prompt replacement rules
func GetPromptReplacements(c *gin.Context) {
	rules, mode := GetPromptReplacementRules()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"rules": rules,
			"mode":  mode,
		},
	})
}

// EnableUTLSTransport enables uTLS Chrome fingerprint for upstream requests
func EnableUTLSTransport(c *gin.Context) {
	var req struct {
		Fingerprint string `json:"fingerprint"` // chrome, firefox, safari, edge
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	fp := common.FingerprintChrome
	switch strings.ToLower(req.Fingerprint) {
	case "firefox":
		fp = common.FingerprintFirefox
	case "safari":
		fp = common.FingerprintSafari
	case "edge":
		fp = common.FingerprintEdge
	}

	service.EnableUTLS(fp)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("uTLS enabled with %s fingerprint", req.Fingerprint)})
}

// DisableUTLSTransport disables uTLS transport
func DisableUTLSTransport(c *gin.Context) {
	service.DisableUTLS()
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "uTLS disabled"})
}

// GetUTLSStatus returns uTLS status
func GetUTLSStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled": service.IsUTLSEnabled(),
		},
	})
}
