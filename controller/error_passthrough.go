package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// ListErrorPassthroughRules lists all error passthrough rules
func ListErrorPassthroughRules(c *gin.Context) {
	rules, err := model.GetAllErrorPassthroughRules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rules})
}

// GetErrorPassthroughRule gets a single rule
func GetErrorPassthroughRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid id"})
		return
	}
	rule, err := model.GetErrorPassthroughRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "rule not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rule})
}

// CreateErrorPassthroughRule creates a new rule
func CreateErrorPassthroughRule(c *gin.Context) {
	var rule model.ErrorPassthroughRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := model.CreateErrorPassthroughRule(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rule})
}

// UpdateErrorPassthroughRule updates an existing rule
func UpdateErrorPassthroughRule(c *gin.Context) {
	var rule model.ErrorPassthroughRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if rule.Id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "id is required"})
		return
	}
	if err := model.UpdateErrorPassthroughRule(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rule})
}

// DeleteErrorPassthroughRule deletes a rule
func DeleteErrorPassthroughRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid id"})
		return
	}
	if err := model.DeleteErrorPassthroughRule(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "deleted"})
}
