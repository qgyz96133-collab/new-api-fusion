package cursor

import (
	"github.com/QuantumNous/new-api/relay/channel/openai"
)

// Adaptor implements the Cursor IDE channel.
// Cursor uses a proprietary gRPC/protobuf streaming protocol.
// This adaptor provides a simplified OpenAI-compatible proxy layer.
// Ported from 9router executors/cursor.js.
//
// Auth: Bearer <cursor_session_token>
// Endpoint: https://api2.cursor.sh/v1/chat/completions
// Features: GPT-4o, Claude, cursor-small models
//
// Note: Full Cursor gRPC protocol requires protobuf serialization
// (see 9router utils/cursorProtobuf.js). This adaptor handles the
// OpenAI-compatible subset.

type Adaptor struct {
	openai.Adaptor
}

func (a *Adaptor) GetChannelName() string {
	return "cursor"
}

func (a *Adaptor) GetModelList() []string {
	return []string{
		"gpt-4o", "gpt-4o-mini",
		"claude-sonnet-4", "claude-sonnet-4.5", "claude-opus-4",
		"cursor-small",
		"o1", "o1-mini", "o3-mini",
	}
}
