package github_copilot

import (
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

// Adaptor implements the GitHub Copilot channel.
// GitHub Copilot uses OpenAI-compatible /chat/completions format
// but requires specific VS Code-style headers.
// Ported from 9router executors/github.js.
//
// Auth: Bearer <copilot_token>
// Endpoint: https://api.githubcopilot.com/chat/completions
// Features: GPT-4o, Claude, GPT-5 models via Copilot subscription

type Adaptor struct {
	openai.Adaptor
}

const (
	vscodeVersion      = "1.101.2"
	copilotChatVersion = "0.28.2025062001"
	apiVersion         = "2025-05-01"
	ghUserAgent        = "GitHubCopilotChat/0.28.2025062001"
)

func (a *Adaptor) GetChannelName() string {
	return "github_copilot"
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) error {
	a.Adaptor.SetupRequestHeader(c, header, info)

	header.Set("copilot-integration-id", "vscode-chat")
	header.Set("editor-version", fmt.Sprintf("vscode/%s", vscodeVersion))
	header.Set("editor-plugin-version", fmt.Sprintf("copilot-chat/%s", copilotChatVersion))
	header.Set("user-agent", ghUserAgent)
	header.Set("openai-intent", "conversation-panel")
	header.Set("x-github-api-version", apiVersion)
	header.Set("X-Initiator", "user")

	return nil
}

func (a *Adaptor) GetModelList() []string {
	return []string{
		"gpt-4o", "gpt-4o-mini", "gpt-4.1", "gpt-4.1-mini",
		"gpt-5", "gpt-5-mini",
		"o1", "o1-mini", "o3-mini", "o4-mini",
		"claude-sonnet-4", "claude-sonnet-4.5",
	}
}
