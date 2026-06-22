package relay

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// KiroRelay handles Kiro (AWS CodeWhisperer) channel requests.
// Ported from 9router executors/kiro.js + translators.
// Supports: text chat + tool_use (function calling) via toolUseEvent.
func KiroRelay(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	info.InitChannelMeta(c)

	textReq, ok := info.Request.(*dto.GeneralOpenAIRequest)
	if !ok {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	creds := parseKiroCredentials(info.ApiKey)
	if creds.AccessToken == "" {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid Kiro credentials"), types.ErrorCodeChannelInvalidKey, http.StatusInternalServerError, types.ErrOptionWithSkipRetry())
	}

	model := info.UpstreamModelName
	if model == "" {
		model = info.OriginModelName
	}

	kiroPayload := buildKiroPayload(model, textReq, creds)

	baseURL := info.ChannelBaseUrl
	if baseURL == "" {
		baseURL = "https://codewhisperer.us-east-1.amazonaws.com"
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/generateAssistantResponse"

	payloadBytes, err := json.Marshal(kiroPayload)
	if err != nil {
		return types.NewError(fmt.Errorf("marshal kiro payload: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	client := &http.Client{
		Timeout: 120 * time.Second,
		Transport: &kiroRelayTransport{},
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return types.NewError(fmt.Errorf("create kiro request: %w", err), types.ErrorCodeDoRequestFailed, types.ErrOptionWithSkipRetry())
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.amazon.eventstream")
	req.Header.Set("X-Amz-Target", "AmazonCodeWhispererStreamingService.GenerateAssistantResponse")
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("Amz-Sdk-Request", "attempt=1; max=3")
	req.Header.Set("Amz-Sdk-Invocation-Id", uuid.New().String())

	resp, err := client.Do(req)
	if err != nil {
		return types.NewError(fmt.Errorf("kiro request failed: %w", err), types.ErrorCodeDoRequestFailed, types.ErrOptionWithSkipRetry())
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		common.SysLog(fmt.Sprintf("[Kiro] upstream error: status=%d body=%s", resp.StatusCode, string(body)))
		return types.NewErrorWithStatusCode(
			fmt.Errorf("kiro upstream error %d: %s", resp.StatusCode, string(body)),
			types.ErrorCodeChannelInvalidKey, resp.StatusCode, types.ErrOptionWithSkipRetry(),
		)
	}

	isStream := textReq.Stream != nil && *textReq.Stream

	if isStream {
		return handleKiroStream(c, info, resp, model)
	}

	return handleKiroNonStream(c, info, resp, model)
}

// kiroToolCallState tracks tool call assembly across streaming events
type kiroToolCallState struct {
	toolCalls    []kiroToolCall
	seenToolIds  map[string]int // toolUseId → index in toolCalls
	nextIndex    int
}

type kiroToolCall struct {
	ID       string
	Name     string
	ArgsBuf  strings.Builder
}

func handleKiroStream(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response, model string) *types.NewAPIError {
	helper.SetEventStreamHeaders(c)

	responseID := fmt.Sprintf("chatcmpl-%d", time.Now().UnixMilli())
	created := time.Now().Unix()
	chunkIndex := 0

	allData, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.NewError(fmt.Errorf("read kiro stream: %w", err), types.ErrorCodeDoRequestFailed, types.ErrOptionWithSkipRetry())
	}

	var content strings.Builder
	toolState := &kiroToolCallState{
		seenToolIds: make(map[string]int),
	}
	offset := 0

	for offset+16 <= len(allData) {
		totalLen := int(binary.BigEndian.Uint32(allData[offset : offset+4]))
		if totalLen < 16 || offset+totalLen > len(allData) {
			break
		}
		frame := allData[offset : offset+totalLen]
		offset += totalLen

		headers, payload := parseKiroEventFrame(frame)
		if payload == nil {
			continue
		}
		eventType := headers[":event-type"]

		switch eventType {
		case "assistantResponseEvent":
			text, _ := payload["content"].(string)
			if text == "" {
				continue
			}
			content.WriteString(text)

			chunk := map[string]interface{}{
				"id":      responseID,
				"object":  "chat.completion.chunk",
				"created": created,
				"model":   model,
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"delta": map[string]interface{}{
							"content": text,
						},
						"finish_reason": nil,
					},
				},
			}
			if chunkIndex == 0 {
				chunk["choices"].([]map[string]interface{})[0]["delta"] = map[string]interface{}{
					"role":    "assistant",
					"content": text,
				}
			}
			chunkIndex++

			chunkBytes, _ := json.Marshal(chunk)
			helper.StringData(c, string(chunkBytes))

		case "toolUseEvent":
			// Kiro toolUseEvent → OpenAI tool_calls
			// Ported from 9router kiro-to-openai.js
			toolUsePayload := payload
			if tu, ok := payload["toolUseEvent"]; ok {
				if m, ok := tu.(map[string]interface{}); ok {
					toolUsePayload = m
				}
			}

			toolCallID, _ := toolUsePayload["toolUseId"].(string)
			if toolCallID == "" {
				toolCallID = fmt.Sprintf("call_%s", uuid.New().String()[:8])
			}
			toolName, _ := toolUsePayload["name"].(string)
			toolInput := toolUsePayload["input"]

			// Check if this is a new tool or continuation
			idx, seen := toolState.seenToolIds[toolCallID]
			if !seen {
				idx = toolState.nextIndex
				toolState.nextIndex++
				toolState.seenToolIds[toolCallID] = idx
				toolState.toolCalls = append(toolState.toolCalls, kiroToolCall{
					ID:   toolCallID,
					Name: toolName,
				})
			}

			// Accumulate input arguments
			var argsStr string
			if toolInput != nil {
				if s, ok := toolInput.(string); ok {
					argsStr = s
				} else {
					argsBytes, _ := json.Marshal(toolInput)
					argsStr = string(argsBytes)
				}
			}
			toolState.toolCalls[idx].ArgsBuf.WriteString(argsStr)

			// Emit tool_calls chunk
			delta := map[string]interface{}{}
			if !seen {
				// First chunk for this tool: include id, type, function name
				delta["tool_calls"] = []map[string]interface{}{
					{
						"index": idx,
						"id":    toolCallID,
						"type":  "function",
						"function": map[string]interface{}{
							"name":      toolName,
							"arguments": argsStr,
						},
					},
				}
			} else {
				// Continuation: only arguments delta
				delta["tool_calls"] = []map[string]interface{}{
					{
						"index": idx,
						"function": map[string]interface{}{
							"arguments": argsStr,
						},
					},
				}
			}

			toolChunk := map[string]interface{}{
				"id":      responseID,
				"object":  "chat.completion.chunk",
				"created": created,
				"model":   model,
				"choices": []map[string]interface{}{
					{
						"index":         0,
						"delta":         delta,
						"finish_reason": nil,
					},
				},
			}
			toolChunkBytes, _ := json.Marshal(toolChunk)
			helper.StringData(c, string(toolChunkBytes))
		}
	}

	// Determine finish reason
	finishReason := "stop"
	if len(toolState.toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	// Final chunk
	finalChunk := map[string]interface{}{
		"id":      responseID,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index":         0,
				"delta":         map[string]interface{}{},
				"finish_reason": finishReason,
			},
		},
	}
	finalBytes, _ := json.Marshal(finalChunk)
	helper.StringData(c, string(finalBytes))
	helper.Done(c)

	return nil
}

func handleKiroNonStream(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response, model string) *types.NewAPIError {
	allData, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.NewError(fmt.Errorf("read kiro response: %w", err), types.ErrorCodeDoRequestFailed, types.ErrOptionWithSkipRetry())
	}

	responseID := fmt.Sprintf("chatcmpl-%d", time.Now().UnixMilli())
	created := time.Now().Unix()

	var content strings.Builder
	toolState := &kiroToolCallState{
		seenToolIds: make(map[string]int),
	}

	offset := 0
	for offset+16 <= len(allData) {
		totalLen := int(binary.BigEndian.Uint32(allData[offset : offset+4]))
		if totalLen < 16 || offset+totalLen > len(allData) {
			break
		}
		frame := allData[offset : offset+totalLen]
		offset += totalLen

		headers, payload := parseKiroEventFrame(frame)
		if payload == nil {
			continue
		}
		eventType := headers[":event-type"]

		switch eventType {
		case "assistantResponseEvent":
			text, _ := payload["content"].(string)
			content.WriteString(text)

		case "toolUseEvent":
			toolUsePayload := payload
			if tu, ok := payload["toolUseEvent"]; ok {
				if m, ok := tu.(map[string]interface{}); ok {
					toolUsePayload = m
				}
			}

			toolCallID, _ := toolUsePayload["toolUseId"].(string)
			if toolCallID == "" {
				toolCallID = fmt.Sprintf("call_%s", uuid.New().String()[:8])
			}
			toolName, _ := toolUsePayload["name"].(string)
			toolInput := toolUsePayload["input"]

			idx, seen := toolState.seenToolIds[toolCallID]
			if !seen {
				idx = toolState.nextIndex
				toolState.nextIndex++
				toolState.seenToolIds[toolCallID] = idx
				toolState.toolCalls = append(toolState.toolCalls, kiroToolCall{
					ID:   toolCallID,
					Name: toolName,
				})
			}

			if toolInput != nil {
				var argsStr string
				if s, ok := toolInput.(string); ok {
					argsStr = s
				} else {
					argsBytes, _ := json.Marshal(toolInput)
					argsStr = string(argsBytes)
				}
				toolState.toolCalls[idx].ArgsBuf.WriteString(argsStr)
			}
		}
	}

	// Build response
	message := map[string]interface{}{
		"role":    "assistant",
		"content": content.String(),
	}

	finishReason := "stop"
	if len(toolState.toolCalls) > 0 {
		finishReason = "tool_calls"
		toolCallsJSON := make([]map[string]interface{}, len(toolState.toolCalls))
		for i, tc := range toolState.toolCalls {
			toolCallsJSON[i] = map[string]interface{}{
				"id":   tc.ID,
				"type": "function",
				"function": map[string]interface{}{
					"name":      tc.Name,
					"arguments": tc.ArgsBuf.String(),
				},
			}
		}
		message["tool_calls"] = toolCallsJSON
	}

	completionTokens := estimateTokens(content.String())

	result := map[string]interface{}{
		"id":      responseID,
		"object":  "chat.completion",
		"created": created,
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index":         0,
				"message":       message,
				"finish_reason": finishReason,
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     0,
			"completion_tokens": completionTokens,
			"total_tokens":      completionTokens,
		},
	}

	c.JSON(http.StatusOK, result)
	return nil
}

func parseKiroEventFrame(data []byte) (map[string]string, map[string]interface{}) {
	if len(data) < 16 {
		return nil, nil
	}
	headersLen := int(binary.BigEndian.Uint32(data[4:8]))
	if 12+headersLen > len(data) {
		return nil, nil
	}

	headers := make(map[string]string)
	offset := 12
	headerEnd := 12 + headersLen
	for offset < headerEnd {
		if offset >= len(data) {
			break
		}
		nameLen := int(data[offset])
		offset++
		if offset+nameLen > len(data) {
			break
		}
		name := string(data[offset : offset+nameLen])
		offset += nameLen

		if offset >= len(data) {
			break
		}
		headerType := data[offset]
		offset++

		if headerType == 7 {
			if offset+2 > len(data) {
				break
			}
			valueLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
			offset += 2
			if offset+valueLen > len(data) {
				break
			}
			value := string(data[offset : offset+valueLen])
			offset += valueLen
			headers[name] = value
		} else {
			break
		}
	}

	payloadStart := 12 + headersLen
	payloadEnd := len(data) - 4
	if payloadEnd <= payloadStart {
		return headers, nil
	}

	payloadBytes := data[payloadStart:payloadEnd]
	if len(payloadBytes) == 0 {
		return headers, nil
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return headers, nil
	}
	return headers, payload
}

type kiroCredentials struct {
	AccessToken  string
	RefreshToken string
	ClientID     string
	ClientSecret string
}

func parseKiroCredentials(key string) kiroCredentials {
	parts := strings.Split(key, "|")
	creds := kiroCredentials{}
	if len(parts) >= 1 {
		creds.AccessToken = strings.TrimSpace(parts[0])
	}
	if len(parts) >= 2 {
		creds.RefreshToken = strings.TrimSpace(parts[1])
	}
	if len(parts) >= 3 {
		creds.ClientID = strings.TrimSpace(parts[2])
	}
	if len(parts) >= 4 {
		creds.ClientSecret = strings.TrimSpace(parts[3])
	}
	return creds
}

// buildKiroPayload converts OpenAI request to Kiro CodeWhisperer format.
// Supports tools: OpenAI tools[] → Kiro userInputMessageContext.tools.
// Tool call history is flattened to text when client doesn't provide tools
// (ported from 9router openai-to-kiro.js flattenToolInteractions).
func buildKiroPayload(model string, req *dto.GeneralOpenAIRequest, creds kiroCredentials) map[string]interface{} {
	messages := req.Messages
	conversationID := uuid.New().String()

	// Check if client provided tools
	hasTools := len(req.Tools) > 0

	var history []interface{}
	var currentContent string
	var systemContent string

	for i, msg := range messages {
		role := msg.Role

		switch role {
		case "system":
			content := extractMessageText(msg)
			systemContent += content + "\n"

		case "user":
			content := extractMessageText(msg)
			if i == len(messages)-1 {
				currentContent = content
			} else {
				histEntry := map[string]interface{}{
					"userInputMessage": map[string]interface{}{
						"content": content,
						"modelId": model,
						"origin":  "AI_EDITOR",
					},
				}
				history = append(history, histEntry)
			}

		case "assistant":
			// If no tools provided, flatten tool_calls to text (9router pattern)
			parsedToolCalls := msg.ParseToolCalls()
			if !hasTools && len(parsedToolCalls) > 0 {
				var parts []string
				textContent := extractMessageText(msg)
				if textContent != "" {
					parts = append(parts, textContent)
				}
				for _, tc := range parsedToolCalls {
					fnName := tc.Function.Name
					fnArgs := tc.Function.Arguments
					parts = append(parts, fmt.Sprintf("[Called tool: %s(%s)]", fnName, fnArgs))
				}
				history = append(history, map[string]interface{}{
					"assistantResponseMessage": map[string]interface{}{
						"content": strings.Join(parts, "\n"),
					},
				})
			} else {
				content := extractMessageText(msg)
				history = append(history, map[string]interface{}{
					"assistantResponseMessage": map[string]interface{}{
						"content": content,
					},
				})
			}

		case "tool":
			// Tool result message
			if hasTools {
				// Forward as structured tool result in history
				toolCallID := msg.ToolCallId
				content := extractMessageText(msg)
				histEntry := map[string]interface{}{
					"userInputMessage": map[string]interface{}{
						"content": content,
						"modelId": model,
						"origin":  "AI_EDITOR",
						"userInputMessageContext": map[string]interface{}{
							"toolResults": []map[string]interface{}{
								{
									"toolUseId": toolCallID,
									"content": map[string]interface{}{
										"json": map[string]interface{}{
											"result": content,
										},
									},
								},
							},
						},
					},
				}
				history = append(history, histEntry)
			} else {
				// No tools: flatten tool result to text (9router pattern)
				content := extractMessageText(msg)
				histEntry := map[string]interface{}{
					"userInputMessage": map[string]interface{}{
						"content": fmt.Sprintf("[Tool result: %s]", content),
						"modelId": model,
						"origin":  "AI_EDITOR",
					},
				}
				history = append(history, histEntry)
			}
		}
	}

	if systemContent != "" {
		currentContent = systemContent + "\n" + currentContent
	}

	profileArn := "arn:aws:codewhisperer:us-east-1:638616132270:profile/AAAACCCCXXXX"

	currentMessage := map[string]interface{}{
		"userInputMessage": map[string]interface{}{
			"content": currentContent,
			"modelId": model,
			"origin":  "AI_EDITOR",
		},
	}

	// Add tools to context if client provided them
	if hasTools {
		kiroTools := convertOpenAIToolsToKiro(req.Tools)
		if len(kiroTools) > 0 {
			userMsg := currentMessage["userInputMessage"].(map[string]interface{})
			userMsg["userInputMessageContext"] = map[string]interface{}{
				"tools": kiroTools,
			}
		}
	}

	payload := map[string]interface{}{
		"conversationState": map[string]interface{}{
			"chatTriggerType": "MANUAL",
			"conversationId":  conversationID,
			"currentMessage":   currentMessage,
			"history":          history,
		},
		"profileArn": profileArn,
	}

	maxTokens := 32000
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		maxTokens = int(*req.MaxTokens)
	}
	inferenceConfig := map[string]interface{}{
		"maxTokens": maxTokens,
	}
	if req.Temperature != nil {
		inferenceConfig["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		inferenceConfig["topP"] = *req.TopP
	}
	payload["inferenceConfig"] = inferenceConfig

	return payload
}

// convertOpenAIToolsToKiro converts OpenAI tool definitions to Kiro format.
// OpenAI: {"type":"function","function":{"name":"...","description":"...","parameters":{...}}}
// Kiro:   {"toolSpecification":{"name":"...","description":"...","inputSchema":{"json":{...}}}}
func convertOpenAIToolsToKiro(tools []dto.ToolCallRequest) []map[string]interface{} {
	var kiroTools []map[string]interface{}
	for _, tool := range tools {
		kiroTool := map[string]interface{}{
			"toolSpecification": map[string]interface{}{
				"name":        tool.Function.Name,
				"description": tool.Function.Description,
				"inputSchema": map[string]interface{}{
					"json": tool.Function.Parameters,
				},
			},
		}
		kiroTools = append(kiroTools, kiroTool)
	}
	return kiroTools
}

// extractMessageText extracts text content from a message, handling all formats
func extractMessageText(msg dto.Message) string {
	// StringContent() handles string and []any content types
	if s := msg.StringContent(); s != "" {
		return s
	}
	// Fallback for other content types
	if msg.Content != nil {
		switch v := msg.Content.(type) {
		case string:
			return v
		case []interface{}:
			var parts []string
			for _, item := range v {
				if m, ok := item.(map[string]interface{}); ok {
					if text, ok := m["text"].(string); ok {
						parts = append(parts, text)
					}
				}
			}
			return strings.Join(parts, "\n")
		}
	}
	return ""
}

func estimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	tokens := len(text) / 4
	if tokens == 0 {
		tokens = 1
	}
	return tokens
}

func IsKiroChannel(channelType int) bool {
	return channelType == constant.ChannelTypeKiro
}

type kiroRelayTransport struct{}

func (t *kiroRelayTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", "AWS-SDK-JS/3.840.0 kiro-ide/0.8.1")
	req.Header.Set("X-Amz-User-Agent", "aws-sdk-js/3.840.0 kiro-ide/0.8.1")
	return http.DefaultTransport.RoundTrip(req)
}
