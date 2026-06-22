package xiaomi_tokenplan

import (
	"github.com/QuantumNous/new-api/relay/channel/openai"
)

// Adaptor implements the Xiaomi TokenPlan channel.
// Xiaomi TokenPlan provides OpenAI-compatible /chat/completions endpoint
// with region-specific base URLs.
// Ported from 9router executors/xiaomi-tokenplan.js.
//
// Auth: Bearer <token>
// Endpoint: {base_url}/chat/completions

type Adaptor struct {
	openai.Adaptor
}

func (a *Adaptor) GetChannelName() string {
	return "xiaomi_tokenplan"
}

func (a *Adaptor) GetModelList() []string {
	return []string{
		"claude-sonnet-4", "claude-sonnet-4.5",
		"claude-opus-4", "claude-haiku-4.5",
		"mimo-v2.5-pro",
	}
}
