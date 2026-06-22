package commandcode

import (
	"net/http"

	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Adaptor implements the CommandCode channel.
// CommandCode uses AI SDK v5 NDJSON format, but we use OpenAI-compatible
// passthrough since the upstream also supports SSE via /alpha/generate.
// Ported from 9router executors/commandcode.js.
//
// Auth: Bearer <user_xxx> API key
// Endpoint: https://api.commandcode.ai/alpha/generate
// Features: Streaming with per-request session ID

type Adaptor struct {
	openai.Adaptor
}

func (a *Adaptor) GetChannelName() string {
	return "commandcode"
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) error {
	a.Adaptor.SetupRequestHeader(c, header, info)
	header.Set("x-session-id", uuid.New().String())
	return nil
}

func (a *Adaptor) GetModelList() []string {
	return []string{
		"commandcode-default",
	}
}
