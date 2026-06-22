package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// GetSubscriptionQuotas returns all quotas for a user
func GetSubscriptionQuotas(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid user id"})
		return
	}

	quotas, err := model.GetSubscriptionQuotasByUser(userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": []interface{}{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": quotas})
}

// UpdateSubscriptionQuota updates a quota record
func UpdateSubscriptionQuota(c *gin.Context) {
	var req struct {
		TokenID      int   `json:"token_id" binding:"required"`
		UserID       int   `json:"user_id" binding:"required"`
		DailyLimit   int64 `json:"daily_limit"`
		WeeklyLimit  int64 `json:"weekly_limit"`
		MonthlyLimit int64 `json:"monthly_limit"`
		RPMLimit     int   `json:"rpm_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	quota, err := model.GetSubscriptionQuota(req.TokenID, req.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	quota.DailyLimit = req.DailyLimit
	quota.WeeklyLimit = req.WeeklyLimit
	quota.MonthlyLimit = req.MonthlyLimit
	quota.RPMLimit = req.RPMLimit

	if err := model.SetSubscriptionQuota(quota); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": quota})
}

// GetMyQuotaStatus returns the current user's quota usage
func GetMyQuotaStatus(c *gin.Context) {
	tokenID, _ := c.Get("token_id")
	userID, _ := c.Get("id")

	tid, _ := tokenID.(int)
	uid, _ := userID.(int)

	if tid == 0 || uid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "unauthorized"})
		return
	}

	quota, err := model.GetSubscriptionQuota(tid, uid)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"message": "no quota configured"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"daily":   gin.H{"limit": quota.DailyLimit, "used": quota.DailyUsed, "remaining": maxI64(quota.DailyLimit - quota.DailyUsed)},
			"weekly":  gin.H{"limit": quota.WeeklyLimit, "used": quota.WeeklyUsed, "remaining": maxI64(quota.WeeklyLimit - quota.WeeklyUsed)},
			"monthly": gin.H{"limit": quota.MonthlyLimit, "used": quota.MonthlyUsed, "remaining": maxI64(quota.MonthlyLimit - quota.MonthlyUsed)},
			"rpm":     gin.H{"limit": quota.RPMLimit},
		},
	})
}

func maxI64(v int64) int64 {
	if v < 0 { return 0 }
	return v
}

func max0(a, b int64) int64 {
	if a > b { return a }
	return b
}
