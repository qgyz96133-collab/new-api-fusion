package kiro

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type Adaptor struct {
	ProfileArn string
	Region     string
	AuthMethod string
}

var KiroModels = []string{
	"claude-sonnet-4-5",
	"claude-opus-4-5",
	"claude-opus-4-6",
	"claude-opus-4-7",
	"claude-opus-4-8",
	"claude-haiku-4-5",
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	a.Region = "us-east-1"
	a.AuthMethod = "social"
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseURL := info.ChannelBaseUrl
	if baseURL == "" {
		baseURL = fmt.Sprintf("https://q.%s.amazonaws.com", a.Region)
	}
	return fmt.Sprintf("%s/generateAssistantResponse", baseURL), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", fmt.Sprintf("Bearer %s", info.ApiKey))
	req.Set("Content-Type", "application/json")
	req.Set("Accept", "application/json")
	req.Set("x-amzn-codewhisperer-optout", "true")
	req.Set("x-amzn-kiro-agent-mode", "vibe")
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if resp == nil {
		return nil, types.NewError(errors.New("response is nil"), types.ErrorCodeModelNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		logger.LogError(c, fmt.Sprintf("[Kiro] API error (status %d): %s", resp.StatusCode, string(bodyBytes)))
		return nil, types.NewError(fmt.Errorf("Kiro API error: %s", string(bodyBytes)), types.ErrorCodeModelNotFound)
	}
	bodyBytes, err2 := io.ReadAll(resp.Body)
	if err2 != nil {
		return nil, types.NewError(err2, types.ErrorCodeModelNotFound)
	}
	var result map[string]interface{}
	if err2 := json.Unmarshal(bodyBytes, &result); err2 != nil {
		return nil, types.NewError(fmt.Errorf("failed to parse response: %w", err2), types.ErrorCodeModelNotFound)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Write(bodyBytes)
	return nil, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("not supported")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("not supported")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("not supported")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("not supported")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("not supported")
}

func (a *Adaptor) GetModelList() []string {
	return KiroModels
}

func (a *Adaptor) GetChannelName() string {
	return "kiro"
}
