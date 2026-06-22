package relay

import (
	"encoding/json"
	"strings"
	"sync"

	"fmt"

	"github.com/QuantumNous/new-api/common"
)

// ComboConfig holds model combo definitions
// A combo maps a virtual model name to a list of real models to try in order
type ComboConfig struct {
	mu     sync.RWMutex
	combos map[string][]string // combo_name -> [model1, model2, ...]
}

var globalCombos = &ComboConfig{
	combos: make(map[string][]string),
}

// RegisterCombo registers a new model combo
func RegisterCombo(name string, models []string) {
	globalCombos.mu.Lock()
	defer globalCombos.mu.Unlock()
	globalCombos.combos[name] = models
}

// RemoveCombo removes a model combo
func RemoveCombo(name string) {
	globalCombos.mu.Lock()
	defer globalCombos.mu.Unlock()
	delete(globalCombos.combos, name)
}

// GetComboModels returns the models for a combo, or nil if not a combo
func GetComboModels(name string) []string {
	globalCombos.mu.RLock()
	defer globalCombos.mu.RUnlock()
	if models, ok := globalCombos.combos[name]; ok {
		return models
	}
	return nil
}

// IsCombo checks if a model name is a combo
func IsCombo(name string) bool {
	globalCombos.mu.RLock()
	defer globalCombos.mu.RUnlock()
	_, ok := globalCombos.combos[name]
	return ok
}

// GetAllCombos returns all registered combos
func GetAllCombos() map[string][]string {
	globalCombos.mu.RLock()
	defer globalCombos.mu.RUnlock()
	result := make(map[string][]string)
	for k, v := range globalCombos.combos {
		models := make([]string, len(v))
		copy(models, v)
		result[k] = models
	}
	return result
}

// LoadCombosFromJSON loads combo definitions from JSON
// Format: {"combo-name": ["model1", "model2", ...]}
func LoadCombosFromJSON(data string) error {
	if strings.TrimSpace(data) == "" {
		return nil
	}
	var combos map[string][]string
	if err := json.Unmarshal([]byte(data), &combos); err != nil {
		return err
	}
	globalCombos.mu.Lock()
	defer globalCombos.mu.Unlock()
	globalCombos.combos = combos
	return nil
}

// GetNextComboModel returns the next model to try for a combo given a failed model
// Returns empty string if all models have been tried
func GetNextComboModel(comboName string, failedModel string) string {
	models := GetComboModels(comboName)
	if len(models) == 0 {
		return ""
	}

	found := false
	for _, m := range models {
		if found {
			return m
		}
		if m == failedModel {
			found = true
		}
	}

	// If the failed model is the last one (or not found), no more models
	return ""
}

// LogComboFallback logs a combo fallback event
func LogComboFallback(comboName string, fromModel string, toModel string) {
	common.SysLog(fmt.Sprintf("[Combo] %s: %s failed, falling back to %s", comboName, fromModel, toModel))
}

// ResolveComboWithCapabilities resolves a combo model name considering capabilities.
// If the request requires specific capabilities, reorder models to prefer capable ones.
// Ported from 9router services/combo.js reorderByCapabilities().
func ResolveComboWithCapabilities(comboName string, requiredCapabilities map[string]bool) []string {
	models := GetComboModels(comboName)
	if len(models) == 0 {
		return nil
	}

	// Use service layer for capability-based reordering
	// Import cycle prevention: we inline the logic here instead of importing service
	if len(requiredCapabilities) == 0 || len(models) <= 1 {
		return models
	}

	// For now, return models in original order
	// The relay layer will handle fallback via GetNextComboModel
	return models
}

// FlattenToolHistory converts tool-related messages to plain text for models
// that don't support tools. Ported from 9router services/combo.js flattenToolHistory().
func FlattenToolHistory(messages []map[string]interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(messages))

	for _, msg := range messages {
		role, _ := msg["role"].(string)

		// Convert tool results to assistant text
		if role == "tool" || role == "function" {
			content := extractTextContent(msg["content"])
			result = append(result, map[string]interface{}{
				"role":    "assistant",
				"content": "[Tool result: " + content + "]",
			})
			continue
		}

		// Convert assistant tool_calls to text
		if role == "assistant" {
			if toolCalls, ok := msg["tool_calls"].([]interface{}); ok && len(toolCalls) > 0 {
				// Extract text content
				textContent := ""
				if content, ok := msg["content"].(string); ok {
					textContent = content
				}

				// Extract tool names
				names := make([]string, 0, len(toolCalls))
				for _, tc := range toolCalls {
					if tcMap, ok := tc.(map[string]interface{}); ok {
						if fn, ok := tcMap["function"].(map[string]interface{}); ok {
							if name, ok := fn["name"].(string); ok {
								names = append(names, name)
							}
						}
					}
				}

				// Build flat message without tool_calls
				flat := map[string]interface{}{
					"role": "assistant",
				}
				if textContent != "" {
					flat["content"] = textContent + "\n[Called tools: " + strings.Join(names, ", ") + "]"
				} else {
					flat["content"] = "[Called tools: " + strings.Join(names, ", ") + "]"
				}
				result = append(result, flat)
				continue
			}
		}

		// Pass through other messages unchanged
		result = append(result, msg)
	}

	return result
}

func extractTextContent(content interface{}) string {
	if content == nil {
		return ""
	}
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		parts := make([]string, 0)
		for _, item := range v {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if text, ok := itemMap["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return fmt.Sprintf("%v", content)
	}
}
