package joycode

import (
	"github.com/QuantumNous/new-api/relay/channel/openai"
)

// Adaptor implements the JoyCode (JD JoyCoder) channel.
// JoyCode provides free AI models via OAuth 2.0.
// Ported from joycode-auth (opencode-joycode-auth plugin).
//
// Auth: OAuth 2.0 Bearer token
// Format: OpenAI-compatible chat completions
// Features: SSE streaming, reasoning_content passthrough

type Adaptor struct {
	openai.Adaptor
}

func (a *Adaptor) GetChannelName() string {
	return "joycode"
}

func (a *Adaptor) GetModelList() []string {
	return []string{
		"coder-model",
		"vision-model",
		"qwen3-coder-plus",
		"qwen3-coder-flash",
	}
}
