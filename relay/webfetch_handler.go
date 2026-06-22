package relay

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
)

// WebFetchRequest is the unified web fetch request format
type WebFetchRequest struct {
	Model    string `json:"model,omitempty"` // for channel routing
	URL       string `json:"url"`
	Format    string `json:"format,omitempty"`     // markdown, text, html (default: markdown)
	WaitFor   int    `json:"wait_for,omitempty"`   // ms to wait for page load
	OnlyMain  bool   `json:"only_main,omitempty"`  // extract only main content
}

// WebFetchResponse is the unified web fetch response
type WebFetchResponse struct {
	Content  string            `json:"content"`
	Title    string            `json:"title,omitempty"`
	URL      string            `json:"url"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Provider string            `json:"provider,omitempty"`
	TookMs   float64           `json:"took_ms,omitempty"`
}

// HandleWebFetch handles /v1/web/fetch requests
func HandleWebFetch(c *gin.Context) {
	var req WebFetchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{"message": "Invalid request: " + err.Error(), "type": "invalid_request_error"},
		})
		return
	}

	if strings.TrimSpace(req.URL) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{"message": "URL is required", "type": "invalid_request_error"},
		})
		return
	}

	if req.Format == "" {
		req.Format = "markdown"
	}

	channelType := common.GetContextKeyInt(c, constant.ContextKeyChannelType)
	start := time.Now()

	var result WebFetchResponse
	var err error

	switch channelType {
	case constant.ChannelTypeJinaReader:
		result, err = fetchJinaReader(c, req)
	case constant.ChannelTypeFirecrawl:
		result, err = fetchFirecrawl(c, req)
	default:
		result, err = fetchJinaReader(c, req)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{"message": err.Error(), "type": "fetch_error"},
		})
		return
	}

	result.Provider = constant.GetChannelTypeName(channelType)
	result.TookMs = float64(time.Since(start).Milliseconds())
	c.JSON(http.StatusOK, result)
}

// fetchJinaReader uses Jina Reader API (r.jina.ai)
func fetchJinaReader(c *gin.Context, req WebFetchRequest) (WebFetchResponse, error) {
	baseURL, apiKey := getChannelCredentials(c)

	readerURL := baseURL + "/" + req.URL

	httpReq, err := http.NewRequest("GET", readerURL, nil)
	if err != nil {
		return WebFetchResponse{}, err
	}

	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	switch req.Format {
	case "text":
		httpReq.Header.Set("Accept", "text/plain")
	case "html":
		httpReq.Header.Set("Accept", "text/html")
	default:
		httpReq.Header.Set("Accept", "text/markdown")
	}

	if req.OnlyMain {
		httpReq.Header.Set("X-Return-Format", "markdown")
		httpReq.Header.Set("X-Target-Selector", "article, main, .content")
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return WebFetchResponse{}, fmt.Errorf("jina reader request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return WebFetchResponse{}, err
	}

	if resp.StatusCode != 200 {
		return WebFetchResponse{}, fmt.Errorf("jina reader returned %d: %s", resp.StatusCode, string(body[:min(500, len(body))]))
	}

	content := string(body)
	title := resp.Header.Get("X-Title")

	return WebFetchResponse{
		Content: content,
		Title:   title,
		URL:     req.URL,
	}, nil
}

// fetchFirecrawl uses Firecrawl API
func fetchFirecrawl(c *gin.Context, req WebFetchRequest) (WebFetchResponse, error) {
	baseURL, apiKey := getChannelCredentials(c)

	format := req.Format
	if format == "text" {
		format = "markdown" // firecrawl doesn't have plain text, use markdown
	}

	payload := map[string]interface{}{
		"url":    req.URL,
		"formats": []string{format},
	}
	if req.OnlyMain {
		payload["onlyMainContent"] = true
	}
	if req.WaitFor > 0 {
		payload["waitFor"] = req.WaitFor
	}

	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequest("POST", baseURL+"/v1/scrape", strings.NewReader(string(body)))
	if err != nil {
		return WebFetchResponse{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return WebFetchResponse{}, fmt.Errorf("firecrawl request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return WebFetchResponse{}, err
	}

	if resp.StatusCode != 200 {
		return WebFetchResponse{}, fmt.Errorf("firecrawl returned %d: %s", resp.StatusCode, string(respBody[:min(500, len(respBody))]))
	}

	var fcResp struct {
		Success bool `json:"success"`
		Data    struct {
			Markdown string `json:"markdown"`
			HTML     string `json:"html"`
			Metadata struct {
				Title       string `json:"title"`
				Description string `json:"description"`
				SourceURL   string `json:"sourceURL"`
			} `json:"metadata"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &fcResp); err != nil {
		return WebFetchResponse{}, err
	}

	content := fcResp.Data.Markdown
	if req.Format == "html" {
		content = fcResp.Data.HTML
	}

	metadata := map[string]string{}
	if fcResp.Data.Metadata.Description != "" {
		metadata["description"] = fcResp.Data.Metadata.Description
	}

	return WebFetchResponse{
		Content:  content,
		Title:    fcResp.Data.Metadata.Title,
		URL:      req.URL,
		Metadata: metadata,
	}, nil
}
