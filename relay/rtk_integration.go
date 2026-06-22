package relay

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/rtk"
	"github.com/QuantumNous/new-api/relay/translate"
)

// ApplyOpenAITranslationFixes applies all translation fixes for OpenAI format requests
func ApplyOpenAITranslationFixes(requestBody []byte) ([]byte, error) {
	var request map[string]interface{}
	if err := json.Unmarshal(requestBody, &request); err != nil {
		return requestBody, err
	}

	messages, ok := request["messages"].([]interface{})
	if !ok {
		return requestBody, nil
	}

	messages = translate.FixOrphanToolResults(messages)
	request["messages"] = messages

	return json.Marshal(request)
}

// ApplyClaudeTranslationFixes applies Claude-specific translation fixes
func ApplyClaudeTranslationFixes(requestBody []byte, model string) ([]byte, error) {
	var request map[string]interface{}
	if err := json.Unmarshal(requestBody, &request); err != nil {
		return requestBody, err
	}

	request = translate.NormalizeClaudePassthrough(request, model)

	return json.Marshal(request)
}

// ApplyGeminiTranslationFixes applies Gemini-specific translation fixes
func ApplyGeminiTranslationFixes(requestBody []byte) ([]byte, error) {
	var request map[string]interface{}
	if err := json.Unmarshal(requestBody, &request); err != nil {
		return requestBody, err
	}

	translate.CleanGeminiSchema(request)

	return json.Marshal(request)
}

// rtkStats holds compression statistics for logging
type rtkStats struct {
	bytesBefore int
	bytesAfter  int
	hits        []rtkHit
}

type rtkHit struct {
	shape  string
	filter string
	saved  int
}

func (s *rtkStats) addHit(shape, filter string, bytesIn, bytesOut int) {
	s.bytesBefore += bytesIn
	if bytesOut < bytesIn && bytesOut > 0 {
		s.bytesAfter += bytesOut
		s.hits = append(s.hits, rtkHit{shape: shape, filter: filter, saved: bytesIn - bytesOut})
	} else {
		s.bytesAfter += bytesIn
	}
}

func formatRtkLog(stats *rtkStats, format string) string {
	if stats == nil || len(stats.hits) == 0 {
		return ""
	}
	saved := stats.bytesBefore - stats.bytesAfter
	pct := 0.0
	if stats.bytesBefore > 0 {
		pct = float64(saved) / float64(stats.bytesBefore) * 100
	}
	filterSet := make(map[string]bool)
	for _, h := range stats.hits {
		filterSet[h.filter] = true
	}
	filters := make([]string, 0, len(filterSet))
	for f := range filterSet {
		filters = append(filters, f)
	}
	return fmt.Sprintf("[RTK] %s: saved %dB / %dB (%.1f%%) via [%s] hits=%d",
		format, saved, stats.bytesBefore, pct, strings.Join(filters, ","), len(stats.hits))
}

// compressTextWithStats compresses a single text string with stats tracking
func compressTextWithStats(text string, stats *rtkStats, shape string) string {
	bytesIn := len(text)
	if bytesIn < rtk.MinCompressSize {
		stats.bytesBefore += bytesIn
		stats.bytesAfter += bytesIn
		return text
	}

	filter, filterN := rtk.AutodetectFilterNamed(text)
	if filter == nil {
		stats.bytesBefore += bytesIn
		stats.bytesAfter += bytesIn
		return text
	}

	result := filter(text)
	stats.addHit(shape, filterN, bytesIn, len(result))

	// Safety: never return empty, never grow
	if len(result) == 0 || len(result) >= bytesIn {
		stats.bytesAfter -= len(result)
		stats.bytesAfter += bytesIn
		if len(stats.hits) > 0 {
			stats.hits = stats.hits[:len(stats.hits)-1]
		}
		return text
	}

	return result
}


// ---- OpenAI format (messages) ----

func applyOpenAIRTKCompression(requestBody []byte, enableRTK bool, cavemanLevel int) ([]byte, string, error) {
	if !enableRTK && cavemanLevel == 0 {
		return requestBody, "", nil
	}

	var request map[string]interface{}
	if err := json.Unmarshal(requestBody, &request); err != nil {
		return requestBody, "", err
	}

	// Check for OpenAI Responses API format: function_call_output
	if input, ok := request["input"].([]interface{}); ok {
		return applyOpenAIResponsesRTK(request, input, enableRTK, cavemanLevel)
	}

	// Standard OpenAI messages format
	messages, ok := request["messages"].([]interface{})
	if !ok {
		return requestBody, "", nil
	}

	stats := &rtkStats{}

	if enableRTK {
		for i, msg := range messages {
			msgMap, ok := msg.(map[string]interface{})
			if !ok {
				continue
			}
			role, _ := msgMap["role"].(string)
			if role != "tool" {
				continue
			}

			// Shape 1: string content
			if content, ok := msgMap["content"].(string); ok {
				msgMap["content"] = compressTextWithStats(content, stats, "openai-tool")
				messages[i] = msgMap
				continue
			}

			// Shape 2: array content [{type:"text", text:"..."}]
			if contentArr, ok := msgMap["content"].([]interface{}); ok {
				for k, part := range contentArr {
					if partMap, ok := part.(map[string]interface{}); ok {
						if partType, _ := partMap["type"].(string); partType == "text" {
							if text, ok := partMap["text"].(string); ok {
								partMap["text"] = compressTextWithStats(text, stats, "openai-tool-array")
								contentArr[k] = partMap
							}
						}
					}
				}
				msgMap["content"] = contentArr
				messages[i] = msgMap
			}
		}
	}

	// Caveman injection for OpenAI format
	rtkLevel := rtk.CavemanPromptLevel(cavemanLevel)
	if rtk.IsCavemanEnabled(rtkLevel) {
		var messageMaps []map[string]interface{}
		for _, msg := range messages {
			if msgMap, ok := msg.(map[string]interface{}); ok {
				messageMaps = append(messageMaps, msgMap)
			}
		}
		messageMaps = rtk.InjectCavemanPrompt(messageMaps, rtkLevel)
		var newMessages []interface{}
		for _, msg := range messageMaps {
			newMessages = append(newMessages, msg)
		}
		messages = newMessages
	}

	request["messages"] = messages
	result, err := json.Marshal(request)
	return result, formatRtkLog(stats, "openai"), err
}

// applyOpenAIResponsesRTK handles OpenAI Responses API format
func applyOpenAIResponsesRTK(request map[string]interface{}, input []interface{}, enableRTK bool, cavemanLevel int) ([]byte, string, error) {
	stats := &rtkStats{}

	if enableRTK {
		for i, item := range input {
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			// function_call_output: { type:"function_call_output", output: string | [{type:"input_text", text}] }
			if itemType, _ := itemMap["type"].(string); itemType == "function_call_output" {
				// String output
				if output, ok := itemMap["output"].(string); ok {
					itemMap["output"] = compressTextWithStats(output, stats, "openai-responses-string")
					input[i] = itemMap
					continue
				}

				// Array output
				if outputArr, ok := itemMap["output"].([]interface{}); ok {
					for k, part := range outputArr {
						if partMap, ok := part.(map[string]interface{}); ok {
							if partType, _ := partMap["type"].(string); partType == "input_text" {
								if text, ok := partMap["text"].(string); ok {
									partMap["text"] = compressTextWithStats(text, stats, "openai-responses-array")
									outputArr[k] = partMap
								}
							}
						}
					}
					itemMap["output"] = outputArr
					input[i] = itemMap
				}
			}
		}
	}

	// Caveman: inject into instructions field
	rtkLevel := rtk.CavemanPromptLevel(cavemanLevel)
	if rtk.IsCavemanEnabled(rtkLevel) {
		prompt := rtk.GetCavemanPrompt(rtkLevel)
		if instructions, ok := request["instructions"].(string); ok && prompt != "" {
			if instructions != "" {
				request["instructions"] = instructions + "\n\n" + prompt
			} else {
				request["instructions"] = prompt
			}
		}
	}

	request["input"] = input
	result, err := json.Marshal(request)
	return result, formatRtkLog(stats, "openai-responses"), err
}

// ---- Claude format ----

func applyClaudeRTKCompression(requestBody []byte, enableRTK bool, cavemanLevel int) ([]byte, string, error) {
	if !enableRTK && cavemanLevel == 0 {
		return requestBody, "", nil
	}

	var request map[string]interface{}
	if err := json.Unmarshal(requestBody, &request); err != nil {
		return requestBody, "", err
	}

	messages, ok := request["messages"].([]interface{})
	if !ok {
		return requestBody, "", nil
	}

	stats := &rtkStats{}

	if enableRTK {
		for i, msg := range messages {
			msgMap, ok := msg.(map[string]interface{})
			if !ok {
				continue
			}

			contentBlocks, ok := msgMap["content"].([]interface{})
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

				// Skip error blocks
				if isError, _ := blockMap["is_error"].(bool); isError {
					continue
				}

				// Shape 1: string content
				if contentStr, ok := blockMap["content"].(string); ok {
					blockMap["content"] = compressTextWithStats(contentStr, stats, "claude-string")
					contentBlocks[j] = blockMap
					continue
				}

				// Shape 2: array content [{type:"text", text:"..."}]
				if contentArr, ok := blockMap["content"].([]interface{}); ok {
					for k, part := range contentArr {
						if partMap, ok := part.(map[string]interface{}); ok {
							if partType, _ := partMap["type"].(string); partType == "text" {
								if text, ok := partMap["text"].(string); ok {
									partMap["text"] = compressTextWithStats(text, stats, "claude-array")
									contentArr[k] = partMap
								}
							}
						}
					}
					blockMap["content"] = contentArr
					contentBlocks[j] = blockMap
				}
			}
			msgMap["content"] = contentBlocks
			messages[i] = msgMap
		}
	}

	request["messages"] = messages

	// Claude caveman injection: use body.system field (Claude's native system prompt)
	rtkLevel := rtk.CavemanPromptLevel(cavemanLevel)
	if rtk.IsCavemanEnabled(rtkLevel) {
		prompt := rtk.GetCavemanPrompt(rtkLevel)
		if prompt != "" {
			injectClaudeSystemPrompt(request, prompt)
		}
	}

	result, err := json.Marshal(request)
	return result, formatRtkLog(stats, "claude"), err
}

// injectClaudeSystemPrompt injects caveman prompt into Claude's body.system field
func injectClaudeSystemPrompt(request map[string]interface{}, prompt string) {
	sep := "\n\n"

	// body.system as string
	if sysStr, ok := request["system"].(string); ok {
		if sysStr != "" {
			request["system"] = sysStr + sep + prompt
		} else {
			request["system"] = prompt
		}
		return
	}

	// body.system as array of {type:"text", text:"...", cache_control?:...}
	if sysArr, ok := request["system"].([]interface{}); ok {
		newBlock := map[string]interface{}{"type": "text", "text": prompt}

		// Insert before the last cache_control block to keep caveman inside cached prefix
		lastCacheIdx := -1
		for i := len(sysArr) - 1; i >= 0; i-- {
			if block, ok := sysArr[i].(map[string]interface{}); ok {
				if _, hasCC := block["cache_control"]; hasCC {
					lastCacheIdx = i
					break
				}
			}
		}

		if lastCacheIdx >= 0 {
			// Insert before the cached block
			sysArr = append(sysArr[:lastCacheIdx], append([]interface{}{newBlock}, sysArr[lastCacheIdx:]...)...)
		} else {
			sysArr = append(sysArr, newBlock)
		}
		request["system"] = sysArr
		return
	}

	// No system field — create one
	request["system"] = prompt
}

// ---- Gemini format ----

func applyGeminiRTKCompression(requestBody []byte, enableRTK bool, cavemanLevel int) ([]byte, string, error) {
	if !enableRTK && cavemanLevel == 0 {
		return requestBody, "", nil
	}

	var request map[string]interface{}
	if err := json.Unmarshal(requestBody, &request); err != nil {
		return requestBody, "", err
	}

	stats := &rtkStats{}

	if enableRTK {
		applyGeminiContentsCompression(request, stats)
	}

	// Gemini caveman injection
	rtkLevel := rtk.CavemanPromptLevel(cavemanLevel)
	if rtk.IsCavemanEnabled(rtkLevel) {
		request = rtk.InjectCavemanIntoGemini(request, rtkLevel)
	}

	result, err := json.Marshal(request)
	return result, formatRtkLog(stats, "gemini"), err
}

func applyGeminiContentsCompression(request map[string]interface{}, stats *rtkStats) {
	contents, ok := request["contents"].([]interface{})
	if !ok {
		return
	}

	for i, content := range contents {
		contentMap, ok := content.(map[string]interface{})
		if !ok {
			continue
		}

		parts, ok := contentMap["parts"].([]interface{})
		if !ok {
			continue
		}

		for j, part := range parts {
			partMap, ok := part.(map[string]interface{})
			if !ok {
				continue
			}

			// Gemini function response: {functionResponse: {response: {content: "..."}}}
			if funcResp, ok := partMap["functionResponse"].(map[string]interface{}); ok {
				if resp, ok := funcResp["response"].(map[string]interface{}); ok {
					if contentStr, ok := resp["content"].(string); ok {
						resp["content"] = compressTextWithStats(contentStr, stats, "gemini-function-response")
					}
				}
			}

			parts[j] = partMap
		}
		contentMap["parts"] = parts
		contents[i] = contentMap
	}

	request["contents"] = contents
}


// ---- Kiro format ----

// ApplyKiroRTKCompression handles Kiro IDE conversationState format
func ApplyKiroRTKCompression(requestBody []byte, enableRTK bool, cavemanLevel int) ([]byte, string, error) {
	if !enableRTK {
		return requestBody, "", nil
	}

	var request map[string]interface{}
	if err := json.Unmarshal(requestBody, &request); err != nil {
		return requestBody, "", err
	}

	state, ok := request["conversationState"].(map[string]interface{})
	if !ok {
		return requestBody, "", nil
	}

	stats := &rtkStats{}

	// Collect all messages: history + currentMessage
	var allMessages []interface{}
	if history, ok := state["history"].([]interface{}); ok {
		allMessages = append(allMessages, history...)
	}
	if currentMsg := state["currentMessage"]; currentMsg != nil {
		allMessages = append(allMessages, currentMsg)
	}

	for _, msg := range allMessages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}
		userInput := msgMap["userInputMessage"]
		if userInput == nil {
			continue
		}
		userInputMap, ok := userInput.(map[string]interface{})
		if !ok {
			continue
		}
		ctx := userInputMap["userInputMessageContext"]
		if ctx == nil {
			continue
		}
		ctxMap, ok := ctx.(map[string]interface{})
		if !ok {
			continue
		}
		toolResults, ok := ctxMap["toolResults"].([]interface{})
		if !ok {
			continue
		}

		for _, tr := range toolResults {
			trMap, ok := tr.(map[string]interface{})
			if !ok {
				continue
			}
			// Preserve error traces
			if status, _ := trMap["status"].(string); status == "error" {
				continue
			}
			contentArr, ok := trMap["content"].([]interface{})
			if !ok {
				continue
			}
			for k, part := range contentArr {
				partMap, ok := part.(map[string]interface{})
				if !ok {
					continue
				}
				if text, ok := partMap["text"].(string); ok {
					partMap["text"] = compressTextWithStats(text, stats, "kiro-tool-result")
					contentArr[k] = partMap
				}
			}
		}
	}

	result, err := json.Marshal(request)
	return result, formatRtkLog(stats, "kiro"), err
}

// ---- Provider Thinking Config Injection ----

// InjectProviderThinking injects provider-level thinking config when client hasn't set it.
// Ported from 9router chatCore.js provider thinking override.
func InjectProviderThinking(requestBody []byte, thinkingMode string) ([]byte, error) {
	if thinkingMode == "" || thinkingMode == "auto" {
		return requestBody, nil
	}

	var request map[string]interface{}
	if err := json.Unmarshal(requestBody, &request); err != nil {
		return requestBody, err
	}

	switch thinkingMode {
	case "on":
		if _, hasThinking := request["thinking"]; !hasThinking {
			request["thinking"] = map[string]interface{}{
				"type":          "enabled",
				"budget_tokens": 10000,
			}
		}
	case "off":
		if _, hasThinking := request["thinking"]; !hasThinking {
			request["thinking"] = map[string]interface{}{
				"type": "disabled",
			}
		}
	default:
		// low/medium/high → reasoning_effort
		if _, hasEffort := request["reasoning_effort"]; !hasEffort {
			request["reasoning_effort"] = thinkingMode
		}
	}

	return json.Marshal(request)
}

// ---- ApplyAllFixes: main entry point ----

// ApplyAllFixes applies all translation fixes and RTK compression in the correct order.
// Format-aware: dispatches to the appropriate handler based on request format.
func ApplyAllFixes(requestBody []byte, format string, model string, enableRTK bool, cavemanLevel int) ([]byte, error) {
	var err error
	var logLine string

	// Apply format-specific translation fixes first
	switch format {
	case "openai":
		requestBody, err = ApplyOpenAITranslationFixes(requestBody)
	case "claude":
		requestBody, err = ApplyClaudeTranslationFixes(requestBody, model)
	case "gemini":
		requestBody, err = ApplyGeminiTranslationFixes(requestBody)
	}
	if err != nil {
		return requestBody, err
	}

	// Tool deduplication (reduce tool definitions token bloat)
	requestBody, _ = applyToolDedup(requestBody)

	// Claude tool parameter mapping (paths→path, description→pattern)
	// Fixes Claude Code CLI compatibility issues
	if format == "claude" {
		requestBody, _ = applyClaudeToolParamFix(requestBody)
	}

	// Apply format-specific RTK compression and caveman injection
	switch format {
	case "claude":
		requestBody, logLine, err = applyClaudeRTKCompression(requestBody, enableRTK, cavemanLevel)
	case "gemini":
		requestBody, logLine, err = applyGeminiRTKCompression(requestBody, enableRTK, cavemanLevel)
	case "kiro":
		requestBody, logLine, err = ApplyKiroRTKCompression(requestBody, enableRTK, cavemanLevel)
	default:
		// Check if this is actually Kiro format (has conversationState)
		if isKiroFormat(requestBody) {
			requestBody, logLine, err = ApplyKiroRTKCompression(requestBody, enableRTK, cavemanLevel)
		} else {
			requestBody, logLine, err = applyOpenAIRTKCompression(requestBody, enableRTK, cavemanLevel)
		}
	}

	// Log RTK stats if compression was applied
	if logLine != "" && common.DebugEnabled {
		common.SysLog(logLine)
	}

	return requestBody, err
}

// applyToolDedup applies tool deduplication to reduce token bloat
func applyToolDedup(requestBody []byte) ([]byte, string) {
	var request map[string]interface{}
	if err := json.Unmarshal(requestBody, &request); err != nil {
		return requestBody, ""
	}

	tools, ok := request["tools"].([]interface{})
	if !ok || len(tools) == 0 {
		return requestBody, ""
	}

	deduped, stripped := translate.DedupeTools(tools)
	if len(stripped) == 0 {
		return requestBody, ""
	}

	request["tools"] = deduped
	result, err := json.Marshal(request)
	if err != nil {
		return requestBody, ""
	}

	logMsg := fmt.Sprintf("[ToolDedup] stripped %d duplicate tools: %s", len(stripped), strings.Join(stripped, ", "))
	return result, logMsg
}

// isKiroFormat checks if the request body contains Kiro conversationState
func isKiroFormat(requestBody []byte) bool {
	// Quick check without full unmarshal
	return len(requestBody) > 30 &&
		(strings.Contains(string(requestBody[:min(200, len(requestBody))]), "conversationState"))
}

// applyClaudeToolParamFix applies Claude-specific tool parameter mapping fixes
func applyClaudeToolParamFix(requestBody []byte) ([]byte, string) {
	var request map[string]interface{}
	if err := json.Unmarshal(requestBody, &request); err != nil {
		return requestBody, ""
	}

	tools, ok := request["tools"].([]interface{})
	if !ok || len(tools) == 0 {
		return requestBody, ""
	}

	fixed := translate.FixClaudeToolParameters(tools)
	if len(fixed) == 0 {
		return requestBody, ""
	}

	request["tools"] = fixed
	result, err := json.Marshal(request)
	if err != nil {
		return requestBody, ""
	}

	logMsg := "[ClaudeToolFix] applied paths→path parameter mapping"
	return result, logMsg
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
