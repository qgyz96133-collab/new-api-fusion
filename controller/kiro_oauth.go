package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const (
	// Kiro CLI 认证配置（从 kiro-cli 二进制逆向）
	kiroSSOEndpoint    = "https://oidc.us-east-1.amazonaws.com"
	kiroClientType     = "public"  // CLI 使用 public client
	kiroUserAgent      = "aws-sdk-rust/2.8.1/AWS_TOOLING_USER_AGENT kiro-cli/2.8.1"
	kiroPollInterval   = 5 * time.Second
	kiroPollTimeout    = 10 * time.Minute
	kiroChannelType    = 65
	kiroBaseURL        = "https://codewhisperer.us-east-1.amazonaws.com"
	kiroStartURL       = "https://view.awsapps.com/start"  // AWS Builder ID start URL
	kiroTokenNamespace = "kirocli:odic:token"  // CLI token storage namespace
)

// kiroHTTPClient 使用标准 Go TLS，配合 Kiro CLI 风格的 User-Agent
var kiroHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &kiroTransport{base: http.DefaultTransport},
}

type kiroTransport struct {
	base http.RoundTripper
}

func (t *kiroTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", kiroUserAgent)
	req.Header.Set("X-Amz-User-Agent", kiroUserAgent)
	req.Header.Set("Content-Type", "application/json")
	return t.base.RoundTrip(req)
}

type kiroSession struct {
	DeviceCode      string
	UserCode        string
	VerificationURI string
	ClientID        string
	ClientSecret    string
	AccessToken     string
	RefreshToken    string
	Status          string // "pending", "authorized", "failed"
	CreatedAt       time.Time
}

var (
	kiroSessions   = make(map[string]*kiroSession)
	kiroSessionsMu sync.RWMutex
)

// KiroStartDeviceAuth starts the AWS Builder ID device code flow (CLI style)
func KiroStartDeviceAuth(c *gin.Context) {
	// 1. Register OIDC client (CLI style)
	regPayload, _ := json.Marshal(map[string]interface{}{
		"clientName": "kiro-cli",
		"clientType": kiroClientType,
		"scopes":     []string{"codewhisperer:completions", "codewhisperer:analysis", "codewhisperer:conversations"},
		"grantTypes": []string{"urn:ietf:params:oauth:grant-type:device_code", "refresh_token"},
		"issuerUrl":  "https://identitycenter.amazonaws.com",
	})

	regReq, _ := http.NewRequest("POST", kiroSSOEndpoint+"/client/register", strings.NewReader(string(regPayload)))
	regReq.Header.Set("Content-Type", "application/json")
	regResp, err := kiroHTTPClient.Do(regReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "OIDC registration failed: " + err.Error()})
		return
	}
	defer regResp.Body.Close()

	regBody, _ := io.ReadAll(regResp.Body)
	if regResp.StatusCode != 200 && regResp.StatusCode != 201 {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": fmt.Sprintf("OIDC registration returned %d: %s", regResp.StatusCode, string(regBody[:min(300, len(regBody))]))})
		return
	}

	var regResult struct {
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
	}
	if err := json.Unmarshal(regBody, &regResult); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to parse registration response"})
		return
	}

	// 2. Request device code (CLI style with start_url)
	dcPayload, _ := json.Marshal(map[string]interface{}{
		"clientId":     regResult.ClientID,
		"clientSecret": regResult.ClientSecret,
		"startUrl":     kiroStartURL,
	})

	dcReq, _ := http.NewRequest("POST", kiroSSOEndpoint+"/device_authorization", strings.NewReader(string(dcPayload)))
	dcReq.Header.Set("Content-Type", "application/json")
	dcResp, err := kiroHTTPClient.Do(dcReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Device code request failed: " + err.Error()})
		return
	}
	defer dcResp.Body.Close()

	dcBody, _ := io.ReadAll(dcResp.Body)
	if dcResp.StatusCode != 200 {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": fmt.Sprintf("Device code returned %d: %s", dcResp.StatusCode, string(dcBody[:min(300, len(dcBody))]))})
		return
	}

	var dcResult struct {
		DeviceCode              string `json:"deviceCode"`
		UserCode                string `json:"userCode"`
		VerificationURI         string `json:"verificationUri"`
		VerificationURIComplete string `json:"verificationUriComplete"`
		ExpiresIn               int    `json:"expiresIn"`
		Interval                int    `json:"interval"`
	}
	if err := json.Unmarshal(dcBody, &dcResult); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to parse device code response"})
		return
	}

	sessionID := base64URLEncode(randomBytes(32))
	kiroSessionsMu.Lock()
	kiroSessions[sessionID] = &kiroSession{
		DeviceCode:      dcResult.DeviceCode,
		UserCode:        dcResult.UserCode,
		VerificationURI: dcResult.VerificationURI,
		ClientID:        regResult.ClientID,
		ClientSecret:    regResult.ClientSecret,
		Status:          "pending",
		CreatedAt:       time.Now(),
	}
	kiroSessionsMu.Unlock()

	common.SysLog(fmt.Sprintf("[Kiro] Device auth started: session=%s user_code=%s", sessionID[:8], dcResult.UserCode))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"session_id":                sessionID,
			"user_code":                 dcResult.UserCode,
			"verification_uri":          dcResult.VerificationURI,
			"verification_uri_complete": dcResult.VerificationURIComplete,
			"expires_in":               dcResult.ExpiresIn,
		},
	})
}

// KiroPollToken polls for the Kiro access token
func KiroPollToken(c *gin.Context) {
	sessionID := c.Param("session_id")

	kiroSessionsMu.RLock()
	session, ok := kiroSessions[sessionID]
	kiroSessionsMu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "session not found"})
		return
	}

	if session.Status == "authorized" {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "authorized", "token": session.AccessToken}})
		return
	}

	if time.Since(session.CreatedAt) > kiroPollTimeout {
		kiroSessionsMu.Lock()
		session.Status = "failed"
		kiroSessionsMu.Unlock()
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "timeout"}})
		return
	}

	// Try to exchange device code for token
	tokenPayload, _ := json.Marshal(map[string]interface{}{
		"clientId":     session.ClientID,
		"clientSecret": session.ClientSecret,
		"deviceCode":   session.DeviceCode,
		"grantType":    "urn:ietf:params:oauth:grant-type:device_code",
	})

	tokenReq, _ := http.NewRequest("POST", kiroSSOEndpoint+"/token", strings.NewReader(string(tokenPayload)))
	tokenReq.Header.Set("Content-Type", "application/json")
	tokenResp, err := kiroHTTPClient.Do(tokenReq)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "pending"}})
		return
	}
	defer tokenResp.Body.Close()

	tokenBody, _ := io.ReadAll(tokenResp.Body)

	// authorization_pending = still waiting
	if tokenResp.StatusCode == 400 && strings.Contains(string(tokenBody), "authorization_pending") {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "pending"}})
		return
	}

	if tokenResp.StatusCode != 200 {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "pending", "detail": string(tokenBody[:min(200, len(tokenBody))])}})
		return
	}

	var tokenResult struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn    int    `json:"expiresIn"`
		TokenType    string `json:"tokenType"`
	}
	if err := json.Unmarshal(tokenBody, &tokenResult); err != nil || tokenResult.AccessToken == "" {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "pending"}})
		return
	}

	kiroSessionsMu.Lock()
	session.AccessToken = tokenResult.AccessToken
	session.RefreshToken = tokenResult.RefreshToken
	session.Status = "authorized"
	kiroSessionsMu.Unlock()

	common.SysLog(fmt.Sprintf("[Kiro] Auth success! token=%s...", tokenResult.AccessToken[:min(15, len(tokenResult.AccessToken))]))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"status":        "authorized",
			"token":         tokenResult.AccessToken,
			"refresh_token": tokenResult.RefreshToken,
			"expires_in":    tokenResult.ExpiresIn,
		},
	})
}

// KiroImportToken manually imports a Kiro token (paste JSON or raw token)
func KiroImportToken(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"` // raw token or JSON
		Name  string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	kiroSessionsMu.Lock()
	sessionID := base64URLEncode(randomBytes(32))
	kiroSessions[sessionID] = &kiroSession{
		AccessToken: req.Token,
		Status:      "authorized",
		CreatedAt:   time.Now(),
	}
	kiroSessionsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"session_id": sessionID},
	})
}

// KiroCreateChannel creates or appends to a Kiro channel (same multi-account stacking as Qoder)
func KiroCreateChannel(c *gin.Context) {
	var req struct {
		SessionID string `json:"session_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	kiroSessionsMu.RLock()
	session, ok := kiroSessions[req.SessionID]
	kiroSessionsMu.RUnlock()

	if !ok || session.Status != "authorized" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "not authorized"})
		return
	}

	// Build credential string: accessToken|refreshToken|clientId|clientSecret
	// so the relay can refresh when needed
	credParts := []string{session.AccessToken}
	if session.RefreshToken != "" {
		credParts = append(credParts, session.RefreshToken)
	}
	if session.ClientID != "" {
		credParts = append(credParts, session.ClientID)
	}
	if session.ClientSecret != "" {
		credParts = append(credParts, session.ClientSecret)
	}
	newCredStr := strings.Join(credParts, "|")

	// Check for existing Kiro channel (multi-account stacking)
	var existingChannel model.Channel
	err := model.DB.Where("type = ? AND status = 1", kiroChannelType).First(&existingChannel).Error

	if err == nil {
		// Append to existing channel
		existingKeys := strings.Split(strings.TrimSpace(existingChannel.Key), "\n")
		existingKeys = append(existingKeys, newCredStr)
		mergedKey := strings.Join(existingKeys, "\n")

		model.DB.Model(&existingChannel).Updates(map[string]interface{}{"key": mergedKey})

		channelInfo := existingChannel.ChannelInfo
		channelInfo.IsMultiKey = true
		channelInfo.MultiKeyMode = "polling"
		channelInfo.MultiKeySize = len(existingKeys)
		existingChannel.ChannelInfo = channelInfo
		model.DB.Save(&existingChannel)

		kiroSessionsMu.Lock()
		delete(kiroSessions, req.SessionID)
		kiroSessionsMu.Unlock()

		common.SysLog(fmt.Sprintf("[Kiro] Credential appended to channel #%d (%d accounts)", existingChannel.Id, len(existingKeys)))

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"channel_id":    existingChannel.Id,
				"name":          existingChannel.Name,
				"action":        "appended",
				"account_count": len(existingKeys),
			},
		})
		return
	}

	// Create new channel
	channelName := "Kiro AI (multi-account)"
	baseURL := kiroBaseURL
	channel := &model.Channel{
		Type:    kiroChannelType,
		Name:    channelName,
		Key:     newCredStr,
		BaseURL: &baseURL,
		Models:  "claude-haiku-4-5,claude-sonnet-4-5,claude-sonnet-4-6,claude-opus-4-5,claude-opus-4-6,claude-opus-4-7,claude-opus-4-8",
		Status:  1,
	}
	channel.ChannelInfo.IsMultiKey = true
	channel.ChannelInfo.MultiKeyMode = "polling"
	channel.ChannelInfo.MultiKeySize = 1

	if err := model.DB.Create(channel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	kiroSessionsMu.Lock()
	delete(kiroSessions, req.SessionID)
	kiroSessionsMu.Unlock()

	common.SysLog(fmt.Sprintf("[Kiro] Channel created: #%d %s", channel.Id, channelName))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"channel_id":    channel.Id,
			"name":          channelName,
			"action":        "created",
			"account_count": 1,
		},
	})
}
