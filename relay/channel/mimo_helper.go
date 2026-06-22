package channel

import (
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

	"github.com/QuantumNous/new-api/common"
)

// MiMo Free Tier JWT Management
// Ported from 9router/open-sse/executors/mimo-free.js
//
// When using MiMo as an OpenAI-compatible channel:
// - Base URL: https://api.xiaomimimo.com/api/free-ai/openai/chat
// - Auth: JWT from bootstrap (auto-managed by this helper)
// - System message must contain MimoSystemMarker

const (
	MimoBootstrapURL  = "https://api.xiaomimimo.com/api/free-ai/bootstrap"
	MimoChatURL       = "https://api.xiaomimimo.com/api/free-ai/openai/chat"
	MimoSystemMarker  = "You are MiMoCode, an interactive CLI tool that helps users with software engineering tasks."
	mimoJwtFallback   = 3000 * time.Second
	mimoJwtBuffer     = 5 * time.Minute
)

var (
	mimoJwt       string
	mimoJwtExpiry time.Time
	mimoMu        sync.Mutex
)

// MimoGetJWT returns a valid JWT, bootstrapping if needed
func MimoGetJWT() (string, error) {
	mimoMu.Lock()
	defer mimoMu.Unlock()

	if mimoJwt != "" && time.Now().Before(mimoJwtExpiry.Add(-mimoJwtBuffer)) {
		return mimoJwt, nil
	}

	return mimoBootstrap()
}

func mimoBootstrap() (string, error) {
	fp := mimoFingerprint()
	payload, _ := json.Marshal(map[string]string{"client": fp})

	resp, err := http.Post(MimoBootstrapURL, "application/json", strings.NewReader(string(payload)))
	if err != nil {
		return "", fmt.Errorf("bootstrap failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("bootstrap %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		JWT string `json:"jwt"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.JWT == "" {
		return "", fmt.Errorf("no JWT in response")
	}

	mimoJwt = result.JWT
	mimoJwtExpiry = mimoParseExp(result.JWT)
	common.SysLog("[MiMo] JWT bootstrapped")
	return mimoJwt, nil
}

// MimoResetJWT invalidates the cached JWT (call on 401/403)
func MimoResetJWT() {
	mimoMu.Lock()
	defer mimoMu.Unlock()
	mimoJwt = ""
	mimoJwtExpiry = time.Time{}
}

func mimoParseExp(jwt string) time.Time {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return time.Now().Add(mimoJwtFallback)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Now().Add(mimoJwtFallback)
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Exp == 0 {
		return time.Now().Add(mimoJwtFallback)
	}
	return time.Unix(claims.Exp, 0)
}

func mimoFingerprint() string {
	hostname, _ := os.Hostname()
	seed := fmt.Sprintf("%s|linux|amd64|gateway|%s", hostname, "new-api")
	h := sha256.Sum256([]byte(seed))
	return fmt.Sprintf("%x", h)
}

// MimoDefaultModels returns the default MiMo model list
func MimoDefaultModels() []string {
	return []string{
		"mimo-auto",
	}
}

// KiroDefaultModels returns the default Kiro model list
func KiroDefaultModels() []string {
	return []string{
		"claude-haiku-4-5", "claude-sonnet-4-5", "claude-sonnet-4-6",
		"claude-opus-4-5", "claude-opus-4-6", "claude-opus-4-7", "claude-opus-4-8",
	}
}

// QoderDefaultModels returns the default Qoder model list
func QoderDefaultModels() []string {
	return []string{"qoder-auto", "gpt-4o", "claude-sonnet-4"}
}
