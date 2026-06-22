package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// GetModelCapabilities returns capabilities for a specific model
func GetModelCapabilities(c *gin.Context) {
	modelID := c.Query("model")
	if modelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "model query parameter required"})
		return
	}
	caps := service.GetModelCapabilities(modelID)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": caps})
}

// ListModelCapabilities returns capabilities for multiple models
func ListModelCapabilities(c *gin.Context) {
	var req struct {
		Models []string `json:"models"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	result := make(map[string]service.ModelCapabilities)
	for _, model := range req.Models {
		result[model] = service.GetModelCapabilities(model)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// ReorderModelsByCapabilities reorders models based on required capabilities
func ReorderModelsByCapabilities(c *gin.Context) {
	var req struct {
		Models       []string        `json:"models"`
		Requirements map[string]bool `json:"requirements"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	reordered := service.ReorderModelsByCapabilities(req.Models, req.Requirements)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": reordered})
}
