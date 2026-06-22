package rtk

// Apply compresses content using autodetection with safety guards
func Apply(content string) string {
	if len(content) < MinCompressSize {
		return content
	}

	filter := AutodetectFilter(content)
	if filter == nil {
		return content
	}

	result := filter(content)

	// Safety: never return empty, never grow the input
	if len(result) == 0 || len(result) >= len(content) {
		return content
	}

	return result
}

// ApplyToToolResult compresses tool result content
func ApplyToToolResult(content string) string {
	return Apply(content)
}

// ApplyToMessages compresses all tool_result messages in the message array (OpenAI format)
func ApplyToMessages(messages []map[string]interface{}) []map[string]interface{} {
	for i, msg := range messages {
		role, _ := msg["role"].(string)

		// Only compress tool results
		if role != "tool" {
			continue
		}

		// Shape 1: OpenAI tool string content — { role:"tool", content: "string" }
		if content, ok := msg["content"].(string); ok {
			messages[i]["content"] = Apply(content)
			continue
		}

		// Shape 2: OpenAI tool array content — { role:"tool", content:[{type:"text", text:"..."}] }
		if contentArr, ok := msg["content"].([]interface{}); ok {
			for k, part := range contentArr {
				if partMap, ok := part.(map[string]interface{}); ok {
					if partType, _ := partMap["type"].(string); partType == "text" {
						if text, ok := partMap["text"].(string); ok {
							compressed := Apply(text)
							partMap["text"] = compressed
							contentArr[k] = partMap
						}
					}
				}
			}
			messages[i]["content"] = contentArr
		}
	}
	return messages
}

// ApplyToClaudeMessages compresses tool_result blocks in Claude format messages
func ApplyToClaudeMessages(messages []map[string]interface{}) []map[string]interface{} {
	for i, msg := range messages {
		contentBlocks, ok := msg["content"].([]interface{})
		if !ok {
			continue
		}

		for j, block := range contentBlocks {
			blockMap, ok := block.(map[string]interface{})
			if !ok {
				continue
			}

			blockType, _ := blockMap["type"].(string)
			if blockType != "tool_result" {
				continue
			}

			// Skip error blocks — preserve error traces
			if isError, _ := blockMap["is_error"].(bool); isError {
				continue
			}

			// Shape 1: Claude string form — { type:"tool_result", content: "string" }
			if contentStr, ok := blockMap["content"].(string); ok {
				compressed := Apply(contentStr)
				contentBlocks[j] = map[string]interface{}{
					"type":    "tool_result",
					"content": compressed,
				}
				continue
			}

			// Shape 2: Claude array form — { type:"tool_result", content:[{type:"text", text:"..."}] }
			if contentArr, ok := blockMap["content"].([]interface{}); ok {
				for k, part := range contentArr {
					if partMap, ok := part.(map[string]interface{}); ok {
						if partType, _ := partMap["type"].(string); partType == "text" {
							if text, ok := partMap["text"].(string); ok {
								partMap["text"] = Apply(text)
								contentArr[k] = partMap
							}
						}
					}
				}
				blockMap["content"] = contentArr
				contentBlocks[j] = blockMap
			}
		}
		messages[i]["content"] = contentBlocks
	}
	return messages
}
