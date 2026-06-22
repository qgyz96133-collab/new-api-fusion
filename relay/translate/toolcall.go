package translate

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// TOOL_ID_PATTERN matches Anthropic tool_use.id requirement: ^[a-zA-Z0-9_-]+$
var toolIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
var invalidCharsPattern = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// Message represents a simplified message structure for tool call processing
type Message struct {
	Role       string        `json:"role"`
	Content    interface{}   `json:"content"`
	ToolCalls  []ToolCall    `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

// ToolCall represents an OpenAI-style tool call
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction represents the function part of a tool call
type ToolFunction struct {
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments"`
}

// ContentBlock represents a Claude-style content block
type ContentBlock struct {
	Type      string `json:"type"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	Text      string `json:"text,omitempty"`
}

// GenerateToolCallID generates a deterministic tool call ID from position + tool name
// Format: call_msg{msgIndex}_tc{tcIndex}_{sanitizedName}
func GenerateToolCallID(msgIndex, tcIndex int, toolName string) string {
	sanitizedName := invalidCharsPattern.ReplaceAllString(toolName, "")
	if sanitizedName != "" {
		return fmt.Sprintf("call_msg%d_tc%d_%s", msgIndex, tcIndex, sanitizedName)
	}
	return fmt.Sprintf("call_msg%d_tc%d", msgIndex, tcIndex)
}

// SanitizeToolID removes invalid characters from tool ID
// Returns empty string if result is empty
func SanitizeToolID(id string) string {
	if id == "" {
		return ""
	}
	sanitized := invalidCharsPattern.ReplaceAllString(id, "")
	return sanitized
}

// IsValidToolID checks if tool ID matches Anthropic pattern
func IsValidToolID(id string) bool {
	return id != "" && toolIDPattern.MatchString(id)
}

// EnsureToolCallIds validates and fixes all tool call IDs in messages
// - Validates/regenerates IDs for Anthropic compatibility
// - Ensures type field is set
// - Stringifies arguments if needed
func EnsureToolCallIds(messages []Message) []Message {
	for i, msg := range messages {
		// Handle OpenAI format: tool_calls array
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			for j := range msg.ToolCalls {
				tc := &messages[i].ToolCalls[j]

				// Validate or regenerate ID
				if tc.ID == "" || !IsValidToolID(tc.ID) {
					sanitized := SanitizeToolID(tc.ID)
					if sanitized != "" {
						tc.ID = sanitized
					} else {
						tc.ID = GenerateToolCallID(i, j, tc.Function.Name)
					}
				}

				// Ensure type is set
				if tc.Type == "" {
					tc.Type = "function"
				}

				// Stringify arguments if needed
				if tc.Function.Arguments != nil {
					if _, isString := tc.Function.Arguments.(string); !isString {
						if jsonBytes, err := json.Marshal(tc.Function.Arguments); err == nil {
							tc.Function.Arguments = string(jsonBytes)
						}
					}
				}
			}
		}

		// Handle tool messages: tool_call_id
		if msg.Role == "tool" && msg.ToolCallID != "" && !IsValidToolID(msg.ToolCallID) {
			sanitized := SanitizeToolID(msg.ToolCallID)
			if sanitized != "" {
				messages[i].ToolCallID = sanitized
			} else {
				messages[i].ToolCallID = GenerateToolCallID(i, 0, "")
			}
		}

		// Handle Claude format: content blocks
		if contentArray, ok := msg.Content.([]interface{}); ok {
			for k, block := range contentArray {
				if blockMap, ok := block.(map[string]interface{}); ok {
					blockType, _ := blockMap["type"].(string)

					// Validate tool_use.id
					if blockType == "tool_use" {
						if id, ok := blockMap["id"].(string); ok && id != "" && !IsValidToolID(id) {
							sanitized := SanitizeToolID(id)
							if sanitized != "" {
								blockMap["id"] = sanitized
							} else {
								name, _ := blockMap["name"].(string)
								blockMap["id"] = GenerateToolCallID(i, k, name)
							}
						}
					}

					// Validate tool_result.tool_use_id
					if blockType == "tool_result" {
						if toolUseID, ok := blockMap["tool_use_id"].(string); ok && toolUseID != "" && !IsValidToolID(toolUseID) {
							sanitized := SanitizeToolID(toolUseID)
							if sanitized != "" {
								blockMap["tool_use_id"] = sanitized
							} else {
								blockMap["tool_use_id"] = GenerateToolCallID(i, k, "")
							}
						}
					}
				}
			}
		}
	}

	return messages
}
