package translate

// FixOrphanToolResults detects and fixes orphaned tool results.
// When a client compacts conversation history, it may remove the assistant message
// containing tool_use but keep the tool_result, causing 400 errors from upstream.
// This function inserts empty tool_result messages for orphaned tool_calls.
func FixOrphanToolResults(messages []interface{}) []interface{} {
	if len(messages) == 0 {
		return messages
	}

	var newMessages []interface{}

	for i := 0; i < len(messages); i++ {
		msg := messages[i]
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			newMessages = append(newMessages, msg)
			continue
		}

		newMessages = append(newMessages, msg)

		// Only process assistant messages
		if msgMap["role"] != "assistant" {
			continue
		}

		// Get tool call IDs from this message
		toolCallIDs := getToolCallIDsFromMessage(msgMap)
		if len(toolCallIDs) == 0 {
			continue
		}

		// Check if next message has tool results
		nextMsg := getNextMessage(messages, i)
		if nextMsg != nil && hasToolResultsForIDs(nextMsg, toolCallIDs) {
			continue // Tool results exist, no fix needed
		}

		// Insert empty tool results for each tool_call
		for _, id := range toolCallIDs {
			toolResult := map[string]interface{}{
				"role":         "tool",
				"tool_call_id": id,
				"content":      "",
			}
			newMessages = append(newMessages, toolResult)
		}
	}

	return newMessages
}

// getToolCallIDsFromMessage extracts all tool call IDs from a message
// Supports both OpenAI format (tool_calls) and Claude format (tool_use blocks)
func getToolCallIDsFromMessage(msg map[string]interface{}) []string {
	var ids []string

	// OpenAI format: tool_calls array
	if toolCalls, ok := msg["tool_calls"].([]interface{}); ok {
		for _, tc := range toolCalls {
			if tcMap, ok := tc.(map[string]interface{}); ok {
				if id, ok := tcMap["id"].(string); ok && id != "" {
					ids = append(ids, id)
				}
			}
		}
	}

	// Claude format: tool_use blocks in content
	if contentArr, ok := msg["content"].([]interface{}); ok {
		for _, block := range contentArr {
			if blockMap, ok := block.(map[string]interface{}); ok {
				if blockType, _ := blockMap["type"].(string); blockType == "tool_use" {
					if id, ok := blockMap["id"].(string); ok && id != "" {
						ids = append(ids, id)
					}
				}
			}
		}
	}

	return ids
}

// getNextMessage safely gets the next message in the array
func getNextMessage(messages []interface{}, currentIndex int) map[string]interface{} {
	if currentIndex+1 >= len(messages) {
		return nil
	}
	nextMsg, ok := messages[currentIndex+1].(map[string]interface{})
	if !ok {
		return nil
	}
	return nextMsg
}

// hasToolResultsForIDs checks if a message has tool results for the given IDs
// Supports OpenAI format (role=tool with tool_call_id) and Claude format (tool_result blocks)
func hasToolResultsForIDs(msg map[string]interface{}, toolCallIDs []string) bool {
	if len(toolCallIDs) == 0 {
		return false
	}

	// OpenAI format: role = "tool" with tool_call_id
	if role, _ := msg["role"].(string); role == "tool" {
		if toolCallID, ok := msg["tool_call_id"].(string); ok {
			return containsString(toolCallIDs, toolCallID)
		}
	}

	// Claude format: tool_result blocks in user message content
	if role, _ := msg["role"].(string); role == "user" {
		if contentArr, ok := msg["content"].([]interface{}); ok {
			for _, block := range contentArr {
				if blockMap, ok := block.(map[string]interface{}); ok {
					if blockType, _ := blockMap["type"].(string); blockType == "tool_result" {
						if toolUseID, ok := blockMap["tool_use_id"].(string); ok {
							if containsString(toolCallIDs, toolUseID) {
								return true
							}
						}
					}
				}
			}
		}
	}

	return false
}

// containsString checks if a string slice contains a specific string
func containsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
