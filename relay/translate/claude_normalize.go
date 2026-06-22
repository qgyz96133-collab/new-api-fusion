package translate

import (
	"regexp"
	"strings"
)

// ADAPTIVE_THINKING_UNSUPPORTED matches models that don't support thinking.type "adaptive"
// Only Sonnet/Opus support adaptive thinking; Haiku rejects it
var adaptiveThinkingUnsupported = regexp.MustCompile(`(?i)haiku`)

// DefaultThinkingSignature is a placeholder signature for thinking blocks
// In production, this should be a real signature from Anthropic
const DefaultThinkingSignature = "EQoUCgIASg4IABABGAEiBjACOAFCBggBEAIYAiICGAQiAB"

// NormalizeClaudePassthrough normalizes a Claude request to match Anthropic Messages API spec.
// Handles:
// 1. Downgrade adaptive thinking for Haiku models
// 2. Hoist mid-conversation system messages to top-level system field
func NormalizeClaudePassthrough(body map[string]interface{}, model string) map[string]interface{} {
	if body == nil {
		return nil
	}

	// 1. Downgrade adaptive thinking for models that don't support it
	if thinking, ok := body["thinking"].(map[string]interface{}); ok {
		if thinkingType, _ := thinking["type"].(string); thinkingType == "adaptive" {
			if adaptiveThinkingUnsupported.MatchString(model) {
				body["thinking"] = map[string]interface{}{
					"type":          "enabled",
					"budget_tokens": 10000,
				}
			}
		}
	}

	// 2. Hoist mid-conversation system messages into top-level system field
	if messages, ok := body["messages"].([]interface{}); ok {
		var systemBlocks []map[string]interface{}
		var filteredMessages []interface{}

		for _, msg := range messages {
			if msgMap, ok := msg.(map[string]interface{}); ok {
				if role, _ := msgMap["role"].(string); role == "system" {
					// Extract text from system message
					var text string
					switch content := msgMap["content"].(type) {
					case string:
						text = content
					case []interface{}:
						var parts []string
						for _, block := range content {
							switch b := block.(type) {
							case string:
								parts = append(parts, b)
							case map[string]interface{}:
								if t, ok := b["text"].(string); ok {
									parts = append(parts, t)
								}
							}
						}
						text = strings.Join(parts, "\n")
					}

					if trimmed := strings.TrimSpace(text); trimmed != "" {
						systemBlocks = append(systemBlocks, map[string]interface{}{
							"type": "text",
							"text": trimmed,
						})
					}
					continue // Skip this message (don't add to filteredMessages)
				}
			}
			filteredMessages = append(filteredMessages, msg)
		}

		if len(systemBlocks) > 0 {
			// Merge with existing system field
			var existing []map[string]interface{}
			switch sys := body["system"].(type) {
			case []interface{}:
				for _, block := range sys {
					if m, ok := block.(map[string]interface{}); ok {
						existing = append(existing, m)
					}
				}
			case string:
				if trimmed := strings.TrimSpace(sys); trimmed != "" {
					existing = append(existing, map[string]interface{}{
						"type": "text",
						"text": trimmed,
					})
				}
			}

			body["system"] = append(existing, systemBlocks...)
			body["messages"] = filteredMessages
		}
	}

	return body
}

// PrepareClaudeRequest applies full preprocessing pipeline for Claude requests:
// 1. Remove MiniMax-specific output_config if needed
// 2. Normalize passthrough (system hoist, thinking downgrade)
// 3. Fix tool_use ordering
// 4. Add cache_control to system and tools
// 5. Inject thinking blocks if needed
func PrepareClaudeRequest(body map[string]interface{}, model, provider string) map[string]interface{} {
	if body == nil {
		return nil
	}

	// Normalize passthrough first
	body = NormalizeClaudePassthrough(body, model)

	// MiniMax-specific cleanup
	if provider == "minimax" || provider == "minimax-cn" {
		delete(body, "output_config")
	}

	// Process system field: remove cache_control, add to last block
	if sysArr, ok := body["system"].([]interface{}); ok && len(sysArr) > 0 {
		cleanedSys := make([]map[string]interface{}, 0, len(sysArr))
		for i, block := range sysArr {
			if blockMap, ok := block.(map[string]interface{}); ok {
				newBlock := make(map[string]interface{})
				for k, v := range blockMap {
					if k != "cache_control" {
						newBlock[k] = v
					}
				}
				// Add cache_control to last block only
				if i == len(sysArr)-1 {
					newBlock["cache_control"] = map[string]interface{}{
						"type": "ephemeral",
						"ttl":  "1h",
					}
				}
				cleanedSys = append(cleanedSys, newBlock)
			}
		}
		body["system"] = cleanedSys
	}

	// Process messages
	if messages, ok := body["messages"].([]interface{}); ok {
		body["messages"] = processClaudeMessages(messages, body, provider)
	}

	// Process tools: filter built-in for non-Anthropic, add cache_control
	if tools, ok := body["tools"].([]interface{}); ok && len(tools) > 0 {
		body["tools"] = processClaudeTools(tools, provider)
	}

	return body
}

// processClaudeMessages handles message-level transformations
func processClaudeMessages(messages []interface{}, body map[string]interface{}, provider string) []interface{} {
	if len(messages) == 0 {
		return messages
	}

	// Pass 1: Remove cache_control from content, filter empty messages
	var filtered []interface{}
	for i, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}

		// Remove cache_control from content blocks
		if contentArr, ok := msgMap["content"].([]interface{}); ok {
			for _, block := range contentArr {
				if blockMap, ok := block.(map[string]interface{}); ok {
					delete(blockMap, "cache_control")
				}
			}
		}

		// Keep final assistant even if empty, otherwise check valid content
		isFinalAssistant := i == len(messages)-1 && msgMap["role"] == "assistant"
		if isFinalAssistant || hasValidContent(msgMap) {
			filtered = append(filtered, msg)
		}
	}

	// Pass 1.5: Fix tool_use ordering
	filtered = fixToolUseOrdering(filtered)

	// Check if thinking is enabled and last message is from user
	lastMsg, _ := filtered[len(filtered)-1].(map[string]interface{})
	lastMsgIsUser := lastMsg != nil && lastMsg["role"] == "user"
	thinkingEnabled := false
	if thinking, ok := body["thinking"].(map[string]interface{}); ok {
		if t, _ := thinking["type"].(string); t == "enabled" {
			thinkingEnabled = true
		}
	}
	shouldInjectThinking := thinkingEnabled && lastMsgIsUser

	// Pass 2 (reverse): Add cache_control to last assistant, handle thinking blocks
	lastAssistantProcessed := false
	isAnthropicProvider := provider == "claude" || strings.HasPrefix(provider, "anthropic-compatible")

	for i := len(filtered) - 1; i >= 0; i-- {
		msgMap, ok := filtered[i].(map[string]interface{})
		if !ok {
			continue
		}

		if msgMap["role"] == "assistant" {
			if contentArr, ok := msgMap["content"].([]interface{}); ok && len(contentArr) > 0 {
				// Add cache_control to last non-thinking block
				if !lastAssistantProcessed {
					for j := len(contentArr) - 1; j >= 0; j-- {
						if blockMap, ok := contentArr[j].(map[string]interface{}); ok {
							blockType, _ := blockMap["type"].(string)
							if blockType != "thinking" && blockType != "redacted_thinking" {
								blockMap["cache_control"] = map[string]interface{}{
									"type": "ephemeral",
								}
								break
							}
						}
					}
					lastAssistantProcessed = true
				}

				// Handle thinking blocks for Anthropic endpoint
				if isAnthropicProvider {
					hasToolUse := false
					hasThinking := false

					// Replace signatures and check for tool_use
					for _, block := range contentArr {
						if blockMap, ok := block.(map[string]interface{}); ok {
							blockType, _ := blockMap["type"].(string)
							if blockType == "thinking" || blockType == "redacted_thinking" {
								blockMap["signature"] = DefaultThinkingSignature
								hasThinking = true
							}
							if blockType == "tool_use" {
								hasToolUse = true
							}
						}
					}

					// Inject thinking block if needed
					if shouldInjectThinking && !hasThinking && hasToolUse {
						thinkingBlock := map[string]interface{}{
							"type":      "thinking",
							"thinking":  ".",
							"signature": DefaultThinkingSignature,
						}
						// Prepend to content array
						newContent := make([]interface{}, 0, len(contentArr)+1)
						newContent = append(newContent, thinkingBlock)
						newContent = append(newContent, contentArr...)
						msgMap["content"] = newContent
					}
				}
			}
		}
	}

	return filtered
}

// hasValidContent checks if a message has non-empty content
func hasValidContent(msg map[string]interface{}) bool {
	switch content := msg["content"].(type) {
	case string:
		return strings.TrimSpace(content) != ""
	case []interface{}:
		for _, block := range content {
			if blockMap, ok := block.(map[string]interface{}); ok {
				blockType, _ := blockMap["type"].(string)
				switch blockType {
				case "text":
					if text, ok := blockMap["text"].(string); ok && strings.TrimSpace(text) != "" {
						return true
					}
				case "tool_use", "tool_result":
					return true
				}
			}
		}
	}
	return false
}

// fixToolUseOrdering fixes Claude's strict ordering requirements:
// 1. Remove text blocks AFTER tool_use in assistant messages
// 2. Merge consecutive same-role messages
func fixToolUseOrdering(messages []interface{}) []interface{} {
	if len(messages) <= 1 {
		return messages
	}

	// Pass 1: Fix assistant messages with tool_use
	for _, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}
		if msgMap["role"] != "assistant" {
			continue
		}
		contentArr, ok := msgMap["content"].([]interface{})
		if !ok {
			continue
		}

		// Check if has tool_use
		hasToolUse := false
		for _, block := range contentArr {
			if blockMap, ok := block.(map[string]interface{}); ok {
				if blockType, _ := blockMap["type"].(string); blockType == "tool_use" {
					hasToolUse = true
					break
				}
			}
		}

		if hasToolUse {
			// Keep: thinking + tool_use (remove text after tool_use)
			var newContent []interface{}
			foundToolUse := false

			for _, block := range contentArr {
				blockMap, ok := block.(map[string]interface{})
				if !ok {
					continue
				}
				blockType, _ := blockMap["type"].(string)

				if blockType == "tool_use" {
					foundToolUse = true
					newContent = append(newContent, block)
				} else if blockType == "thinking" || blockType == "redacted_thinking" {
					newContent = append(newContent, block)
				} else if !foundToolUse {
					// Keep text blocks BEFORE tool_use
					newContent = append(newContent, block)
				}
				// Skip text blocks AFTER tool_use
			}
			msgMap["content"] = newContent
		}
	}

	// Pass 2: Merge consecutive same-role messages
	var merged []interface{}
	for _, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}

		if len(merged) == 0 {
			merged = append(merged, msg)
			continue
		}

		lastMap, _ := merged[len(merged)-1].(map[string]interface{})
		if lastMap == nil || lastMap["role"] != msgMap["role"] {
			merged = append(merged, msg)
			continue
		}

		// Same role — merge content arrays
		lastContent := ensureContentArray(lastMap["content"])
		msgContent := ensureContentArray(msgMap["content"])

		// Put tool_result first, then other content
		var toolResults, otherContent []interface{}
		for _, block := range append(lastContent, msgContent...) {
			if blockMap, ok := block.(map[string]interface{}); ok {
				if blockType, _ := blockMap["type"].(string); blockType == "tool_result" {
					toolResults = append(toolResults, block)
				} else {
					otherContent = append(otherContent, block)
				}
			}
		}
		lastMap["content"] = append(toolResults, otherContent...)
	}

	return merged
}

// ensureContentArray converts content to array format
func ensureContentArray(content interface{}) []interface{} {
	switch c := content.(type) {
	case []interface{}:
		return c
	case string:
		return []interface{}{map[string]interface{}{"type": "text", "text": c}}
	case map[string]interface{}:
		return []interface{}{c}
	default:
		return nil
	}
}

// processClaudeTools handles tool field preprocessing.
// Returns nil if tools are empty after filtering (caller should delete the key).
func processClaudeTools(tools []interface{}, provider string) []interface{} {
	// Strip built-in tools (e.g. web_search_20250305) for non-Anthropic providers
	if provider != "claude" {
		var filtered []interface{}
		for _, tool := range tools {
			if toolMap, ok := tool.(map[string]interface{}); ok {
				toolType, _ := toolMap["type"].(string)
				// Keep if no type or type is "function"
				if toolType == "" || toolType == "function" {
					filtered = append(filtered, tool)
				}
			}
		}
		tools = filtered
	}

	if len(tools) == 0 {
		return nil
	}

	// Add cache_control to last tool
	cleanedTools := make([]interface{}, 0, len(tools))
	for i, tool := range tools {
		if toolMap, ok := tool.(map[string]interface{}); ok {
			newTool := make(map[string]interface{})
			for k, v := range toolMap {
				if k != "cache_control" {
					newTool[k] = v
				}
			}
			if i == len(tools)-1 {
				newTool["cache_control"] = map[string]interface{}{
					"type": "ephemeral",
					"ttl":  "1h",
				}
			}
			cleanedTools = append(cleanedTools, newTool)
		}
	}

	return cleanedTools
}
