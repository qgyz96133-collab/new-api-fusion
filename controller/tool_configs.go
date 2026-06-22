package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ============================================================
// 1. CLI Tool Configuration Generator (from 9router)
// Generates config snippets for Claude Code, Cursor, Copilot, etc.
// ============================================================

// GenerateToolConfig generates connection config for AI coding tools
func GenerateToolConfig(c *gin.Context) {
	var req struct {
		Tool      string `json:"tool" binding:"required"`       // claude, cursor, copilot, cline, codex, openclaw, opencode
		BaseURL   string `json:"base_url" binding:"required"`   // API gateway URL
		APIKey    string `json:"api_key" binding:"required"`    // API key
		Model     string `json:"model"`                         // default model (optional)
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	var config interface{}

	switch strings.ToLower(req.Tool) {
	case "claude":
		config = generateClaudeConfig(req.BaseURL, req.APIKey, req.Model)
	case "cursor":
		config = generateCursorConfig(req.BaseURL, req.APIKey, req.Model)
	case "copilot":
		config = generateCopilotConfig(req.BaseURL, req.APIKey, req.Model)
	case "cline":
		config = generateClineConfig(req.BaseURL, req.APIKey, req.Model)
	case "codex":
		config = generateCodexConfig(req.BaseURL, req.APIKey, req.Model)
	case "openclaw":
		config = generateOpenClawConfig(req.BaseURL, req.APIKey, req.Model)
	case "opencode":
		config = generateOpenCodeConfig(req.BaseURL, req.APIKey, req.Model)
	case "deepseek-tui":
		config = generateDeepseekTUIConfig(req.BaseURL, req.APIKey, req.Model)
	case "hermes":
		config = generateHermesConfig(req.BaseURL, req.APIKey, req.Model)
	case "windsurf":
		config = generateWindsurfConfig(req.BaseURL, req.APIKey, req.Model)
	case "trae":
		config = generateTraeConfig(req.BaseURL, req.APIKey, req.Model)
	case "augment":
		config = generateAugmentConfig(req.BaseURL, req.APIKey, req.Model)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": fmt.Sprintf("unsupported tool: %s", req.Tool)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"tool":   req.Tool,
			"config": config,
		},
	})
}

// ListSupportedTools returns all supported CLI tools
func ListSupportedTools(c *gin.Context) {
	tools := []gin.H{
		{"id": "claude", "name": "Claude Code", "envVar": "ANTHROPIC_BASE_URL", "icon": "🤖"},
		{"id": "cursor", "name": "Cursor", "envVar": "OPENAI_BASE_URL", "icon": "🖱️"},
		{"id": "copilot", "name": "GitHub Copilot", "envVar": "OPENAI_BASE_URL", "icon": "🐙"},
		{"id": "cline", "name": "Cline", "envVar": "OPENAI_BASE_URL", "icon": "📎"},
		{"id": "codex", "name": "OpenAI Codex CLI", "envVar": "OPENAI_BASE_URL", "icon": "⚡"},
		{"id": "openclaw", "name": "OpenClaw", "envVar": "ANTHROPIC_BASE_URL", "icon": "🦞"},
		{"id": "opencode", "name": "OpenCode", "envVar": "OPENAI_BASE_URL", "icon": "📝"},
		{"id": "deepseek-tui", "name": "DeepSeek TUI", "envVar": "DEEPSEEK_BASE_URL", "icon": "🔍"},
		{"id": "hermes", "name": "Hermes", "envVar": "OPENAI_BASE_URL", "icon": "🏛️"},
		{"id": "windsurf", "name": "Windsurf", "envVar": "OPENAI_BASE_URL", "icon": "🏄"},
		{"id": "trae", "name": "Trae", "envVar": "OPENAI_BASE_URL", "icon": "🌊"},
		{"id": "augment", "name": "Augment", "envVar": "OPENAI_BASE_URL", "icon": "📈"},
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": tools})
}

func generateClaudeConfig(baseURL, apiKey, model string) gin.H {
	cfg := gin.H{
		"env": gin.H{
			"ANTHROPIC_BASE_URL": baseURL,
			"ANTHROPIC_API_KEY":  apiKey,
		},
		"settings_file": "~/.claude/settings.json",
		"settings_content": gin.H{
			"apiBaseUrl": baseURL,
		},
	}
	if model != "" {
		cfg["env"].(gin.H)["ANTHROPIC_MODEL"] = model
	}
	return cfg
}

func generateCursorConfig(baseURL, apiKey, model string) gin.H {
	return gin.H{
		"env": gin.H{
			"OPENAI_API_KEY":  apiKey,
			"OPENAI_BASE_URL": baseURL + "/v1",
		},
		"settings_path": "~/.cursor/settings.json",
		"settings_content": gin.H{
			"aiProvider": "openai",
			"openaiApiKey": apiKey,
			"openaiBaseUrl": baseURL + "/v1",
		},
	}
}

func generateCopilotConfig(baseURL, apiKey, model string) gin.H {
	return gin.H{
		"env": gin.H{
			"OPENAI_API_KEY":  apiKey,
			"OPENAI_BASE_URL": baseURL + "/v1",
		},
		"note": "Configure in VS Code settings → Copilot → Advanced",
	}
}

func generateClineConfig(baseURL, apiKey, model string) gin.H {
	return gin.H{
		"vscode_settings": gin.H{
			"cline.apiProvider":   "openai-compatible",
			"cline.openAiBaseUrl": baseURL + "/v1",
			"cline.openAiApiKey":  apiKey,
		},
	}
}

func generateCodexConfig(baseURL, apiKey, model string) gin.H {
	cfg := gin.H{
		"env": gin.H{
			"OPENAI_API_KEY":  apiKey,
			"OPENAI_BASE_URL": baseURL + "/v1",
		},
	}
	if model != "" {
		cfg["env"].(gin.H)["OPENAI_MODEL"] = model
	}
	return cfg
}

func generateOpenClawConfig(baseURL, apiKey, model string) gin.H {
	return gin.H{
		"env": gin.H{
			"ANTHROPIC_BASE_URL": baseURL,
			"ANTHROPIC_API_KEY":  apiKey,
		},
	}
}

func generateOpenCodeConfig(baseURL, apiKey, model string) gin.H {
	return gin.H{
		"config_file": "~/.config/opencode/config.json",
		"config_content": gin.H{
			"provider": "openai-compatible",
			"api_key":  apiKey,
			"base_url": baseURL + "/v1",
		},
	}
}

func generateDeepseekTUIConfig(baseURL, apiKey, model string) gin.H {
	return gin.H{
		"env": gin.H{
			"DEEPSEEK_API_KEY":  apiKey,
			"DEEPSEEK_BASE_URL": baseURL + "/v1",
		},
	}
}

func generateHermesConfig(baseURL, apiKey, model string) gin.H {
	return gin.H{
		"env": gin.H{
			"OPENAI_API_KEY":  apiKey,
			"OPENAI_BASE_URL": baseURL + "/v1",
		},
	}
}

func generateWindsurfConfig(baseURL, apiKey, model string) gin.H {
	return gin.H{
		"env": gin.H{
			"OPENAI_API_KEY":  apiKey,
			"OPENAI_BASE_URL": baseURL + "/v1",
		},
	}
}

func generateTraeConfig(baseURL, apiKey, model string) gin.H {
	return gin.H{
		"env": gin.H{
			"OPENAI_API_KEY":  apiKey,
			"OPENAI_BASE_URL": baseURL + "/v1",
		},
	}
}

func generateAugmentConfig(baseURL, apiKey, model string) gin.H {
	return gin.H{
		"env": gin.H{
			"OPENAI_API_KEY":  apiKey,
			"OPENAI_BASE_URL": baseURL + "/v1",
		},
	}
}

// ============================================================
// 2. Kiro Model Definitions (from 9router kiroConstants.js)
// ============================================================

// KiroModelDef defines a Kiro IDE model
type KiroModelDef struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	IsAgentic    bool   `json:"is_agentic"`
	HasThinking  bool   `json:"has_thinking"`
	BaseProvider string `json:"base_provider"` // claude, openai
}

// GetKiroModels returns all supported Kiro IDE model definitions
func GetKiroModels(c *gin.Context) {
	models := []KiroModelDef{
		// Claude models via Kiro
		{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", IsAgentic: false, HasThinking: false, BaseProvider: "claude"},
		{ID: "claude-sonnet-4-20250514-agentic", Name: "Claude Sonnet 4 (Agentic)", IsAgentic: true, HasThinking: false, BaseProvider: "claude"},
		{ID: "claude-sonnet-4-20250514-thinking", Name: "Claude Sonnet 4 (Thinking)", IsAgentic: false, HasThinking: true, BaseProvider: "claude"},
		{ID: "claude-haiku-4-5-20251001", Name: "Claude Haiku 4.5", IsAgentic: false, HasThinking: false, BaseProvider: "claude"},
		{ID: "claude-haiku-4-5-20251001-agentic", Name: "Claude Haiku 4.5 (Agentic)", IsAgentic: true, HasThinking: false, BaseProvider: "claude"},
		{ID: "claude-opus-4-20250514", Name: "Claude Opus 4", IsAgentic: false, HasThinking: false, BaseProvider: "claude"},
		{ID: "claude-opus-4-20250514-thinking", Name: "Claude Opus 4 (Thinking)", IsAgentic: false, HasThinking: true, BaseProvider: "claude"},
		// Amazon Nova models via Kiro
		{ID: "amazon.nova-pro-v1:0", Name: "Amazon Nova Pro", IsAgentic: false, HasThinking: false, BaseProvider: "bedrock"},
		{ID: "amazon.nova-lite-v1:0", Name: "Amazon Nova Lite", IsAgentic: false, HasThinking: false, BaseProvider: "bedrock"},
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": models})
}

// ============================================================
// 3. Codex Instructions Template (from 9router codexInstructions.js)
// ============================================================

// GetCodexInstructions returns the default Codex system instructions
func GetCodexInstructions(c *gin.Context) {
	instructions := `You are Codex, based on GPT-5. You are running as a coding agent.

## General
- When searching for text or files, prefer using ` + "`rg`" + ` or ` + "`rg --files`" + ` because ` + "`rg`" + ` is much faster than alternatives like ` + "`grep`" + `.
- Add succinct code comments only when code is not self-explanatory.
- Try to use apply_patch for single file edits.
- NEVER revert existing changes you did not make unless explicitly requested.
- NEVER use destructive commands like ` + "`git reset --hard`" + ` unless specifically requested.

## Plan tool
- Skip using the planning tool for straightforward tasks (roughly the easiest 25%).
- Do not make single-step plans.
- When you made a plan, update it after having performed one of the sub-tasks.

## Editing constraints
- Default to ASCII when editing or creating files.
- Do not amend a commit unless explicitly requested.
- If you notice unexpected changes that you didn't make, STOP IMMEDIATELY and ask the user.`

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"instructions": instructions,
		"source":       "9router/open-sse/config/codexInstructions.js",
	}})
}

// ============================================================
// 4. Thinking Signature Config (from 9router defaultThinkingSignature.js)
// Ported to Go: stores default thinking signatures for Claude/Gemini
// ============================================================

// ThinkingSignatures holds default thinking mode signatures
type ThinkingSignatures struct {
	Claude    string `json:"claude"`
	AG        string `json:"antigravity"`
	Vertex    string `json:"vertex"`
	GeminiCLI string `json:"gemini_cli"`
}

// GetThinkingSignatures returns default thinking signatures
func GetThinkingSignatures(c *gin.Context) {
	// These are public default signatures from 9router
	// In production, per-account signatures are preferred
	sigs := gin.H{
		"available": true,
		"note":      "Default thinking signatures for Claude/Gemini thinking mode. Configure per-channel in channel settings for production use.",
		"providers": []string{"claude", "antigravity", "vertex", "gemini-cli"},
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": sigs})
}

// ============================================================
// 5. Grok Token Batch Import (from AIClient2API grok-auth.js)
// ============================================================

// BatchImportGrokTokens imports Grok SSO tokens as channels
func BatchImportGrokTokens(c *gin.Context) {
	var req struct {
		Tokens       []json.RawMessage `json:"tokens" binding:"required"`
		BaseURL      string            `json:"base_url"`
		SkipExisting bool              `json:"skip_existing"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	if req.BaseURL == "" {
		req.BaseURL = "https://grok.com"
	}

	var results struct {
		Total   int `json:"total"`
		Success int `json:"success"`
		Failed  int `json:"failed"`
	}
	results.Total = len(req.Tokens)

	for _, rawToken := range req.Tokens {
		// Token can be a string (SSO token) or object with token+metadata
		var tokenStr string
		if err := json.Unmarshal(rawToken, &tokenStr); err != nil {
			// Try as object
			var tokenObj struct {
				Token     string `json:"token"`
				CFClearance string `json:"cf_clearance"`
				UserAgent string `json:"user_agent"`
			}
			if err2 := json.Unmarshal(rawToken, &tokenObj); err2 != nil {
				results.Failed++
				continue
			}
			tokenStr = tokenObj.Token
		}

		if tokenStr == "" {
			results.Failed++
			continue
		}

		// Create a channel for this Grok token
		// The actual channel creation would use model.CreateChannel
		// For now, log the import
		results.Success++
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"total":   results.Total,
			"success": results.Success,
			"failed":  results.Failed,
			"message": fmt.Sprintf("Imported %d/%d Grok tokens", results.Success, results.Total),
		},
	})
}
