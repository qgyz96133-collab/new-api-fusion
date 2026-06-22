package model

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

// FallbackChainConfig stores cross-provider fallback chain definitions
// Ported from AIClient2API's providerFallbackChain + modelFallbackMapping
type FallbackChainConfig struct {
	// Provider Fallback Chain: when provider A fails, try provider B
	// Format: {"gemini-cli": ["antigravity", "openai-custom"], "kiro": ["claude-custom"]}
	ProviderChains map[string][]string `json:"provider_chains"`

	// Model Fallback Mapping: specific model → different provider's model
	// Format: {"gemini-claude-opus-4-5": {"channel_type": 14, "model": "claude-opus-4-5"}}
	ModelMappings map[string]ModelMappingTarget `json:"model_mappings"`
}

// ModelMappingTarget defines where a model should be routed
type ModelMappingTarget struct {
	ChannelType int    `json:"channel_type"` // target channel type
	Model       string `json:"model"`        // target model name
	Enabled     bool   `json:"enabled"`
}

var (
	globalFallbackConfig   FallbackChainConfig
	globalFallbackConfigMu sync.RWMutex
)

func init() {
	globalFallbackConfig = FallbackChainConfig{
		ProviderChains: make(map[string][]string),
		ModelMappings:  make(map[string]ModelMappingTarget),
	}
}

// LoadFallbackConfig loads fallback config from DB
func LoadFallbackConfig() error {
	data, err := GetOption("fallback_chain_config")
	if err != nil || data == "" {
		return nil // no config, use empty defaults
	}

	var cfg FallbackChainConfig
	if err := json.Unmarshal([]byte(data), &cfg); err != nil {
		return err
	}

	globalFallbackConfigMu.Lock()
	defer globalFallbackConfigMu.Unlock()
	globalFallbackConfig = cfg
	return nil
}

// SaveFallbackConfig persists config to DB
func SaveFallbackConfig(cfg FallbackChainConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	globalFallbackConfigMu.Lock()
	globalFallbackConfig = cfg
	globalFallbackConfigMu.Unlock()

	return UpdateOption("fallback_chain_config", string(data))
}

// GetFallbackConfig returns the current config
func GetFallbackConfig() FallbackChainConfig {
	globalFallbackConfigMu.RLock()
	defer globalFallbackConfigMu.RUnlock()
	return globalFallbackConfig
}

// GetFallbackChannels returns the fallback channel types for a given channel type name
func GetFallbackChannels(channelTypeName string) []string {
	globalFallbackConfigMu.RLock()
	defer globalFallbackConfigMu.RUnlock()
	return globalFallbackConfig.ProviderChains[channelTypeName]
}

// GetModelMapping returns the target mapping for a model name
func GetModelMapping(modelName string) (*ModelMappingTarget, bool) {
	globalFallbackConfigMu.RLock()
	defer globalFallbackConfigMu.RUnlock()
	target, ok := globalFallbackConfig.ModelMappings[modelName]
	if !ok || !target.Enabled {
		return nil, false
	}
	return &target, true
}

// GetOption retrieves a single option value by key
func GetOption(key string) (string, error) {
	common.OptionMapRWMutex.RLock()
	val, ok := common.OptionMap[key]
	common.OptionMapRWMutex.RUnlock()
	if !ok {
		return "", nil
	}
	return val, nil
}

// ChannelTypeName returns a name for channel type (for fallback chain lookup)
func ChannelTypeName(channelType int) string {
	names := map[int]string{
		1: "openai-custom", 14: "claude-custom", 24: "gemini-cli-oauth",
		48: "grok-cli-oauth", 57: "openai-codex-oauth",
	}
	if name, ok := names[channelType]; ok {
		return name
	}
	return ""
}

// ParseChannelTypeName extracts channel type name from comma-separated list
func ParseChannelTypeNames(names string) []int {
	nameToType := map[string]int{
		"openai-custom": 1, "claude-custom": 14, "gemini-cli-oauth": 24,
		"gemini-antigravity": 24, "grok-cli-oauth": 48, "openai-codex-oauth": 57,
		"claude-kiro-oauth": 14, "openai-qwen-oauth": 1, "forward-api": 1,
	}
	parts := strings.Split(names, ",")
	var result []int
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if t, ok := nameToType[p]; ok {
			result = append(result, t)
		}
	}
	return result
}
