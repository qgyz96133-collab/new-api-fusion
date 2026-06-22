package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// DingTalk OAuth (ported from sub2api auth_dingtalk_oauth.go)
// Enterprise login via DingTalk OAuth2

type DingTalkConfig struct {
	Enabled      bool   `json:"enabled"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	CorpID       string `json:"corp_id"`
	RedirectURI  string `json:"redirect_uri"`
}

var dingtalkConfig DingTalkConfig

func SetDingTalkConfig(cfg DingTalkConfig) {
	dingtalkConfig = cfg
}

func GetDingTalkConfig() DingTalkConfig {
	return dingtalkConfig
}

// DingTalkOAuthRedirect redirects to DingTalk OAuth
func DingTalkOAuthRedirect(c *gin.Context) {
	if !dingtalkConfig.Enabled || dingtalkConfig.ClientID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "DingTalk OAuth not configured"})
		return
	}

	session := sessions.Default(c)
	state := fmt.Sprintf("%d", time.Now().UnixNano())
	session.Set("oauth_state", state)
	session.Save()

	authURL := fmt.Sprintf(
		"https://login.dingtalk.com/oauth2/auth?client_id=%s&redirect_uri=%s&response_type=code&scope=openid&state=%s&prompt=consent",
		dingtalkConfig.ClientID,
		dingtalkConfig.RedirectURI,
		state,
	)
	c.Redirect(http.StatusFound, authURL)
}

// DingTalkOAuthCallback handles DingTalk OAuth callback
func DingTalkOAuthCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	session := sessions.Default(c)
	savedState := session.Get("oauth_state")
	if savedState == nil || savedState.(string) != state {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid state"})
		return
	}
	session.Delete("oauth_state")
	session.Save()

	// Exchange code for access token
	tokenResp, err := exchangeDingTalkCode(code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	// Get user info
	userInfo, err := getDingTalkUserInfo(tokenResp.AccessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	// Find or create user
	user := model.FindUserByDingTalkID(userInfo.OpenID)
	if user == nil {
		user = &model.User{
			Username:    fmt.Sprintf("dingtalk_%s", userInfo.OpenID[:8]),
			DisplayName: userInfo.Name,
			Status:      common.UserStatusEnabled,
			Role:        common.RoleCommonUser,
		}
		if err := model.DB.Create(user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to create user"})
			return
		}
	}

	// Login
	setupLogin(user, c)
}

type dingTalkTokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpireIn     int    `json:"expireIn"`
}

type dingTalkUserInfo struct {
	OpenID string `json:"openId"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Avatar string `json:"avatarUrl"`
}

func exchangeDingTalkCode(code string) (*dingTalkTokenResponse, error) {
	payload := fmt.Sprintf(`{"clientId":"%s","clientSecret":"%s","code":"%s","grantType":"authorization_code"}`,
		dingtalkConfig.ClientID, dingtalkConfig.ClientSecret, code)

	resp, err := http.Post("https://api.dingtalk.com/v1.0/oauth2/userAccessToken",
		"application/json", strings.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tokenResp dingTalkTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %s", string(body))
	}
	return &tokenResp, nil
}

func getDingTalkUserInfo(accessToken string) (*dingTalkUserInfo, error) {
	req, _ := http.NewRequest("GET", "https://api.dingtalk.com/v1.0/contact/users/me", nil)
	req.Header.Set("x-acs-dingtalk-access-token", accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var userInfo dingTalkUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse user info: %s", string(body))
	}
	return &userInfo, nil
}


// ============================================================
// WeCom (企业微信) OAuth (ported from sub2api auth_wechat_oauth.go)
// ============================================================

type WeComConfig struct {
	Enabled      bool   `json:"enabled"`
	CorpID       string `json:"corp_id"`
	AgentID      string `json:"agent_id"`
	CorpSecret   string `json:"corp_secret"`
	RedirectURI  string `json:"redirect_uri"`
}

var wecomConfig WeComConfig

func SetWeComConfig(cfg WeComConfig) {
	wecomConfig = cfg
}

func GetWeComConfig() WeComConfig {
	return wecomConfig
}

// WeComOAuthRedirect redirects to WeCom OAuth
func WeComOAuthRedirect(c *gin.Context) {
	if !wecomConfig.Enabled || wecomConfig.CorpID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "WeCom OAuth not configured"})
		return
	}

	state := fmt.Sprintf("%d", time.Now().UnixNano())
	session := sessions.Default(c)
	session.Set("oauth_state", state)
	session.Save()

	authURL := fmt.Sprintf(
		"https://login.work.weixin.qq.com/wwlogin/sso/login?login_type=CorpApp&appid=%s&agentid=%s&redirect_uri=%s&state=%s",
		wecomConfig.CorpID, wecomConfig.AgentID, wecomConfig.RedirectURI, state,
	)
	c.Redirect(http.StatusFound, authURL)
}

// WeComOAuthCallback handles WeCom OAuth callback
func WeComOAuthCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	session := sessions.Default(c)
	savedState := session.Get("oauth_state")
	if savedState == nil || savedState.(string) != state {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid state"})
		return
	}
	session.Delete("oauth_state")
	session.Save()

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "missing code"})
		return
	}

	// Get access token
	accessToken, err := getWeComAccessToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	// Get user ID from code
	userID, err := getWeComUserID(accessToken, code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	// Find or create user
	user := model.FindUserByWeComID(userID)
	if user == nil {
		user = &model.User{
			Username:    fmt.Sprintf("wecom_%s", userID[:8]),
			DisplayName: userID,
			Status:      common.UserStatusEnabled,
			Role:        common.RoleCommonUser,
		}
		if err := model.DB.Create(user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to create user"})
			return
		}
	}

	setupLogin(user, c)
}

func getWeComAccessToken() (string, error) {
	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		wecomConfig.CorpID, wecomConfig.CorpSecret)

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		AccessToken string `json:"access_token"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	json.Unmarshal(body, &result)
	if result.ErrCode != 0 {
		return "", fmt.Errorf("wecom error: %s", result.ErrMsg)
	}
	return result.AccessToken, nil
}

func getWeComUserID(accessToken, code string) (string, error) {
	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/auth/getuserinfo?access_token=%s&code=%s",
		accessToken, code)

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		UserID  string `json:"userid"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	json.Unmarshal(body, &result)
	if result.ErrCode != 0 {
		return "", fmt.Errorf("wecom error: %s", result.ErrMsg)
	}
	return result.UserID, nil
}
