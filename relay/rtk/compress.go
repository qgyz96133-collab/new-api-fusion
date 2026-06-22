package rtk

// CompressMessages applies RTK compression and caveman mode to messages
// Returns the modified messages array
func CompressMessages(messages []map[string]interface{}, enableRTK bool, cavemanLevel CavemanPromptLevel) []map[string]interface{} {
	// Apply RTK compression to tool results
	if enableRTK {
		messages = ApplyToMessages(messages)
	}

	// Inject caveman prompt if enabled
	if IsCavemanEnabled(cavemanLevel) {
		messages = InjectCavemanPrompt(messages, cavemanLevel)
	}

	return messages
}

// CompressClaudeMessages applies RTK compression for Claude format
func CompressClaudeMessages(messages []map[string]interface{}, enableRTK bool, cavemanLevel CavemanPromptLevel) []map[string]interface{} {
	// Apply RTK compression to tool_result blocks
	if enableRTK {
		messages = ApplyToClaudeMessages(messages)
	}

	// Inject caveman prompt if enabled
	if IsCavemanEnabled(cavemanLevel) {
		messages = InjectCavemanIntoClaude(messages, cavemanLevel)
	}

	return messages
}

// Stats holds compression statistics
type Stats struct {
	OriginalSize   int
	CompressedSize int
	Savings        float64
}

// CompressWithStats applies compression and returns stats
func CompressWithStats(messages []map[string]interface{}, enableRTK bool, cavemanLevel CavemanPromptLevel) ([]map[string]interface{}, Stats) {
	originalSize := estimateSize(messages)

	compressed := CompressMessages(messages, enableRTK, cavemanLevel)
	compressedSize := estimateSize(compressed)

	savings := 0.0
	if originalSize > 0 {
		savings = float64(originalSize-compressedSize) / float64(originalSize) * 100
	}

	return compressed, Stats{
		OriginalSize:   originalSize,
		CompressedSize: compressedSize,
		Savings:        savings,
	}
}

// estimateSize estimates the byte size of messages
func estimateSize(messages []map[string]interface{}) int {
	size := 0
	for _, msg := range messages {
		if content, ok := msg["content"].(string); ok {
			size += len(content)
		} else if contentArr, ok := msg["content"].([]interface{}); ok {
			for _, block := range contentArr {
				if blockMap, ok := block.(map[string]interface{}); ok {
					if text, ok := blockMap["text"].(string); ok {
						size += len(text)
					}
				}
			}
		}
	}
	return size
}
