/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

// GetRTKSettings 获取 RTK 配置
func GetRTKSettings(c *gin.Context) {
	rtkSetting := operation_setting.GetRTKSetting()

	response := gin.H{
		"rtk_enabled":            rtkSetting.RTKEnabled,
		"rtk_compression_level":  rtkSetting.RTKCompressionLevel,
		"rtk_min_tokens":         rtkSetting.RTKMinTokens,
		"rtk_max_tokens":         rtkSetting.RTKMaxTokens,
		"caveman_enabled":        rtkSetting.CavemanEnabled,
		"caveman_mode_level":     rtkSetting.CavemanModeLevel,
		"caveman_min_tokens":     rtkSetting.CavemanMinTokens,
		"enable_tool_call_validation":    rtkSetting.EnableToolCallValidation,
		"enable_orphan_tool_fix":         rtkSetting.EnableOrphanToolFix,
		"enable_gemini_schema_cleaning":  rtkSetting.EnableGeminiSchemaCleaning,
		"enable_claude_normalization":    rtkSetting.EnableClaudeNormalization,
		"enable_remote_image_fetch":      rtkSetting.EnableRemoteImageFetch,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "获取 RTK 配置成功",
		"data":    response,
	})
}

// UpdateRTKSettings 更新 RTK 配置
func UpdateRTKSettings(c *gin.Context) {
	var req struct {
		RTKEnabled          *bool `json:"rtk_enabled"`
		RTKCompressionLevel *int  `json:"rtk_compression_level"`
		RTKMinTokens        *int  `json:"rtk_min_tokens"`
		RTKMaxTokens        *int  `json:"rtk_max_tokens"`
		CavemanEnabled      *bool `json:"caveman_enabled"`
		CavemanModeLevel    *int  `json:"caveman_mode_level"`
		CavemanMinTokens    *int  `json:"caveman_min_tokens"`
		EnableToolCallValidation   *bool `json:"enable_tool_call_validation"`
		EnableOrphanToolFix        *bool `json:"enable_orphan_tool_fix"`
		EnableGeminiSchemaCleaning *bool `json:"enable_gemini_schema_cleaning"`
		EnableClaudeNormalization  *bool `json:"enable_claude_normalization"`
		EnableRemoteImageFetch     *bool `json:"enable_remote_image_fetch"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	// 获取当前配置
	rtkSetting := operation_setting.GetRTKSetting()

	// 更新配置（仅更新提供的字段）
	if req.RTKEnabled != nil {
		rtkSetting.RTKEnabled = *req.RTKEnabled
	}
	if req.RTKCompressionLevel != nil {
		if *req.RTKCompressionLevel >= 0 && *req.RTKCompressionLevel <= 6 {
			rtkSetting.RTKCompressionLevel = operation_setting.RTKCompressionLevel(*req.RTKCompressionLevel)
		}
	}
	if req.RTKMinTokens != nil {
		if *req.RTKMinTokens >= 0 {
			rtkSetting.RTKMinTokens = *req.RTKMinTokens
		}
	}
	if req.RTKMaxTokens != nil {
		if *req.RTKMaxTokens > 0 {
			rtkSetting.RTKMaxTokens = *req.RTKMaxTokens
		}
	}
	if req.CavemanEnabled != nil {
		rtkSetting.CavemanEnabled = *req.CavemanEnabled
	}
	if req.CavemanModeLevel != nil {
		if *req.CavemanModeLevel >= 0 && *req.CavemanModeLevel <= 6 {
			rtkSetting.CavemanModeLevel = operation_setting.CavemanModeLevel(*req.CavemanModeLevel)
		}
	}
	if req.CavemanMinTokens != nil {
		if *req.CavemanMinTokens >= 0 {
			rtkSetting.CavemanMinTokens = *req.CavemanMinTokens
		}
	}
	if req.EnableToolCallValidation != nil {
		rtkSetting.EnableToolCallValidation = *req.EnableToolCallValidation
	}
	if req.EnableOrphanToolFix != nil {
		rtkSetting.EnableOrphanToolFix = *req.EnableOrphanToolFix
	}
	if req.EnableGeminiSchemaCleaning != nil {
		rtkSetting.EnableGeminiSchemaCleaning = *req.EnableGeminiSchemaCleaning
	}
	if req.EnableClaudeNormalization != nil {
		rtkSetting.EnableClaudeNormalization = *req.EnableClaudeNormalization
	}
	if req.EnableRemoteImageFetch != nil {
		rtkSetting.EnableRemoteImageFetch = *req.EnableRemoteImageFetch
	}

	// 保存配置到内存
	operation_setting.UpdateRTKSetting(*rtkSetting)

	// 保存配置到数据库
	if err := model.UpdateOptionsBulk(operation_setting.GetRTKSettingForDB()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "保存配置到数据库失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "RTK 配置更新成功",
	})
}

// ResetRTKSettings 重置 RTK 配置为默认值
func ResetRTKSettings(c *gin.Context) {
	// 重置为默认配置
	defaultSetting := operation_setting.RTKSetting{
		RTKEnabled:          true,
		RTKCompressionLevel: operation_setting.RTKLevelModerate,
		RTKMinTokens:        100,
		RTKMaxTokens:        50000,
		CavemanEnabled:      false,
		CavemanModeLevel:    operation_setting.CavemanModeFull,
		CavemanMinTokens:    200,
		EnableToolCallValidation:   true,
		EnableOrphanToolFix:        true,
		EnableGeminiSchemaCleaning: true,
		EnableClaudeNormalization:  true,
		EnableRemoteImageFetch:     true,
	}

	operation_setting.UpdateRTKSetting(defaultSetting)

	// 保存配置到数据库
	if err := model.UpdateOptionsBulk(operation_setting.GetRTKSettingForDB()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "保存配置到数据库失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "RTK 配置已重置为默认值",
	})
}

// GetRTKCompressionStats 获取 RTK 压缩统计信息（占位符）
func GetRTKCompressionStats(c *gin.Context) {
	// TODO: 实现压缩统计功能
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "获取压缩统计成功",
		"data": gin.H{
			"total_requests":      0,
			"compressed_requests": 0,
			"total_tokens_saved":  0,
			"compression_ratio":   "0%",
		},
	})
}
