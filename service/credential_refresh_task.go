package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

const (
	CredentialRefreshCheckInterval = 5 * time.Minute
	CredentialRefreshThreshold     = 30 * time.Minute
)

func StartCredentialRefreshTask() {
	common.SysLog("Starting credential auto-refresh task...")
	go runCredentialRefreshLoop()
}

func runCredentialRefreshLoop() {
	ticker := time.NewTicker(CredentialRefreshCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		checkAndRefreshCredentials()
	}
}

func checkAndRefreshCredentials() {
	ctx := context.Background()

	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		common.SysError("credential refresh: failed to get channels: " + err.Error())
		return
	}

	for _, ch := range channels {
		if ch.Status != 1 {
			continue
		}

		switch ch.Type {
		case constant.ChannelTypeKiro:
			checkAndRefreshKiroCredential(ctx, ch)
		case constant.ChannelTypeGrokCLI:
			checkAndRefreshGrokCredential(ctx, ch)
		case constant.ChannelTypeAntigravity:
			checkAndRefreshAntigravityCredential(ctx, ch)
		case constant.ChannelTypeCodex:
			checkAndRefreshCodexCredential(ctx, ch)
		}
	}
}

func checkAndRefreshKiroCredential(ctx context.Context, ch *model.Channel) {
	oauthKey, err := parseKiroOAuthKey(strings.TrimSpace(ch.Key))
	if err != nil {
		return
	}
	if strings.TrimSpace(oauthKey.RefreshToken) == "" {
		return
	}
	if strings.TrimSpace(oauthKey.ExpiresAt) == "" {
		return
	}

	expiresAt, err := time.Parse(time.RFC3339, oauthKey.ExpiresAt)
	if err != nil {
		return
	}

	if time.Until(expiresAt) > CredentialRefreshThreshold {
		return
	}

	common.SysLog(fmt.Sprintf("Kiro channel %d token expiring soon, refreshing...", ch.Id))
	_, _, err = RefreshKiroChannelCredential(ctx, ch.Id, KiroCredentialRefreshOptions{ResetCaches: true})
	if err != nil {
		common.SysError(fmt.Sprintf("Failed to refresh Kiro channel %d: %v", ch.Id, err))
	} else {
		common.SysLog(fmt.Sprintf("Successfully refreshed Kiro channel %d", ch.Id))
	}
}

func checkAndRefreshGrokCredential(ctx context.Context, ch *model.Channel) {
	oauthKey, err := parseGrokOAuthKey(strings.TrimSpace(ch.Key))
	if err != nil {
		return
	}
	if strings.TrimSpace(oauthKey.RefreshToken) == "" {
		return
	}
	if strings.TrimSpace(oauthKey.ExpiresAt) == "" {
		return
	}

	expiresAt, err := time.Parse(time.RFC3339, oauthKey.ExpiresAt)
	if err != nil {
		return
	}

	if time.Until(expiresAt) > CredentialRefreshThreshold {
		return
	}

	common.SysLog(fmt.Sprintf("Grok channel %d token expiring soon, refreshing...", ch.Id))
	_, _, err = RefreshGrokChannelCredential(ctx, ch.Id, GrokCredentialRefreshOptions{ResetCaches: true})
	if err != nil {
		common.SysError(fmt.Sprintf("Failed to refresh Grok channel %d: %v", ch.Id, err))
	} else {
		common.SysLog(fmt.Sprintf("Successfully refreshed Grok channel %d", ch.Id))
	}
}

func checkAndRefreshAntigravityCredential(ctx context.Context, ch *model.Channel) {
	oauthKey, err := parseAntigravityOAuthKey(strings.TrimSpace(ch.Key))
	if err != nil {
		return
	}
	if strings.TrimSpace(oauthKey.RefreshToken) == "" {
		return
	}
	if strings.TrimSpace(oauthKey.ExpiresAt) == "" {
		return
	}

	expiresAt, err := time.Parse(time.RFC3339, oauthKey.ExpiresAt)
	if err != nil {
		return
	}

	if time.Until(expiresAt) > CredentialRefreshThreshold {
		return
	}

	common.SysLog(fmt.Sprintf("Antigravity channel %d token expiring soon, refreshing...", ch.Id))
	_, _, err = RefreshAntigravityChannelCredential(ctx, ch.Id, AntigravityCredentialRefreshOptions{ResetCaches: true})
	if err != nil {
		common.SysError(fmt.Sprintf("Failed to refresh Antigravity channel %d: %v", ch.Id, err))
	} else {
		common.SysLog(fmt.Sprintf("Successfully refreshed Antigravity channel %d", ch.Id))
	}
}

func checkAndRefreshCodexCredential(ctx context.Context, ch *model.Channel) {
	oauthKey, err := parseCodexOAuthKey(strings.TrimSpace(ch.Key))
	if err != nil {
		return
	}
	if strings.TrimSpace(oauthKey.RefreshToken) == "" {
		return
	}
	if strings.TrimSpace(oauthKey.Expired) == "" {
		return
	}

	expiresAt, err := time.Parse(time.RFC3339, oauthKey.Expired)
	if err != nil {
		return
	}

	if time.Until(expiresAt) > CredentialRefreshThreshold {
		return
	}

	common.SysLog(fmt.Sprintf("Codex channel %d token expiring soon, refreshing...", ch.Id))
	_, _, err = RefreshCodexChannelCredential(ctx, ch.Id, CodexCredentialRefreshOptions{ResetCaches: true})
	if err != nil {
		common.SysError(fmt.Sprintf("Failed to refresh Codex channel %d: %v", ch.Id, err))
	} else {
		common.SysLog(fmt.Sprintf("Successfully refreshed Codex channel %d", ch.Id))
	}
}
