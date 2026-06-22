package rtk

// InjectCavemanPrompt injects caveman system prompt into messages
func InjectCavemanPrompt(messages []map[string]interface{}, level CavemanPromptLevel) []map[string]interface{} {
	if !IsCavemanEnabled(level) {
		return messages
	}

	prompt := GetCavemanPrompt(level)
	if prompt == "" {
		return messages
	}

	// Find or create system message
	systemIdx := -1
	for i, msg := range messages {
		if role, ok := msg["role"].(string); ok && role == "system" {
			systemIdx = i
			break
		}
	}

	if systemIdx >= 0 {
		// Append to existing system message
		existingContent, _ := messages[systemIdx]["content"].(string)
		messages[systemIdx]["content"] = existingContent + "\n\n" + prompt
	} else {
		// Prepend new system message
		systemMsg := map[string]interface{}{
			"role":    "system",
			"content": prompt,
		}
		messages = append([]map[string]interface{}{systemMsg}, messages...)
	}

	return messages
}

// InjectCavemanIntoClaude injects caveman prompt into Claude format messages
func InjectCavemanIntoClaude(messages []map[string]interface{}, level CavemanPromptLevel) []map[string]interface{} {
	if !IsCavemanEnabled(level) {
		return messages
	}

	prompt := GetCavemanPrompt(level)
	if prompt == "" {
		return messages
	}

	// Claude format: inject into first user message
	for i, msg := range messages {
		role, _ := msg["role"].(string)
		if role == "user" {
			content, ok := msg["content"].([]interface{})
			if !ok {
				continue
			}

			// Prepend system instruction as text block
			systemBlock := map[string]interface{}{
				"type": "text",
				"text": "<system-reminder>" + prompt + "</system-reminder>\n\n",
			}
			newContent := append([]interface{}{systemBlock}, content...)
			messages[i]["content"] = newContent
			break
		}
	}

	return messages
}

// InjectCavemanIntoGemini injects caveman prompt into Gemini format requests
// Gemini uses systemInstruction.parts[].text for system instructions
func InjectCavemanIntoGemini(request map[string]interface{}, level CavemanPromptLevel) map[string]interface{} {
	if !IsCavemanEnabled(level) {
		return request
	}

	prompt := GetCavemanPrompt(level)
	if prompt == "" {
		return request
	}

	// Check for nested request (Antigravity wraps in body.request)
	target := request
	if innerReq, ok := request["request"].(map[string]interface{}); ok {
		target = innerReq
	}

	// Try both systemInstruction and system_instruction
	for _, key := range []string{"systemInstruction", "system_instruction"} {
		if sysInstr, ok := target[key].(map[string]interface{}); ok {
			if parts, ok := sysInstr["parts"].([]interface{}); ok {
				sysInstr["parts"] = append(parts, map[string]interface{}{"text": prompt})
				return request
			}
		}
	}

	// No existing systemInstruction — create one
	target["systemInstruction"] = map[string]interface{}{
		"parts": []interface{}{
			map[string]interface{}{"text": prompt},
		},
	}

	return request
}
