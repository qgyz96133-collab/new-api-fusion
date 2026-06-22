package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// CreateUsageCleanupTask creates a new cleanup task
func CreateUsageCleanupTask(c *gin.Context) {
	var req struct {
		Filters model.UsageCleanupFilters `json:"filters"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	userId := c.GetInt("id")
	task, err := service.CreateCleanupTask(req.Filters, int64(userId))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": task})
}

// ListUsageCleanupTasks lists all cleanup tasks
func ListUsageCleanupTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("p", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	tasks, total, err := model.ListUsageCleanupTasks(page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    tasks,
		"total":   total,
	})
}

// GetUsageCleanupTask gets a single cleanup task
func GetUsageCleanupTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid id"})
		return
	}
	task, err := model.GetUsageCleanupTask(int64(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "task not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": task})
}

// CancelUsageCleanupTask cancels a pending or running task
func CancelUsageCleanupTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid id"})
		return
	}
	userId := c.GetInt("id")
	if err := model.CancelUsageCleanupTask(int64(id), int64(userId)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "task canceled"})
}
