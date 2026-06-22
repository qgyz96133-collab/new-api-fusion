package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// ContentModerationConfig holds moderation settings
type ContentModerationConfig struct {
	Enabled          bool
	Mode             string // "off", "observe", "pre_block"
	BlockedKeywords  []string
	APIBaseURL       string // external moderation API
	APIKey           string
	TimeoutMs        int
	BanThreshold     int // violations before key ban
}

// ContentModerationDecision represents the moderation result
type ContentModerationDecision struct {
	Allowed    bool
	Action     string // "allow", "keyword_block", "api_block"
	Reason     string
	Category   string
	StatusCode int
}

var (
	moderationConfig     ContentModerationConfig
	moderationConfigMu   sync.RWMutex
	violationCounters    = make(map[string]int) // api_key -> count
	violationCountersMu  sync.Mutex
)

// SetContentModerationConfig updates the moderation configuration
func SetContentModerationConfig(cfg ContentModerationConfig) {
	moderationConfigMu.Lock()
	defer moderationConfigMu.Unlock()
	moderationConfig = cfg
}

// GetContentModerationConfig returns the current config
func GetContentModerationConfig() ContentModerationConfig {
	moderationConfigMu.RLock()
	defer moderationConfigMu.RUnlock()
	return moderationConfig
}

// ContentModeration middleware checks request content against moderation rules
func ContentModeration() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := GetContentModerationConfig()
		if !cfg.Enabled || cfg.Mode == "off" {
			c.Next()
			return
		}

		// Read request body
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Next()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		// Extract text content from request
		textContent := extractTextFromRequestBody(body)
		if textContent == "" {
			c.Next()
			return
		}

		// Check keyword blocklist
		decision := checkKeywords(textContent, cfg.BlockedKeywords)
		if !decision.Allowed {
			if cfg.Mode == "pre_block" {
				// Block the request
				apiKey := extractAPIKey(c)
				recordViolation(apiKey, decision)

				if isKeyBanned(apiKey, cfg.BanThreshold) {
					c.JSON(http.StatusForbidden, gin.H{
						"error": gin.H{
							"message": "API key has been suspended due to repeated content policy violations",
							"type":    "content_policy_violation",
							"code":    "key_suspended",
						},
					})
					c.Abort()
					return
				}

				c.JSON(decision.StatusCode, gin.H{
					"error": gin.H{
						"message": decision.Reason,
						"type":    "content_policy_violation",
						"code":    decision.Action,
					},
				})
				c.Abort()
				return
			}
			// observe mode: log but don't block
			common.SysLog(fmt.Sprintf("[ContentModeration] OBSERVE: %s | key=%s | reason=%s",
				decision.Action, extractAPIKeyMasked(c), decision.Reason))
		}

		c.Next()
	}
}

// extractTextFromRequestBody extracts text content from various API request formats
func extractTextFromRequestBody(body []byte) string {
	var request map[string]interface{}
	if err := json.Unmarshal(body, &request); err != nil {
		return ""
	}

	var texts []string

	// OpenAI messages format
	if messages, ok := request["messages"].([]interface{}); ok {
		for _, msg := range messages {
			if msgMap, ok := msg.(map[string]interface{}); ok {
				if content, ok := msgMap["content"].(string); ok {
					texts = append(texts, content)
				}
				// Array content format
				if contentArr, ok := msgMap["content"].([]interface{}); ok {
					for _, part := range contentArr {
						if partMap, ok := part.(map[string]interface{}); ok {
							if text, ok := partMap["text"].(string); ok {
								texts = append(texts, text)
							}
						}
					}
				}
			}
		}
	}

	// Claude system field
	if system, ok := request["system"].(string); ok {
		texts = append(texts, system)
	}
	if systemArr, ok := request["system"].([]interface{}); ok {
		for _, block := range systemArr {
			if blockMap, ok := block.(map[string]interface{}); ok {
				if text, ok := blockMap["text"].(string); ok {
					texts = append(texts, text)
				}
			}
		}
	}

	// Prompt field (completions API)
	if prompt, ok := request["prompt"].(string); ok {
		texts = append(texts, prompt)
	}

	// Input field (responses API)
	if input, ok := request["input"].(string); ok {
		texts = append(texts, input)
	}

	return strings.Join(texts, "\n")
}

// checkKeywords checks text against blocked keywords
func checkKeywords(text string, keywords []string) ContentModerationDecision {
	textLower := strings.ToLower(text)
	for _, kw := range keywords {
		if kw == "" {
			continue
		}
		if strings.Contains(textLower, strings.ToLower(kw)) {
			return ContentModerationDecision{
				Allowed:    false,
				Action:     "keyword_block",
				Reason:     "Content blocked by keyword filter",
				Category:   "keyword",
				StatusCode: http.StatusForbidden,
			}
		}
	}
	return ContentModerationDecision{Allowed: true, Action: "allow"}
}

// recordViolation records a content moderation violation
func recordViolation(apiKey string, decision ContentModerationDecision) {
	if apiKey == "" {
		return
	}
	violationCountersMu.Lock()
	defer violationCountersMu.Unlock()
	violationCounters[apiKey]++
	common.SysLog(fmt.Sprintf("[ContentModeration] BLOCK: %s | key=%s...%s | violations=%d | reason=%s",
		decision.Action, apiKey[:min(6, len(apiKey))], apiKey[max(0, len(apiKey)-4):], violationCounters[apiKey], decision.Reason))
}

// isKeyBanned checks if an API key has exceeded the violation threshold
func isKeyBanned(apiKey string, threshold int) bool {
	if apiKey == "" || threshold <= 0 {
		return false
	}
	violationCountersMu.Lock()
	defer violationCountersMu.Unlock()
	return violationCounters[apiKey] >= threshold
}

func extractAPIKey(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return c.GetHeader("x-api-key")
}

func extractAPIKeyMasked(c *gin.Context) string {
	key := extractAPIKey(c)
	if len(key) <= 10 {
		return "***"
	}
	return key[:6] + "..." + key[len(key)-4:]
}

// CleanupViolationCounters periodically cleans up old violation counters
func init() {
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			violationCountersMu.Lock()
			violationCounters = make(map[string]int)
			violationCountersMu.Unlock()
		}
	}()
}
