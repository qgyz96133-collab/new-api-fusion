package service

import (
	"encoding/json"
	"strings"
	"sync"
)

// ModelCapabilities describes what a model can do.
// Ported from 9router providers/capabilities.js.
type ModelCapabilities struct {
	Vision        bool   `json:"vision"`         // can read images
	PDF           bool   `json:"pdf"`            // can read PDFs
	AudioInput    bool   `json:"audio_input"`    // can read audio
	VideoInput    bool   `json:"video_input"`    // can read video
	ImageOutput   bool   `json:"image_output"`   // can generate images
	AudioOutput   bool   `json:"audio_output"`   // can generate audio
	Search        bool   `json:"search"`         // built-in web search
	Tools         bool   `json:"tools"`          // function/tool calling
	Reasoning     bool   `json:"reasoning"`      // thinking/reasoning
	ContextWindow int    `json:"context_window"` // context window in tokens
	MaxOutput     int    `json:"max_output"`     // max output tokens
}

var (
	capabilitiesDB   = map[string]ModelCapabilities{}
	capabilitiesDBMu sync.RWMutex
)

// DefaultCapabilities returns safe defaults
func DefaultCapabilities() ModelCapabilities {
	return ModelCapabilities{
		Tools:         true,
		ContextWindow: 200000,
		MaxOutput:     64000,
	}
}

// RegisterModelCapabilities registers capabilities for a specific model
func RegisterModelCapabilities(modelID string, caps ModelCapabilities) {
	capabilitiesDBMu.Lock()
	defer capabilitiesDBMu.Unlock()
	capabilitiesDB[modelID] = caps
}

// GetModelCapabilities returns capabilities for a model, using pattern matching
func GetModelCapabilities(modelID string) ModelCapabilities {
	capabilitiesDBMu.RLock()
	defer capabilitiesDBMu.RUnlock()

	// Exact match first
	if caps, ok := capabilitiesDB[modelID]; ok {
		return caps
	}

	// Pattern matching (specific before generic)
	lower := strings.ToLower(modelID)

	// Claude models
	if strings.Contains(lower, "claude") {
		caps := DefaultCapabilities()
		caps.Vision = true
		caps.Reasoning = strings.Contains(lower, "opus") || strings.Contains(lower, "sonnet-4")
		caps.Search = strings.Contains(lower, "4.6") || strings.Contains(lower, "4.7")
		if strings.Contains(lower, "opus-4.6") || strings.Contains(lower, "opus-4.7") {
			caps.ContextWindow = 1000000
			caps.MaxOutput = 128000
		} else {
			caps.ContextWindow = 200000
			caps.MaxOutput = 64000
		}
		return caps
	}

	// GPT models
	if strings.Contains(lower, "gpt") {
		caps := DefaultCapabilities()
		caps.Vision = true
		caps.Search = true
		caps.ImageOutput = strings.Contains(lower, "gpt-4o") || strings.Contains(lower, "gpt-5")
		if strings.Contains(lower, "gpt-5") {
			caps.Reasoning = true
			caps.ContextWindow = 400000
		}
		return caps
	}

	// Gemini models
	if strings.Contains(lower, "gemini") {
		caps := DefaultCapabilities()
		caps.Vision = true
		caps.AudioInput = true
		caps.VideoInput = true
		caps.PDF = true
		caps.Reasoning = true
		caps.Search = true
		if strings.Contains(lower, "2.5-pro") || strings.Contains(lower, "3") {
			caps.ContextWindow = 1000000
			caps.MaxOutput = 65536
		}
		return caps
	}

	// DeepSeek models
	if strings.Contains(lower, "deepseek") {
		caps := DefaultCapabilities()
		caps.Reasoning = strings.Contains(lower, "r1") || strings.Contains(lower, "reasoner")
		caps.ContextWindow = 128000
		return caps
	}

	// O-series reasoning models
	if strings.HasPrefix(lower, "o1") || strings.HasPrefix(lower, "o3") || strings.HasPrefix(lower, "o4") {
		caps := DefaultCapabilities()
		caps.Vision = true
		caps.Reasoning = true
		caps.Search = true
		caps.ContextWindow = 200000
		return caps
	}

	return DefaultCapabilities()
}

// ReorderModelsByCapabilities sorts models by capability fit.
// Tier 0: satisfies all hard + soft requirements.
// Tier 1: satisfies hard requirements only.
// Tier 2: doesn't satisfy hard requirements.
// Ported from 9router services/combo.js reorderByCapabilities().
func ReorderModelsByCapabilities(models []string, required map[string]bool) []string {
	if len(required) == 0 || len(models) <= 1 {
		return models
	}

	hardCaps := map[string]bool{
		"vision":       true,
		"pdf":          true,
		"audio_input":  true,
		"video_input":  true,
	}

	type ranked struct {
		model string
		tier  int
		index int
	}

	ranked_models := make([]ranked, len(models))
	for i, m := range models {
		caps := GetModelCapabilities(m)
		tier := 2

		// Check hard requirements
		allHard := true
		for cap, needed := range required {
			if !needed {
				continue
			}
			if !hardCaps[cap] {
				continue
			}
			if !hasCapability(caps, cap) {
				allHard = false
				break
			}
		}

		if allHard {
			tier = 1
			// Check soft requirements
			allSoft := true
			for cap, needed := range required {
				if !needed {
					continue
				}
				if hardCaps[cap] {
					continue
				}
				if !hasCapability(caps, cap) {
					allSoft = false
					break
				}
			}
			if allSoft {
				tier = 0
			}
		}

		ranked_models[i] = ranked{model: m, tier: tier, index: i}
	}

	// Stable sort by tier
	for i := 0; i < len(ranked_models)-1; i++ {
		for j := i + 1; j < len(ranked_models); j++ {
			if ranked_models[j].tier < ranked_models[i].tier {
				ranked_models[i], ranked_models[j] = ranked_models[j], ranked_models[i]
			}
		}
	}

	result := make([]string, len(models))
	for i, r := range ranked_models {
		result[i] = r.model
	}
	return result
}

func hasCapability(caps ModelCapabilities, cap string) bool {
	switch cap {
	case "vision":
		return caps.Vision
	case "pdf":
		return caps.PDF
	case "audio_input":
		return caps.AudioInput
	case "video_input":
		return caps.VideoInput
	case "image_output":
		return caps.ImageOutput
	case "audio_output":
		return caps.AudioOutput
	case "search":
		return caps.Search
	case "tools":
		return caps.Tools
	case "reasoning":
		return caps.Reasoning
	default:
		return false
	}
}

// InitDefaultCapabilities registers well-known model capabilities
func InitDefaultCapabilities() {
	// Claude
	RegisterModelCapabilities("claude-opus-4.6", ModelCapabilities{Vision: true, Reasoning: true, Search: true, Tools: true, ContextWindow: 1000000, MaxOutput: 128000})
	RegisterModelCapabilities("claude-opus-4.7", ModelCapabilities{Vision: true, Reasoning: true, Search: true, Tools: true, ContextWindow: 1000000, MaxOutput: 128000})
	RegisterModelCapabilities("claude-sonnet-4.6", ModelCapabilities{Vision: true, Reasoning: true, Search: true, Tools: true, ContextWindow: 1000000, MaxOutput: 64000})
	RegisterModelCapabilities("claude-sonnet-4.5", ModelCapabilities{Vision: true, Reasoning: true, Tools: true, ContextWindow: 200000, MaxOutput: 64000})

	// GPT
	RegisterModelCapabilities("gpt-5", ModelCapabilities{Vision: true, Reasoning: true, Search: true, Tools: true, ImageOutput: true, ContextWindow: 400000, MaxOutput: 64000})
	RegisterModelCapabilities("gpt-4o", ModelCapabilities{Vision: true, Search: true, Tools: true, ImageOutput: true, ContextWindow: 128000, MaxOutput: 16384})

	// Gemini
	RegisterModelCapabilities("gemini-2.5-pro", ModelCapabilities{Vision: true, PDF: true, AudioInput: true, VideoInput: true, Reasoning: true, Search: true, Tools: true, ContextWindow: 1000000, MaxOutput: 65536})
	RegisterModelCapabilities("gemini-3-flash", ModelCapabilities{Vision: true, PDF: true, AudioInput: true, VideoInput: true, Reasoning: true, Search: true, Tools: true, ContextWindow: 1000000, MaxOutput: 65536})

	// Reasoning
	RegisterModelCapabilities("o1", ModelCapabilities{Vision: true, Reasoning: true, Search: true, Tools: true, ContextWindow: 200000, MaxOutput: 64000})
	RegisterModelCapabilities("o3-mini", ModelCapabilities{Reasoning: true, Tools: true, ContextWindow: 200000, MaxOutput: 64000})
}

// LoadCapabilitiesFromJSON loads capabilities from JSON config
// Format: {"model-id": {"vision": true, "reasoning": true, ...}}
func LoadCapabilitiesFromJSON(data string) error {
	if strings.TrimSpace(data) == "" {
		return nil
	}
	var caps map[string]ModelCapabilities
	if err := json.Unmarshal([]byte(data), &caps); err != nil {
		return err
	}
	capabilitiesDBMu.Lock()
	defer capabilitiesDBMu.Unlock()
	for k, v := range caps {
		capabilitiesDB[k] = v
	}
	return nil
}
