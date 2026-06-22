package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// GetOpsAlerts returns recent operational alerts
func GetOpsAlerts(c *gin.Context) {
	limit := 50
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	alerts := model.GetRecentAlerts(limit)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": alerts})
}

// UpdateAlertThresholds updates alert evaluation thresholds
func UpdateAlertThresholds(c *gin.Context) {
	var req model.AlertThresholds
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	model.GetAlertEvaluator().SetThresholds(req)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": req})
}

// GetOpsTrends returns operational trend data
func GetOpsTrends(c *gin.Context) {
	minutes := 60
	if m := c.Query("minutes"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v > 0 {
			minutes = v
		}
	}
	trends := model.GetTrends(minutes)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": trends})
}

// Console log SSE + buffer
func GetConsoleLogs(c *gin.Context)      { GetConsoleLogBuffer(c) }
func StreamConsoleLogs(c *gin.Context)   { ConsoleLogStream(c) }

// Image concurrency status
func GetImageConcurrency(c *gin.Context) {
	// This is in middleware package
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"message": "see middleware.ImageConcurrencyLimit"}})
}

// Balance notify config
func GetBalanceNotifyConfig(c *gin.Context) {
	cfg := model.GetBalanceNotifyConfig()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": cfg})
}

func UpdateBalanceNotifyConfig(c *gin.Context) {
	var cfg model.BalanceNotifyConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	model.SetBalanceNotifyConfig(cfg)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": cfg})
}

// DingTalk / WeCom config management
func GetDingTalkConfigAdmin(c *gin.Context) {
	cfg := GetDingTalkConfig()
	cfg.ClientSecret = "" // mask secret
	c.JSON(http.StatusOK, gin.H{"success": true, "data": cfg})
}

func UpdateDingTalkConfigAdmin(c *gin.Context) {
	var cfg DingTalkConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	SetDingTalkConfig(cfg)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetWeComConfigAdmin(c *gin.Context) {
	cfg := GetWeComConfig()
	cfg.CorpSecret = "" // mask secret
	c.JSON(http.StatusOK, gin.H{"success": true, "data": cfg})
}

func UpdateWeComConfigAdmin(c *gin.Context) {
	var cfg WeComConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	SetWeComConfig(cfg)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// User attribute management
func GetUserAttributeDefs(c *gin.Context) {
	defs, err := model.GetUserAttributeDefinitions()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": []interface{}{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": defs})
}

func CreateUserAttributeDef(c *gin.Context) {
	var def model.UserAttributeDefinition
	if err := c.ShouldBindJSON(&def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	def.ID = 0
	if err := model.CreateUserAttributeDefinition(&def); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": def})
}

func DeleteUserAttributeDef(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := model.DeleteUserAttributeDefinition(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetUserAttributes(c *gin.Context) {
	userID, _ := strconv.Atoi(c.Param("id"))
	attrs, err := model.GetUserAttributes(userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": attrs})
}

func SetUserAttribute(c *gin.Context) {
	userID, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		DefinitionID int    `json:"definition_id" binding:"required"`
		Value        string `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := model.SetUserAttribute(userID, req.DefinitionID, req.Value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
