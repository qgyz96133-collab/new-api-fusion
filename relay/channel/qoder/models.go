package qoder

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// ModelConfig holds per-model configuration fetched from Qoder's API
type ModelConfig struct {
	Key             string `json:"key"`
	IsReasoning     bool   `json:"is_reasoning"`
	MaxOutputTokens int    `json:"max_output_tokens"`
	Source          string `json:"source"`
}

var (
	modelConfigCache   = make(map[string]map[string]*ModelConfig) // credsHash -> modelKey -> config
	modelConfigCacheMu sync.RWMutex
	modelConfigExpiry  = make(map[string]time.Time)
	cacheTTL           = 1 * time.Hour
)

// FetchModelConfigs fetches live model configs from Qoder's /algo/api/v2/model/list
func FetchModelConfigs(creds *Creds) (map[string]*ModelConfig, error) {
	if creds == nil || creds.UserID == "" || creds.AuthToken == "" {
		return nil, fmt.Errorf("invalid credentials")
	}

	cacheKey := credsCacheKey(creds)

	// Check cache
	modelConfigCacheMu.RLock()
	if configs, ok := modelConfigCache[cacheKey]; ok {
		if expiry, ok := modelConfigExpiry[cacheKey]; ok && time.Now().Before(expiry) {
			modelConfigCacheMu.RUnlock()
			return configs, nil
		}
	}
	modelConfigCacheMu.RUnlock()

	// Fetch from API
	headers, err := BuildCosyHeaders(nil, QoderModelListURL, creds)
	if err != nil {
		return nil, fmt.Errorf("COSY signing failed: %w", err)
	}
	headers["Accept"] = "application/json"
	headers["Accept-Encoding"] = "identity"

	req, _ := http.NewRequest("GET", QoderModelListURL, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("model list request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("model list returned %d: %s", resp.StatusCode, string(body[:min(200, len(body))]))
	}

	// Parse response - Qoder returns an array of model configs
	var rawModels []json.RawMessage
	if err := json.Unmarshal(body, &rawModels); err != nil {
		// Try wrapper format
		var wrapper struct {
			Data   []json.RawMessage `json:"data"`
			Models []json.RawMessage `json:"models"`
		}
		if err2 := json.Unmarshal(body, &wrapper); err2 != nil {
			return nil, fmt.Errorf("failed to parse model list: %w", err)
		}
		if len(wrapper.Data) > 0 {
			rawModels = wrapper.Data
		} else {
			rawModels = wrapper.Models
		}
	}

	configs := make(map[string]*ModelConfig)
	for _, raw := range rawModels {
		var mc ModelConfig
		if err := json.Unmarshal(raw, &mc); err != nil {
			continue
		}
		if mc.Key != "" {
			configs[mc.Key] = &mc
		}
	}

	// Cache result
	modelConfigCacheMu.Lock()
	modelConfigCache[cacheKey] = configs
	modelConfigExpiry[cacheKey] = time.Now().Add(cacheTTL)
	modelConfigCacheMu.Unlock()

	fmt.Printf("[Qoder] Fetched %d model configs\n", len(configs))
	return configs, nil
}

// GetModelConfig returns the model config for a specific model, fetching if needed
func GetModelConfig(creds *Creds, modelKey string) *ModelConfig {
	configs, err := FetchModelConfigs(creds)
	if err == nil {
		if mc, ok := configs[modelKey]; ok {
			return mc
		}
	}

	// Fallback: return a default config
	return &ModelConfig{
		Key:             modelKey,
		IsReasoning:     false,
		MaxOutputTokens: 32768,
		Source:          "system",
	}
}

func credsCacheKey(creds *Creds) string {
	h := sha256.New()
	h.Write([]byte("qoder:" + creds.UserID))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// QoderAllModels returns all known Qoder model IDs
func QoderAllModels() []string {
	return []string{
		"qmodel_latest", "qmodel", "dmodel", "dfmodel",
		"gm51model", "kmodel", "mmodel",
		"auto", "ultimate", "performance", "efficient", "lite",
	}
}
