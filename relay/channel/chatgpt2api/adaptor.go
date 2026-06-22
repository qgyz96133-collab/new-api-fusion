package chatgpt2api

import (
	"github.com/QuantumNous/new-api/relay/channel/openai"
)

// Adaptor implements the ChatGPT2API channel.
// ChatGPT2API is a Python sidecar service that reverse-engineers ChatGPT's
// web interface and exposes an OpenAI-compatible API.
//
// Architecture:
//   fusion → ChatGPT2API container → ChatGPT web backend
//
// ChatGPT2API handles:
//   - Auto-registration of free ChatGPT accounts
//   - Account pool management with token refresh
//   - Anti-detection (Turnstile, PoW, sentinel tokens)
//   - Cloudflare bypass (WARP + FlareSolverr)
//   - /v1/chat/completions (GPT-4o, GPT-5, etc.)
//   - /v1/images/generations (DALL-E / gpt-image-2)
//   - /v1/images/edits (image editing)
//   - /v1/responses (Responses API)
//   - /v1/messages (Anthropic format)
//
// Configuration in fusion:
//   - Base URL: http://chatgpt2api:80 (or wherever the sidecar runs)
//   - API Key: The auth-key from chatgpt2api's config.json
//   - Models: auto-detected from /v1/models
//
// Since ChatGPT2API is fully OpenAI-compatible, this adaptor simply
// embeds openai.Adaptor with no modifications needed.

type Adaptor struct {
	openai.Adaptor
}

func (a *Adaptor) GetChannelName() string {
	return "chatgpt2api"
}

func (a *Adaptor) GetModelList() []string {
	return []string{
		"gpt-4o", "gpt-4o-mini", "gpt-5", "gpt-5-mini",
		"o1", "o1-mini", "o3-mini", "o4-mini",
		"gpt-image-2", "codex-gpt-image-2",
		"auto",
	}
}
