package qoder

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Adaptor struct {
	creds *Creds
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	key := info.ApiKey
	if key == "" {
		return
	}
	creds, err := ParseCredsFromKey(key)
	if err != nil {
		return
	}
	a.creds = creds
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return QoderChatURLEncoded, nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, fmt.Errorf("request is nil")
	}
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	if a.creds == nil {
		return nil, fmt.Errorf("Qoder credentials not initialized")
	}

	// Read the OpenAI-format request body
	bodyBytes, err := io.ReadAll(requestBody)
	if err != nil {
		return nil, err
	}

	// Parse OpenAI request
	var oaiReq map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &oaiReq); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI request: %w", err)
	}

	// Build Qoder request body
	model := "auto"
	if m, ok := oaiReq["model"].(string); ok {
		model = mapModel(m)
	}

	messages, systemText := normalizeMessages(oaiReq)
	maxTokens := 32768
	if mt, ok := oaiReq["max_tokens"].(float64); ok && mt > 0 && int(mt) < maxTokens {
		maxTokens = int(mt)
	}

	sessionID := stableHash("qoder-session", a.creds.UserID, model)
	recordID := stableHash("qoder-record", model, fmt.Sprintf("%v", messages))
	lastUser := lastUserText(messages)

	qoderPayload := map[string]interface{}{
		"request_id":      uuid.New().String(),
		"request_set_id":  recordID,
		"chat_record_id":  recordID,
		"session_id":      sessionID,
		"stream":          true,
		"chat_task":       "FREE_INPUT",
		"is_reply":        true,
		"is_retry":        false,
		"source":          1,
		"version":         "3",
		"session_type":    "qodercli",
		"agent_id":        "agent_common",
		"task_id":         "common",
		"code_language":   "",
		"chat_prompt":     "",
		"image_urls":      nil,
		"aliyun_user_type": "",
		"system":          systemText,
		"messages":        messages,
		"tools":           []interface{}{},
		"parameters":      map[string]interface{}{"max_tokens": maxTokens},
		"chat_context": map[string]interface{}{
			"chatPrompt": "",
			"imageUrls":  nil,
			"extra": map[string]interface{}{
				"context":         []interface{}{},
				"modelConfig":     map[string]interface{}{"key": model, "is_reasoning": false},
				"originalContent": lastUser,
			},
			"features": []interface{}{},
			"text":     lastUser,
		},
		"model_config": func() map[string]interface{} {
			mc := GetModelConfig(a.creds, model)
			return map[string]interface{}{
				"key":               mc.Key,
				"is_reasoning":      mc.IsReasoning,
				"max_output_tokens": mc.MaxOutputTokens,
				"source":            mc.Source,
			}
		}(),
		"business": map[string]interface{}{
			"product":  "cli",
			"version":  "1.0.0",
			"type":     "agent",
			"stage":    "start",
			"id":       uuid.New().String(),
			"name":     truncate(lastUser, 30),
			"begin_at": time.Now().UnixMilli(),
		},
	}

	// Encode the body with WAF-bypass encoding
	plainBody, _ := json.Marshal(qoderPayload)
	encodedBody := QoderEncodeBody(plainBody)
	encodedBodyBytes := []byte(encodedBody)

	// Build COSY headers from the ENCODED body
	requestURL := QoderChatURLEncoded
	headers, err := BuildCosyHeaders(encodedBodyBytes, requestURL, a.creds)
	if err != nil {
		return nil, fmt.Errorf("COSY signing failed: %w", err)
	}

	// Add Qoder-specific headers
	headers["Accept"] = "text/event-stream"
	headers["Accept-Encoding"] = "identity"
	headers["Cache-Control"] = "no-cache"
	headers["X-Model-Key"] = model
	headers["X-Model-Source"] = "system"

	req, err := http.NewRequestWithContext(c.Request.Context(), "POST", requestURL, bytes.NewReader(encodedBodyBytes))
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := service.GetHttpClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if resp == nil {
		return nil, types.NewError(fmt.Errorf("nil response from Qoder"), types.ErrorCodeDoRequestFailed)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, types.NewError(fmt.Errorf("Qoder returned %d: %s", resp.StatusCode, string(body[:min(500, len(body))])), types.ErrorCodeDoRequestFailed)
	}

	// 从 context 读取客户端的原始 stream 请求（在 GenRelayInfo 中设置的）
	// 不能用 info.IsStream，因为它会被 compatible_handler.go:212 用响应头覆盖
	clientWantsStream := c.GetBool(string(constant.ContextKeyIsStream))

	// 根据客户端的原始意图决定处理方式
	if clientWantsStream {
		return a.handleStreamResponse(c, resp, info)
	}
	return a.handleNonStreamResponse(c, resp, info)
}

func (a *Adaptor) handleStreamResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.WriteHeader(http.StatusOK)

	scanner := bufio.NewScanner(resp.Body)
	var lastUsage *dto.Usage

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(line[5:])
		if data == "[DONE]" {
			fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
			c.Writer.Flush()
			break
		}

		// Unwrap Qoder envelope: {"statusCodeValue":200,"body":"{...}"}
		var envelope struct {
			StatusCodeValue int    `json:"statusCodeValue"`
			Body            string `json:"body"`
		}
		if json.Unmarshal([]byte(data), &envelope) == nil && envelope.Body != "" {
			if envelope.Body == "[DONE]" {
				fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
				c.Writer.Flush()
				break
			}
			// Forward the inner body as-is (it's OpenAI SSE format)
			fmt.Fprintf(c.Writer, "data: %s\n\n", envelope.Body)
			c.Writer.Flush()

			// Try to extract usage from the chunk
			var chunk struct {
				Usage *dto.Usage `json:"usage"`
			}
			if json.Unmarshal([]byte(envelope.Body), &chunk) == nil && chunk.Usage != nil && chunk.Usage.TotalTokens > 0 {
				lastUsage = chunk.Usage
			}
		}
	}

	if lastUsage == nil {
		lastUsage = &dto.Usage{}
	}
	return lastUsage, nil
}

func (a *Adaptor) handleNonStreamResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewError(readErr, types.ErrorCodeReadResponseBodyFailed)
	}

	// Parse SSE stream, unwrap Qoder envelope, and accumulate into JSON response
	var contentBuilder strings.Builder
	var reasoningBuilder strings.Builder
	toolCallsMap := make(map[int]*dto.ToolCallResponse)
	var finishReason string
	var usage *dto.Usage
	var firstID string
	var model string
	var created int64

	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(line[5:])
		if data == "[DONE]" {
			break
		}

		// Unwrap Qoder envelope: {"statusCodeValue":200,"body":"{...}"}
		var envelope struct {
			StatusCodeValue int    `json:"statusCodeValue"`
			Body            string `json:"body"`
		}
		var chunkData []byte
		if json.Unmarshal([]byte(data), &envelope) == nil && envelope.Body != "" {
			if envelope.Body == "[DONE]" {
				break
			}
			chunkData = []byte(envelope.Body)
		} else {
			chunkData = []byte(data)
		}

		// Parse OpenAI stream chunk
		var chunk dto.ChatCompletionsStreamResponse
		if json.Unmarshal(chunkData, &chunk) != nil {
			continue
		}

		if firstID == "" {
			firstID = chunk.Id
		}
		if model == "" {
			model = chunk.Model
		}
		if created == 0 {
			created = chunk.Created
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}

		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta
			if content := delta.GetContentString(); content != "" {
				contentBuilder.WriteString(content)
			}
			if reasoning := delta.GetReasoningContent(); reasoning != "" {
				reasoningBuilder.WriteString(reasoning)
			}
			if chunk.Choices[0].FinishReason != nil && *chunk.Choices[0].FinishReason != "" {
				finishReason = *chunk.Choices[0].FinishReason
			}

			// Accumulate tool calls by index
			for _, tc := range delta.ToolCalls {
				idx := 0
				if tc.Index != nil {
					idx = *tc.Index
				}
				if _, ok := toolCallsMap[idx]; !ok {
					toolCallsMap[idx] = &dto.ToolCallResponse{
						Index:    &idx,
						ID:       tc.ID,
						Type:     tc.Type,
						Function: dto.FunctionResponse{},
					}
				}
				if tc.ID != "" {
					toolCallsMap[idx].ID = tc.ID
				}
				if tc.Function.Name != "" {
					toolCallsMap[idx].Function.Name += tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					toolCallsMap[idx].Function.Arguments += tc.Function.Arguments
				}
			}
		}
	}

	// Build final non-streaming response
	message := dto.Message{
		Role:    "assistant",
		Content: contentBuilder.String(),
	}
	if reasoningBuilder.Len() > 0 {
		reasoning := reasoningBuilder.String()
		message.ReasoningContent = &reasoning
	}
	if len(toolCallsMap) > 0 {
		// Convert map to sorted slice
		var toolCalls []dto.ToolCallResponse
		for i := 0; i < len(toolCallsMap); i++ {
			if tc, ok := toolCallsMap[i]; ok {
				toolCalls = append(toolCalls, *tc)
			}
		}
		toolCallsJSON, _ := json.Marshal(toolCalls)
		message.ToolCalls = toolCallsJSON
		if finishReason == "" {
			finishReason = "tool_calls"
		}
	}
	if finishReason == "" {
		finishReason = "stop"
	}
	if firstID == "" {
		firstID = "chatcmpl-qoder"
	}
	if model == "" {
		model = info.UpstreamModelName
	}
	if created == 0 {
		created = time.Now().Unix()
	}

	finalResp := dto.OpenAITextResponse{
		Id:      firstID,
		Object:  "chat.completion",
		Created: created,
		Model:   model,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index:        0,
				Message:      message,
				FinishReason: finishReason,
			},
		},
	}
	if usage != nil {
		finalResp.Usage = *usage
	}

	c.JSON(http.StatusOK, finalResp)
	return &finalResp.Usage, nil
}

func (a *Adaptor) GetModelList() []string {
	return QoderAllModels()
}

func (a *Adaptor) GetChannelName() string { return "Qoder" }

// Stubs for unsupported features
func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) { return nil, fmt.Errorf("not supported") }
func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) { return nil, fmt.Errorf("not supported") }
func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) { return nil, fmt.Errorf("not supported") }
func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) { return nil, fmt.Errorf("not supported") }
func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) { return nil, fmt.Errorf("not supported") }
func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) { return nil, fmt.Errorf("not supported") }
func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) { return nil, fmt.Errorf("not supported") }

// --- Helpers ---

func normalizeMessages(oaiReq map[string]interface{}) ([]map[string]interface{}, string) {
	rawMsgs, _ := oaiReq["messages"].([]interface{})
	var systemParts []string
	var out []map[string]interface{}

	for _, raw := range rawMsgs {
		msg, ok := raw.(map[string]interface{})
		if !ok { continue }
		role, _ := msg["role"].(string)
		text := extractText(msg["content"])

		if role == "system" {
			if text != "" { systemParts = append(systemParts, text) }
			continue
		}
		out = append(out, map[string]interface{}{"role": role, "content": text})
	}
	return out, strings.Join(systemParts, "\n\n")
}

func extractText(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		var parts []string
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				if t, ok := m["text"].(string); ok { parts = append(parts, t) }
			}
		}
		return strings.Join(parts, "\n")
	default:
		if content == nil { return "" }
		return fmt.Sprintf("%v", content)
	}
}

func lastUserText(messages []map[string]interface{}) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i]["role"] == "user" {
			if t, ok := messages[i]["content"].(string); ok { return t }
		}
	}
	return ""
}

func stableHash(prefix string, parts ...string) string {
	h := sha256.New()
	h.Write([]byte(prefix))
	for _, p := range parts {
		h.Write([]byte{0})
		h.Write([]byte(p))
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

func truncate(s string, n int) string {
	if len(s) > n { return s[:n] + "..." }
	return s
}

func mapModel(model string) string {
	mapping := map[string]string{
		"qoder-auto": "qmodel_latest", "gpt-4o": "qmodel_latest",
		"claude-sonnet-4": "qmodel_latest", "auto": "qmodel_latest",
		"qmodel_latest": "qmodel_latest", "qmodel": "qmodel",
		"dmodel": "dmodel", "dfmodel": "dfmodel",
		"gm51model": "gm51model", "kmodel": "kmodel", "mmodel": "mmodel",
		"ultimate": "ultimate", "performance": "performance",
		"efficient": "efficient", "lite": "lite",
	}
	if m, ok := mapping[model]; ok { return m }
	return "qmodel_latest"
}
