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

// SearchRequest is the unified search request format
type SearchRequest struct {
	Model    string `json:"model,omitempty"` // for channel routing
	Query      string `json:"query"`
	MaxResults int    `json:"max_results,omitempty"`
	Topic      string `json:"topic,omitempty"`      // tavily: general/news
	Depth      string `json:"depth,omitempty"`       // tavily: basic/advanced
	IncludeDomains []string `json:"include_domains,omitempty"`
	ExcludeDomains []string `json:"exclude_domains,omitempty"`
}

// SearchResult is a single search result
type SearchResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
	Score   float64 `json:"score,omitempty"`
}

// SearchResponse is the unified search response
type SearchResponse struct {
	Results []SearchResult `json:"results"`
	Query   string         `json:"query"`
	Provider string        `json:"provider,omitempty"`
	Took    float64        `json:"took_ms,omitempty"`
}

// HandleSearch handles /v1/search requests
func HandleSearch(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{"message": "Invalid request: " + err.Error(), "type": "invalid_request_error"},
		})
		return
	}

	if strings.TrimSpace(req.Query) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{"message": "Query is required", "type": "invalid_request_error"},
		})
		return
	}

	if req.MaxResults <= 0 {
		req.MaxResults = 5
	}
	if req.MaxResults > 20 {
		req.MaxResults = 20
	}

	channelType := common.GetContextKeyInt(c, constant.ContextKeyChannelType)
	start := time.Now()

	var results []SearchResult
	var err error

	switch channelType {
	case constant.ChannelTypeTavily:
		results, err = searchTavily(c, req)
	case constant.ChannelTypeBrave:
		results, err = searchBrave(c, req)
	case constant.ChannelTypeSerper:
		results, err = searchSerper(c, req)
	case constant.ChannelTypeExa:
		results, err = searchExa(c, req)
	default:
		// Default to Tavily
		results, err = searchTavily(c, req)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{"message": err.Error(), "type": "search_error"},
		})
		return
	}

	resp := SearchResponse{
		Results:  results,
		Query:    req.Query,
		Provider: constant.GetChannelTypeName(channelType),
		Took:     float64(time.Since(start).Milliseconds()),
	}

	c.JSON(http.StatusOK, resp)
}

// searchTavily calls Tavily Search API
func searchTavily(c *gin.Context, req SearchRequest) ([]SearchResult, error) {
	baseURL, apiKey := getChannelCredentials(c)

	payload := map[string]interface{}{
		"query":       req.Query,
		"max_results": req.MaxResults,
		"api_key":     apiKey,
	}
	if req.Topic != "" {
		payload["topic"] = req.Topic
	}
	if req.Depth != "" {
		payload["search_depth"] = req.Depth
	}
	if len(req.IncludeDomains) > 0 {
		payload["include_domains"] = req.IncludeDomains
	}
	if len(req.ExcludeDomains) > 0 {
		payload["exclude_domains"] = req.ExcludeDomains
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", baseURL+"/search", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("tavily request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("tavily returned %d: %s", resp.StatusCode, string(respBody))
	}

	var tavilyResp struct {
		Results []struct {
			Title   string  `json:"title"`
			URL     string  `json:"url"`
			Content string  `json:"content"`
			Score   float64 `json:"score"`
		} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &tavilyResp); err != nil {
		return nil, err
	}

	var results []SearchResult
	for _, r := range tavilyResp.Results {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Content: r.Content,
			Score:   r.Score,
		})
	}
	return results, nil
}

// searchBrave calls Brave Search API
func searchBrave(c *gin.Context, req SearchRequest) ([]SearchResult, error) {
	baseURL, apiKey := getChannelCredentials(c)

	url := fmt.Sprintf("%s/res/v1/web/search?q=%s&count=%d",
		baseURL, strings.ReplaceAll(req.Query, " ", "+"), req.MaxResults)

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("X-Subscription-Token", apiKey)
	httpReq.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("brave request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("brave returned %d: %s", resp.StatusCode, string(respBody))
	}

	var braveResp struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.Unmarshal(respBody, &braveResp); err != nil {
		return nil, err
	}

	var results []SearchResult
	for _, r := range braveResp.Web.Results {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Content: r.Description,
		})
	}
	return results, nil
}

// searchSerper calls Serper API (Google Search)
func searchSerper(c *gin.Context, req SearchRequest) ([]SearchResult, error) {
	baseURL, apiKey := getChannelCredentials(c)

	payload := map[string]interface{}{
		"q":   req.Query,
		"num": req.MaxResults,
	}
	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequest("POST", baseURL+"/search", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("X-API-KEY", apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("serper request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("serper returned %d: %s", resp.StatusCode, string(respBody))
	}

	var serperResp struct {
		Organic []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"organic"`
	}
	if err := json.Unmarshal(respBody, &serperResp); err != nil {
		return nil, err
	}

	var results []SearchResult
	for _, r := range serperResp.Organic {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.Link,
			Content: r.Snippet,
		})
	}
	return results, nil
}

// searchExa calls Exa Search API
func searchExa(c *gin.Context, req SearchRequest) ([]SearchResult, error) {
	baseURL, apiKey := getChannelCredentials(c)

	payload := map[string]interface{}{
		"query":         req.Query,
		"numResults":    req.MaxResults,
		"contents":      map[string]bool{"text": true},
	}
	if len(req.IncludeDomains) > 0 {
		payload["includeDomains"] = req.IncludeDomains
	}
	if len(req.ExcludeDomains) > 0 {
		payload["excludeDomains"] = req.ExcludeDomains
	}

	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequest("POST", baseURL+"/search", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("exa request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("exa returned %d: %s", resp.StatusCode, string(respBody))
	}

	var exaResp struct {
		Results []struct {
			Title string `json:"title"`
			URL   string `json:"url"`
			Text  string `json:"text"`
			Score float64 `json:"score"`
		} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &exaResp); err != nil {
		return nil, err
	}

	var results []SearchResult
	for _, r := range exaResp.Results {
		content := r.Text
		if len(content) > 2000 {
			content = content[:2000] + "..."
		}
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Content: content,
			Score:   r.Score,
		})
	}
	return results, nil
}

func getChannelCredentials(c *gin.Context) (string, string) {
	baseURL := common.GetContextKeyString(c, constant.ContextKeyChannelBaseUrl)
	apiKey := common.GetContextKeyString(c, constant.ContextKeyChannelKey)
	return baseURL, apiKey
}

