package controller

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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

// Qoder OAuth Device Token Flow
// Ported from 9router/src/lib/oauth/services/qoder.js
//
// Flow:
// 1. Generate PKCE (32 random bytes → base64url verifier → SHA256 challenge)
// 2. Generate nonce (UUID) + machineId (UUID)
// 3. Open https://qoder.com/device/selectAccounts?challenge=...&nonce=...&machine_id=...
// 4. Poll GET https://openapi.qoder.sh/api/v1/deviceToken/poll?nonce=...&verifier=...
// 5. On 200: get { token: "dt-...", user_id, expires_at }
// 6. Create channel with token as key

const (
	qoderLoginURL         = "https://qoder.com/device/selectAccounts"
	qoderDeviceTokenURL   = "https://openapi.qoder.sh/api/v1/deviceToken/poll"
	qoderChatBaseURL      = "https://api3.qoder.sh"
	qoderPollTimeout      = 5 * time.Minute
	qoderFetchTimeout     = 15 * time.Second
	qoderClientID         = "e883ade2-e6e3-4d6d-adf7-f92ceff5fdcb" // CLI client_id (reversed from qodercli binary v1.0.24)
)

type qoderSession struct {
	CodeVerifier string
	MachineID    string
	Nonce        string
	LoginURL     string
	Token        string
	UserID       string
	Status       string // "pending", "authorized", "failed"
	CreatedAt    time.Time
}

var (
	qoderSessions   = make(map[string]*qoderSession)
	qoderSessionsMu sync.RWMutex
)

// QoderStartAuth initiates the Qoder OAuth device flow
func QoderStartAuth(c *gin.Context) {
	// Generate PKCE pair (32 random bytes)
	verifierBytes := make([]byte, 32)
	rand.Read(verifierBytes)
	verifier := base64URLEncode(verifierBytes)

	challengeHash := sha256.Sum256([]byte(verifier))
	challenge := base64URLEncode(challengeHash[:])

	// Generate nonce and machine ID
	nonce := generateQoderUUID()
	machineID := generateQoderUUID()

	// Build login URL with CLI client_id
	loginURL := fmt.Sprintf("%s?challenge=%s&challenge_method=S256&nonce=%s&machine_id=%s&client_id=%s",
		qoderLoginURL, challenge, nonce, machineID, qoderClientID)

	sessionID := base64URLEncode(randomBytes(32))

	qoderSessionsMu.Lock()
	qoderSessions[sessionID] = &qoderSession{
		CodeVerifier: verifier,
		MachineID:    machineID,
		Nonce:        nonce,
		LoginURL:     loginURL,
		Status:       "pending",
		CreatedAt:    time.Now(),
	}
	qoderSessionsMu.Unlock()

	// Cleanup old sessions
	go cleanupQoderSessions()

	common.SysLog(fmt.Sprintf("[Qoder] Auth started session=%s nonce=%s", sessionID[:8], nonce[:8]))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"session_id": sessionID,
			"login_url":  loginURL,
			"nonce":      nonce[:8],
		},
	})
}

// QoderPollToken polls for the Qoder device token
func QoderPollToken(c *gin.Context) {
	sessionID := c.Param("session_id")

	qoderSessionsMu.RLock()
	session, ok := qoderSessions[sessionID]
	qoderSessionsMu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "session not found"})
		return
	}

	if session.Status == "authorized" {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"status":  "authorized",
				"token":   session.Token,
				"user_id": session.UserID,
			},
		})
		return
	}

	// Check timeout
	if time.Since(session.CreatedAt) > qoderPollTimeout {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    gin.H{"status": "timeout"},
		})
		return
	}

	// Poll the device token endpoint
	token, userID, err := qoderPollDeviceToken(session.Nonce, session.CodeVerifier)
	if err != nil {
		// Still pending
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    gin.H{"status": "pending"},
		})
		return
	}

	// Token obtained!
	qoderSessionsMu.Lock()
	session.Token = token
	session.UserID = userID
	session.Status = "authorized"
	qoderSessionsMu.Unlock()

	common.SysLog(fmt.Sprintf("[Qoder] Auth success! token=%s... user=%s", token[:min(8, len(token))], userID))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"status":  "authorized",
			"token":   token,
			"user_id": userID,
		},
	})
}

// QoderAutoCreateChannel creates or appends to a Qoder channel after successful auth
// Supports multi-account stacking: multiple accounts in one channel with round-robin rotation
func QoderAutoCreateChannel(c *gin.Context) {
	var req struct {
		SessionID string `json:"session_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	qoderSessionsMu.RLock()
	session, ok := qoderSessions[req.SessionID]
	qoderSessionsMu.RUnlock()

	if !ok || session.Status != "authorized" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "not authorized"})
		return
	}

	// Build new credential JSON
	newCredJSON, _ := json.Marshal(map[string]string{
		"user_id":    session.UserID,
		"auth_token": session.Token,
		"machine_id": session.MachineID,
	})
	newCredStr := string(newCredJSON)

	// Check if a Qoder multi-key channel already exists
	var existingChannel model.Channel
	err := model.DB.Where("type = ? AND status = 1", 67).First(&existingChannel).Error

	if err == nil {
		// Channel exists - append new credential (multi-account stacking)
		existingKeys := strings.Split(strings.TrimSpace(existingChannel.Key), "\n")
		
		// Check for duplicate (same user_id)
		isDuplicate := false
		for _, k := range existingKeys {
			var existing map[string]string
			if json.Unmarshal([]byte(k), &existing) == nil {
				if existing["user_id"] == session.UserID {
					isDuplicate = true
					// Update the existing credential with new token
					existing["auth_token"] = session.Token
					existing["machine_id"] = session.MachineID
					updatedCred, _ := json.Marshal(existing)
					for i, ek := range existingKeys {
						var em map[string]string
						if json.Unmarshal([]byte(ek), &em) == nil && em["user_id"] == session.UserID {
							existingKeys[i] = string(updatedCred)
							break
						}
					}
					break
				}
			}
		}
		
		if !isDuplicate {
			existingKeys = append(existingKeys, newCredStr)
		}

		// Update channel with all keys
		mergedKey := strings.Join(existingKeys, "\n")
		existingChannel.Key = mergedKey
		
		// Enable multi-key mode with polling rotation
		channelInfo := existingChannel.ChannelInfo
		channelInfo.IsMultiKey = true
		channelInfo.MultiKeyMode = "polling"
		channelInfo.MultiKeySize = len(existingKeys)
		existingChannel.ChannelInfo = channelInfo
		model.DB.Save(&existingChannel)

		action := "updated"
		if !isDuplicate {
			action = "appended"
		}
		common.SysLog(fmt.Sprintf("[Qoder] Credential %s to channel #%d (%d accounts total)", action, existingChannel.Id, len(existingKeys)))

		// Cleanup session
		qoderSessionsMu.Lock()
		delete(qoderSessions, req.SessionID)
		qoderSessionsMu.Unlock()

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"channel_id":    existingChannel.Id,
				"name":          existingChannel.Name,
				"action":        action,
				"account_count": len(existingKeys),
				"user_id":       session.UserID,
			},
		})
		return
	}

	// No existing channel - create new one with multi-key mode enabled
	channelName := "Qoder (multi-account)"
	baseURL := qoderChatBaseURL
	channel := &model.Channel{
		Type:    67,
		Name:    channelName,
		Key:     newCredStr,
		BaseURL: &baseURL,
		Models:  "qmodel_latest,qmodel,dmodel,dfmodel,gm51model,kmodel,mmodel,auto,ultimate,performance,efficient,lite",
		Status:  1,
	}
	channel.ChannelInfo.IsMultiKey = true
	channel.ChannelInfo.MultiKeyMode = "polling"
	channel.ChannelInfo.MultiKeySize = 1

	if err := model.DB.Create(channel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	// Cleanup session
	qoderSessionsMu.Lock()
	delete(qoderSessions, req.SessionID)
	qoderSessionsMu.Unlock()

	common.SysLog(fmt.Sprintf("[Qoder] Channel created: #%d %s (multi-key mode)", channel.Id, channelName))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"channel_id":    channel.Id,
			"name":          channelName,
			"action":        "created",
			"account_count": 1,
			"user_id":       session.UserID,
		},
	})
}

// qoderPollDeviceToken polls openapi.qoder.sh for the device token
func qoderPollDeviceToken(nonce, verifier string) (string, string, error) {
	url := fmt.Sprintf("%s?nonce=%s&verifier=%s&challenge_method=S256",
		qoderDeviceTokenURL, nonce, verifier)

	client := &http.Client{Timeout: qoderFetchTimeout}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Go-http-client/2.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	// 202 or 404 = still pending
	if resp.StatusCode == 202 || resp.StatusCode == 404 {
		return "", "", fmt.Errorf("pending")
	}

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Token     string `json:"token"`
		UserID    string `json:"user_id"`
		ExpiresAt any    `json:"expires_at"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", fmt.Errorf("invalid response: %w", err)
	}

	if result.Token == "" {
		return "", "", fmt.Errorf("no token in response")
	}

	return result.Token, result.UserID, nil
}

// Helper functions
func base64URLEncode(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}

func generateQoderUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func cleanupQoderSessions() {
	qoderSessionsMu.Lock()
	defer qoderSessionsMu.Unlock()
	cutoff := time.Now().Add(-10 * time.Minute)
	for id, s := range qoderSessions {
		if s.CreatedAt.Before(cutoff) {
			delete(qoderSessions, id)
		}
	}
}

func strPtr(s string) *string {
	return &s
}
