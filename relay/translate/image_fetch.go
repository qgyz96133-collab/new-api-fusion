package translate

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// FetchRemoteImages converts remote image URLs (http/https) to base64 data URIs.
// Only modifies image_url blocks with external URLs; base64 and file URIs are untouched.
// On any failure (timeout, bad status, network error), leaves the original URL unchanged.
func FetchRemoteImages(messages []interface{}, timeout time.Duration) []interface{} {
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	for _, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}
		contentArr, ok := msgMap["content"].([]interface{})
		if !ok {
			continue
		}

		for _, block := range contentArr {
			blockMap, ok := block.(map[string]interface{})
			if !ok {
				continue
			}

			// Handle image_url blocks
			if blockType, _ := blockMap["type"].(string); blockType == "image_url" {
				if imageURL, ok := blockMap["image_url"].(map[string]interface{}); ok {
					if urlStr, ok := imageURL["url"].(string); ok {
						if strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://") {
							if dataURI, err := fetchImageAsBase64(urlStr, timeout); err == nil {
								imageURL["url"] = dataURI
							}
							// On error: leave original URL unchanged
						}
					}
				}
			}
		}
	}

	return messages
}

// fetchImageAsBase64 downloads a remote image and returns a data URI string.
func fetchImageAsBase64(imageURL string, timeout time.Duration) (string, error) {
	client := &http.Client{Timeout: timeout}

	resp, err := client.Get(imageURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Read body (cap at 10MB to prevent abuse)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return "", err
	}

	// Determine MIME type
	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "image/jpeg"
	}
	// Strip parameters (e.g. "image/png; charset=utf-8" → "image/png")
	if idx := strings.IndexByte(mimeType, ';'); idx != -1 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}

	// Build data URI
	b64 := base64.StdEncoding.EncodeToString(body)
	return "data:" + mimeType + ";base64," + b64, nil
}
