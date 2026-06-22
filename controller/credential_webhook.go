package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// CredentialRequest 凭证推送请求格式
type CredentialRequest struct {
	Platform     string `json:"platform" binding:"required"`     // kiro, grok, antigravity, codex
	AccessToken  string `json:"access_token" binding:"required"` // OAuth access token
	RefreshToken string `json:"refresh_token"`                   // OAuth refresh token (optional)
	AccountID    string `json:"account_id"`                      // 账户标识（可选）
	Email        string `json:"email"`                           // 用户邮箱（可选）
	ExpiresIn    int64  `json:"expires_in"`                      // token 过期时间（秒）
	BaseURL      string `json:"base_url"`                        // 自定义 base URL（可选）
	ChannelName  string `json:"channel_name"`                    // 自定义 channel 名称（可选）
	Group        string `json:"group"`                           // 用户组（可选，默认 "default"）
}

// CredentialResponse 凭证推送响应
type CredentialResponse struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	ChannelID  int    `json:"channel_id,omitempty"`
	Platform   string `json:"platform"`
	Models     string `json:"models,omitempty"`
	ExpiresAt  int64  `json:"expires_at,omitempty"`
}

// ReceiveCredential 接收 auto_reg 推送的凭证
// POST /api/credential/webhook
func ReceiveCredential(c *gin.Context) {
	var req CredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, CredentialResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	// 验证平台类型
	channelType, models, baseURL := getPlatformConfig(req.Platform)
	if channelType == 0 {
		c.JSON(http.StatusBadRequest, CredentialResponse{
			Success:  false,
			Message:  fmt.Sprintf("Unsupported platform: %s", req.Platform),
			Platform: req.Platform,
		})
		return
	}

	// 如果提供了自定义 base_url，使用它
	if req.BaseURL != "" {
		baseURL = req.BaseURL
	}

	// 构建 API key（不同平台格式不同）
	apiKey := buildAPIKey(req)

	// 生成 channel 名称
	channelName := req.ChannelName
	if channelName == "" {
		channelName = generateChannelName(req)
	}

	// 计算过期时间
	var expiresAt int64
	if req.ExpiresIn > 0 {
		expiresAt = time.Now().Add(time.Duration(req.ExpiresIn) * time.Second).Unix()
	}

	// 检查是否已存在相同平台的 channel
	existingChannel := findExistingChannel(req.Platform, req.AccountID, req.Email)

	var channelID int
	var action string

	if existingChannel != nil {
		// 更新现有 channel
		existingChannel.Key = apiKey
		existingChannel.Name = channelName
		if baseURL != "" {
			existingChannel.BaseURL = &baseURL
		}
		existingChannel.Status = 1 // 启用

		if err := existingChannel.Update(); err != nil {
			c.JSON(http.StatusInternalServerError, CredentialResponse{
				Success:  false,
				Message:  fmt.Sprintf("Failed to update channel: %v", err),
				Platform: req.Platform,
			})
			return
		}

		channelID = existingChannel.Id
		action = "updated"
		common.SysLog(fmt.Sprintf("Channel %d (%s) updated via webhook", channelID, req.Platform))
	} else {
		// 创建新 channel
		channel := &model.Channel{
			Name:        channelName,
			Type:        channelType,
			Key:         apiKey,
			BaseURL:     &baseURL,
			Models:      models,
			Status:      1,
			Weight:      common.GetPointer(uint(0)),
			Priority:    common.GetPointer(int64(0)),
			CreatedTime: time.Now().Unix(),
			Group:       "default",
		}

		if req.Group != "" {
			channel.Group = req.Group
		}

		if err := channel.Insert(); err != nil {
			c.JSON(http.StatusInternalServerError, CredentialResponse{
				Success:  false,
				Message:  fmt.Sprintf("Failed to create channel: %v", err),
				Platform: req.Platform,
			})
			return
		}

		channelID = channel.Id
		action = "created"
		common.SysLog(fmt.Sprintf("Channel %d (%s) created via webhook", channelID, req.Platform))
	}

	c.JSON(http.StatusOK, CredentialResponse{
		Success:   true,
		Message:   fmt.Sprintf("Channel %s successfully", action),
		ChannelID: channelID,
		Platform:  req.Platform,
		Models:    models,
		ExpiresAt: expiresAt,
	})
}

// getPlatformConfig 获取平台配置
func getPlatformConfig(platform string) (channelType int, models string, baseURL string) {
	switch strings.ToLower(platform) {
	case "kiro":
		return constant.ChannelTypeKiro,
			"claude-sonnet-4-5,claude-opus-4-5,claude-opus-4-6,claude-opus-4-7,claude-opus-4-8",
			"https://q.us-east-1.amazonaws.com"

	case "grok", "grok-cli":
		return constant.ChannelTypeGrokCLI,
			"grok-3,grok-3-mini,grok-4,grok-4.1-thinking",
			"https://api.x.ai"

	case "antigravity":
		return constant.ChannelTypeAntigravity,
			"gemini-3-flash,gemini-3-pro-high,gemini-pro-agent,claude-sonnet-4-6",
			"https://daily-cloudcode-pa.googleapis.com"

	case "codex":
		return constant.ChannelTypeCodex,
			"gpt-4o,gpt-4o-mini,o1,o1-mini,o3-mini",
			"https://api.openai.com"

	case "gemini", "gemini-cli":
		return constant.ChannelTypeGeminiCLI,
			"gemini-2.5-pro,gemini-2.5-flash,gemini-2.0-flash",
			"https://generativelanguage.googleapis.com"

	default:
		return 0, "", ""
	}
}

// buildAPIKey 构建 API key（不同平台格式不同）
func buildAPIKey(req CredentialRequest) string {
	switch strings.ToLower(req.Platform) {
	case "codex":
		// Codex 使用 JSON 格式存储 OAuth 信息
		keyData := map[string]interface{}{
			"access_token": req.AccessToken,
		}
		if req.RefreshToken != "" {
			keyData["refresh_token"] = req.RefreshToken
		}
		if req.AccountID != "" {
			keyData["account_id"] = req.AccountID
		}
		if req.Email != "" {
			keyData["email"] = req.Email
		}
		if req.ExpiresIn > 0 {
			keyData["expires_in"] = req.ExpiresIn
			keyData["expires_at"] = time.Now().Add(time.Duration(req.ExpiresIn) * time.Second).Unix()
		}

		keyJSON, _ := json.Marshal(keyData)
		return string(keyJSON)

	case "kiro", "grok", "grok-cli", "antigravity":
		// 这些平台直接使用 Bearer token
		return req.AccessToken

	case "gemini", "gemini-cli":
		// Gemini 使用 API key（不是 OAuth）
		return req.AccessToken

	default:
		return req.AccessToken
	}
}

// generateChannelName 生成 channel 名称
func generateChannelName(req CredentialRequest) string {
	platform := strings.Title(strings.ToLower(req.Platform))

	if req.Email != "" {
		// 使用邮箱前缀
		emailParts := strings.Split(req.Email, "@")
		if len(emailParts) > 0 {
			return fmt.Sprintf("%s - %s", platform, emailParts[0])
		}
	}

	if req.AccountID != "" {
		// 使用账户 ID 后 8 位
		id := req.AccountID
		if len(id) > 8 {
			id = id[len(id)-8:]
		}
		return fmt.Sprintf("%s - %s", platform, id)
	}

	// 使用时间戳
	return fmt.Sprintf("%s - %d", platform, time.Now().Unix())
}

// findExistingChannel 查找已存在的 channel
func findExistingChannel(platform, accountID, email string) *model.Channel {
	channelType, _, _ := getPlatformConfig(platform)
	if channelType == 0 {
		return nil
	}

	// 查找相同类型的 channel
	channels, err := model.GetChannelsByType(0, 100, false, channelType)
	if err != nil || len(channels) == 0 {
		return nil
	}

	// 如果有 account_id 或 email，尝试匹配
	for _, ch := range channels {
		if accountID != "" && strings.Contains(ch.Key, accountID) {
			return ch
		}
		if email != "" && strings.Contains(ch.Key, email) {
			return ch
		}
		// 如果没有标识符，匹配第一个相同平台的 channel
		if accountID == "" && email == "" {
			return ch
		}
	}

	return nil
}

// BatchReceiveCredentials 批量接收凭证
// POST /api/credential/webhook/batch
func BatchReceiveCredentials(c *gin.Context) {
	var reqs []CredentialRequest
	if err := c.ShouldBindJSON(&reqs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	results := make([]CredentialResponse, 0, len(reqs))
	successCount := 0
	failCount := 0

	for _, req := range reqs {
		// 验证平台类型
		channelType, models, baseURL := getPlatformConfig(req.Platform)
		if channelType == 0 {
			results = append(results, CredentialResponse{
				Success:  false,
				Message:  fmt.Sprintf("Unsupported platform: %s", req.Platform),
				Platform: req.Platform,
			})
			failCount++
			continue
		}

		if req.BaseURL != "" {
			baseURL = req.BaseURL
		}

		apiKey := buildAPIKey(req)
		channelName := req.ChannelName
		if channelName == "" {
			channelName = generateChannelName(req)
		}

		existingChannel := findExistingChannel(req.Platform, req.AccountID, req.Email)

		var channelID int
		var success bool
		var message string

		if existingChannel != nil {
			existingChannel.Key = apiKey
			existingChannel.Name = channelName
			if baseURL != "" {
				existingChannel.BaseURL = &baseURL
			}
			existingChannel.Status = 1

			if err := existingChannel.Update(); err != nil {
				success = false
				message = fmt.Sprintf("Failed to update: %v", err)
				failCount++
			} else {
				channelID = existingChannel.Id
				success = true
				message = "Channel updated successfully"
				successCount++
				common.SysLog(fmt.Sprintf("Channel %d (%s) updated via batch webhook", channelID, req.Platform))
			}
		} else {
			channel := &model.Channel{
				Name:        channelName,
				Type:        channelType,
				Key:         apiKey,
				BaseURL:     &baseURL,
				Models:      models,
				Status:      1,
				Weight:      common.GetPointer(uint(0)),
				Priority:    common.GetPointer(int64(0)),
				CreatedTime: time.Now().Unix(),
				Group:       "default",
			}

			if req.Group != "" {
				channel.Group = req.Group
			}

			if err := channel.Insert(); err != nil {
				success = false
				message = fmt.Sprintf("Failed to create: %v", err)
				failCount++
			} else {
				channelID = channel.Id
				success = true
				message = "Channel created successfully"
				successCount++
				common.SysLog(fmt.Sprintf("Channel %d (%s) created via batch webhook", channelID, req.Platform))
			}
		}

		var expiresAt int64
		if req.ExpiresIn > 0 {
			expiresAt = time.Now().Add(time.Duration(req.ExpiresIn) * time.Second).Unix()
		}

		results = append(results, CredentialResponse{
			Success:   success,
			Message:   message,
			ChannelID: channelID,
			Platform:  req.Platform,
			Models:    models,
			ExpiresAt: expiresAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"total":         len(reqs),
		"success_count": successCount,
		"fail_count":    failCount,
		"results":       results,
	})
}
