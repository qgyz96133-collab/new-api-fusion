package mimo

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const (
	MimoBootstrapURL = "https://api.xiaomimimo.com/api/free-ai/bootstrap"
	MimoChatURL      = "https://api.xiaomimimo.com/api/free-ai/openai/chat"
	MimoSystemMarker = "You are MiMoCode, an interactive CLI tool that helps users with software engineering tasks."
)

var (
	cachedJWT    string
	jwtExpiresAt time.Time
	jwtMu        sync.Mutex
)

type Adaptor struct{}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return MimoChatURL, nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	jwt, err := getOrBootstrapJWT()
	if err != nil {
		return fmt.Errorf("MiMo bootstrap failed: %w", err)
	}
	req.Set("Authorization", "Bearer "+jwt)
	req.Set("X-Mimo-Source", "mimocode-cli-free")
	req.Set("x-session-affinity", generateSessionID())
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, fmt.Errorf("request is nil")
	}
	// Inject system marker if not present
	hasMarker := false
	for _, msg := range request.Messages {
		if msg.Role == "system" {
			if text, ok := msg.Content.(string); ok && strings.Contains(text, MimoSystemMarker) {
				hasMarker = true
				break
			}
		}
	}
	if !hasMarker {
		markerMsg := dto.Message{Role: "system", Content: MimoSystemMarker}
		request.Messages = append([]dto.Message{markerMsg}, request.Messages...)
	}
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	jwt, err := getOrBootstrapJWT()
	if err != nil {
		return nil, fmt.Errorf("MiMo bootstrap failed: %w", err)
	}

	bodyBytes, _ := io.ReadAll(requestBody)

	// Inject marker if needed
	var reqBody map[string]interface{}
	if json.Unmarshal(bodyBytes, &reqBody) == nil {
		msgs, _ := reqBody["messages"].([]interface{})
		hasMarker := false
		for _, m := range msgs {
			if mm, ok := m.(map[string]interface{}); ok {
				if mm["role"] == "system" {
					if content, ok := mm["content"].(string); ok && strings.Contains(content, MimoSystemMarker) {
						hasMarker = true
						break
					}
				}
			}
		}
		if !hasMarker {
			marker := map[string]interface{}{"role": "system", "content": MimoSystemMarker}
			reqBody["messages"] = append([]interface{}{marker}, msgs...)
			bodyBytes, _ = json.Marshal(reqBody)
		}
	}

	req, _ := http.NewRequestWithContext(c.Request.Context(), "POST", MimoChatURL, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("X-Mimo-Source", "mimocode-cli-free")
	req.Header.Set("x-session-affinity", generateSessionID())

	client := service.GetHttpClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	// On 401/403, re-bootstrap and retry once
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		resp.Body.Close()
		resetJWTCache()
		jwt, _ = getOrBootstrapJWT()
		req2, _ := http.NewRequestWithContext(c.Request.Context(), "POST", MimoChatURL, bytes.NewReader(bodyBytes))
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Authorization", "Bearer "+jwt)
		req2.Header.Set("X-Mimo-Source", "mimocode-cli-free")
		req2.Header.Set("x-session-affinity", generateSessionID())
		return client.Do(req2)
	}

	return resp, nil
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	// MiMo returns OpenAI-compatible responses, delegate to OpenAI handler
	oa := &openai.Adaptor{}
	return oa.DoResponse(c, resp, info)
}

func (a *Adaptor) GetModelList() []string {
	return []string{"mimo-auto", "qmodel_latest", "qmodel", "dmodel", "kmodel", "mmodel"}
}

func (a *Adaptor) GetChannelName() string { return "MiMo Code Free" }

// Stubs
func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) { return nil, fmt.Errorf("not supported") }
func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) { return nil, fmt.Errorf("not supported") }
func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) { return nil, fmt.Errorf("not supported") }
func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) { return nil, fmt.Errorf("not supported") }
func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) { return nil, fmt.Errorf("not supported") }
func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) { return nil, fmt.Errorf("not supported") }
func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) { return nil, fmt.Errorf("not supported") }

// JWT management
func getOrBootstrapJWT() (string, error) {
	jwtMu.Lock()
	defer jwtMu.Unlock()
	if cachedJWT != "" && time.Now().Before(jwtExpiresAt.Add(-5*time.Minute)) {
		return cachedJWT, nil
	}
	return bootstrapJWT()
}

func bootstrapJWT() (string, error) {
	fingerprint := generateFingerprint()
	payload, _ := json.Marshal(map[string]string{"client": fingerprint})
	resp, err := http.Post(MimoBootstrapURL, "application/json", strings.NewReader(string(payload)))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("bootstrap %d: %s", resp.StatusCode, string(body))
	}
	var result struct {
		JWT string `json:"jwt"`
	}
	if json.Unmarshal(body, &result) != nil || result.JWT == "" {
		return "", fmt.Errorf("no JWT in response")
	}
	cachedJWT = result.JWT
	jwtExpiresAt = parseJWTExpiry(result.JWT)
	return cachedJWT, nil
}

func resetJWTCache() {
	jwtMu.Lock()
	defer jwtMu.Unlock()
	cachedJWT = ""
	jwtExpiresAt = time.Time{}
}

func parseJWTExpiry(jwt string) time.Time {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return time.Now().Add(50 * time.Minute)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Now().Add(50 * time.Minute)
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Exp == 0 {
		return time.Now().Add(50 * time.Minute)
	}
	return time.Unix(claims.Exp, 0)
}

func generateFingerprint() string {
	hostname, _ := os.Hostname()
	seed := fmt.Sprintf("%s|linux|amd64|gateway|new-api", hostname)
	h := sha256.Sum256([]byte(seed))
	return fmt.Sprintf("%x", h)
}

func generateSessionID() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	id := "ses_"
	b := make([]byte, 24)
	for i := range b {
		b[i] = chars[time.Now().UnixNano()%int64(len(chars))]
		time.Sleep(time.Nanosecond)
	}
	return id + string(b)
}
