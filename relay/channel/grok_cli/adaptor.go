package grok_cli

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type Adaptor struct{}

var GrokCliModels = []string{
	"grok-3",
	"grok-3-mini",
	"grok-4",
	"grok-4.1-thinking",
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseURL := info.ChannelBaseUrl
	if baseURL == "" {
		baseURL = "https://api.x.ai"
	}
	// Grok CLI uses xAI Responses API
	return baseURL + "/v1/responses", nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	key := strings.TrimSpace(info.ApiKey)
	req.Set("Authorization", "Bearer "+key)
	req.Set("Content-Type", "application/json")
	if info.IsStream {
		req.Set("Accept", "text/event-stream")
	} else {
		req.Set("Accept", "application/json")
	}
	return nil
}

// ConvertOpenAIRequest converts OpenAI chat completions to xAI Responses format
func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}

	responsesReq := dto.OpenAIResponsesRequest{
		Model:  request.Model,
		Stream: request.Stream,
	}

	if request.MaxTokens != nil {
		maxOutput := uint(*request.MaxTokens)
		responsesReq.MaxOutputTokens = &maxOutput
	}
	if request.MaxCompletionTokens != nil {
		responsesReq.MaxOutputTokens = request.MaxCompletionTokens
	}
	if request.Temperature != nil {
		responsesReq.Temperature = request.Temperature
	}
	if request.TopP != nil {
		responsesReq.TopP = request.TopP
	}

	// Convert messages to Responses input format
	var inputItems []interface{}
	var instructions string

	for _, msg := range request.Messages {
		switch msg.Role {
		case "system":
			if instructions != "" {
				instructions += "\n"
			}
			instructions += getContentText(msg.Content)
		case "tool":
			inputItems = append(inputItems, map[string]interface{}{
				"type":    "function_call_output",
				"call_id": msg.ToolCallId,
				"output":  getContentText(msg.Content),
			})
		case "assistant":
			toolCalls := msg.ParseToolCalls()
			for _, tc := range toolCalls {
				inputItems = append(inputItems, map[string]interface{}{
					"type":      "function_call",
					"call_id":   tc.ID,
					"name":      tc.Function.Name,
					"arguments": tc.Function.Arguments,
				})
			}
			text := getContentText(msg.Content)
			if text != "" {
				inputItems = append(inputItems, map[string]interface{}{
					"type": "message",
					"role": "assistant",
					"content": []map[string]interface{}{
						{"type": "output_text", "text": text},
					},
				})
			}
		default: // user
			text := getContentText(msg.Content)
			if text != "" {
				inputItems = append(inputItems, map[string]interface{}{
					"type": "message",
					"role": "user",
					"content": []map[string]interface{}{
						{"type": "input_text", "text": text},
					},
				})
			}
		}
	}

	if instructions != "" {
		responsesReq.Instructions = []byte(`"` + instructions + `"`)
	}

	// Set input
	if len(inputItems) == 1 {
		inputJSON, _ := json.Marshal(inputItems[0])
		responsesReq.Input = inputJSON
	} else if len(inputItems) > 1 {
		inputJSON, _ := json.Marshal(inputItems)
		responsesReq.Input = inputJSON
	}

	// Convert tools to Responses format
	if request.Tools != nil {
		var toolsList []map[string]interface{}
		for _, tool := range request.Tools {
			if tool.Type == "function" {
				toolsList = append(toolsList, map[string]interface{}{
					"type":        "function",
					"name":        tool.Function.Name,
					"description": tool.Function.Description,
					"parameters":  tool.Function.Parameters,
				})
			}
		}
		if len(toolsList) > 0 {
			toolsJSON, _ := json.Marshal(toolsList)
			responsesReq.Tools = toolsJSON
		}
	}

	return responsesReq, nil
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, req *dto.ClaudeRequest) (any, error) {
	adaptor := openai.Adaptor{}
	oaiReq, err := adaptor.ConvertClaudeRequest(c, info, req)
	if err != nil {
		return nil, err
	}
	return a.ConvertOpenAIRequest(c, info, oaiReq.(*dto.GeneralOpenAIRequest))
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("Gemini format not supported via Grok CLI channel")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

// DoResponse delegates to OpenAI Responses handlers for proper xAI SSE/JSON parsing
func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info.RelayMode == relayconstant.RelayModeResponses || info.RelayMode == relayconstant.RelayModeResponsesCompact {
		if info.IsStream {
			return openai.OaiResponsesStreamHandler(c, info, resp)
		}
		return openai.OaiResponsesHandler(c, info, resp)
	}
	// Fallback for chat completions relay mode
	if info.IsStream {
		return openai.OaiStreamHandler(c, info, resp)
	}
	return openai.OpenaiHandler(c, info, resp)
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("rerank not supported by Grok CLI")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("embedding not supported by Grok CLI")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("audio not supported by Grok CLI")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) GetModelList() []string {
	return GrokCliModels
}

func (a *Adaptor) GetChannelName() string {
	return "grok-cli"
}

// getContentText extracts text content from various content formats
func getContentText(content interface{}) string {
	switch c := content.(type) {
	case string:
		return c
	case []interface{}:
		var text string
		for _, part := range c {
			if partMap, ok := part.(map[string]interface{}); ok {
				if t, ok := partMap["text"].(string); ok {
					text += t
				}
			}
		}
		return text
	default:
		return ""
	}
}
