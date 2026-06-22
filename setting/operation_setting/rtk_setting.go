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
package operation_setting

import (
	"fmt"

	"github.com/QuantumNous/new-api/setting/config"
)

// RTKCompressionLevel RTK 压缩等级 (0-6)
type RTKCompressionLevel int

const (
	RTKLevelOff      RTKCompressionLevel = 0 // 关闭
	RTKLevelLight    RTKCompressionLevel = 1 // 轻量 (10-20%)
	RTKLevelModerate RTKCompressionLevel = 2 // 中等 (20-40%)
	RTKLevelStrong   RTKCompressionLevel = 3 // 强力 (40-60%)
	RTKLevelAggressive RTKCompressionLevel = 4 // 激进 (60-80%)
	RTKLevelExtreme  RTKCompressionLevel = 5 // 极限 (80-90%)
	RTKLevelMaximum  RTKCompressionLevel = 6 // 最大 (90%+)
)

// CavemanModeLevel Caveman 模式等级 (0-6)
type CavemanModeLevel int

const (
	CavemanModeOff        CavemanModeLevel = 0 // 关闭
	CavemanModeLight      CavemanModeLevel = 1 // 轻量
	CavemanModeFull       CavemanModeLevel = 2 // 完整
	CavemanModeUltra      CavemanModeLevel = 3 // 极限
	CavemanModeWenyanLite CavemanModeLevel = 4 // 文言轻量
	CavemanModeWenyan     CavemanModeLevel = 5 // 文言完整
	CavemanModeWenyanUltra CavemanModeLevel = 6 // 文言极限
)

// RTKSetting RTK 和 Caveman 配置
type RTKSetting struct {
	// RTK 配置
	RTKEnabled         bool                `json:"rtk_enabled"`          // 是否启用 RTK 压缩
	RTKCompressionLevel RTKCompressionLevel `json:"rtk_compression_level"` // RTK 压缩等级 (0-6)
	RTKMinTokens       int                 `json:"rtk_min_tokens"`       // 最小触发压缩的 token 数
	RTKMaxTokens       int                 `json:"rtk_max_tokens"`       // 单次请求最大 token 数

	// Caveman 配置
	CavemanEnabled     bool               `json:"caveman_enabled"`      // 是否启用 Caveman 模式
	CavemanModeLevel   CavemanModeLevel   `json:"caveman_mode_level"`   // Caveman 模式等级 (0-6)
	CavemanMinTokens   int                `json:"caveman_min_tokens"`   // 最小触发 Caveman 的 token 数

	// 高级配置
	EnableToolCallValidation   bool `json:"enable_tool_call_validation"`    // 启用 tool call ID 验证
	EnableOrphanToolFix        bool `json:"enable_orphan_tool_fix"`         // 启用孤立 tool 修复
	EnableGeminiSchemaCleaning bool `json:"enable_gemini_schema_cleaning"`  // 启用 Gemini schema 清理
	EnableClaudeNormalization  bool `json:"enable_claude_normalization"`    // 启用 Claude 规范化
	EnableRemoteImageFetch     bool `json:"enable_remote_image_fetch"`      // 启用远程图片获取
}

var rtkSetting RTKSetting

// 默认配置
var defaultRTKSetting = RTKSetting{
	// RTK 默认配置
	RTKEnabled:          true,
	RTKCompressionLevel: RTKLevelModerate, // 中等压缩 (20-40%)
	RTKMinTokens:        100,
	RTKMaxTokens:        50000,

	// Caveman 默认配置
	CavemanEnabled:   false, // 默认关闭
	CavemanModeLevel: CavemanModeFull,
	CavemanMinTokens: 200,

	// 高级配置默认全部启用
	EnableToolCallValidation:   true,
	EnableOrphanToolFix:        true,
	EnableGeminiSchemaCleaning: true,
	EnableClaudeNormalization:  true,
	EnableRemoteImageFetch:     true,
}

func init() {
	// 初始化默认配置
	rtkSetting = defaultRTKSetting
	// 注册到全局配置管理器
	config.GlobalConfig.Register("rtk_setting", &rtkSetting)
}

// GetRTKSetting 获取 RTK 配置
func GetRTKSetting() *RTKSetting {
	return &rtkSetting
}

// IsRTKEnabled 是否启用 RTK 压缩
func IsRTKEnabled() bool {
	return rtkSetting.RTKEnabled
}

// IsCavemanEnabled 是否启用 Caveman 模式
func IsCavemanEnabled() bool {
	return rtkSetting.CavemanEnabled
}

// GetRTKCompressionLevel 获取 RTK 压缩等级
func GetRTKCompressionLevel() RTKCompressionLevel {
	return rtkSetting.RTKCompressionLevel
}

// GetCavemanModeLevel 获取 Caveman 模式等级
func GetCavemanModeLevel() CavemanModeLevel {
	return rtkSetting.CavemanModeLevel
}

// ShouldValidateToolCalls 是否验证 tool calls
func ShouldValidateToolCalls() bool {
	return rtkSetting.EnableToolCallValidation
}

// ShouldFixOrphanTools 是否修复孤立 tools
func ShouldFixOrphanTools() bool {
	return rtkSetting.EnableOrphanToolFix
}

// ShouldCleanGeminiSchema 是否清理 Gemini schema
func ShouldCleanGeminiSchema() bool {
	return rtkSetting.EnableGeminiSchemaCleaning
}

// ShouldNormalizeClaude 是否规范化 Claude 请求
func ShouldNormalizeClaude() bool {
	return rtkSetting.EnableClaudeNormalization
}

// ShouldFetchRemoteImages 是否获取远程图片
func ShouldFetchRemoteImages() bool {
	return rtkSetting.EnableRemoteImageFetch
}

// GetRTKMinTokens 获取 RTK 最小 token 数
func GetRTKMinTokens() int {
	return rtkSetting.RTKMinTokens
}

// GetRTKMaxTokens 获取 RTK 最大 token 数
func GetRTKMaxTokens() int {
	return rtkSetting.RTKMaxTokens
}

// GetCavemanMinTokens 获取 Caveman 最小 token 数
func GetCavemanMinTokens() int {
	return rtkSetting.CavemanMinTokens
}

// UpdateRTKSetting 更新 RTK 配置（仅更新内存，需要外部调用 SaveRTKSettingToDB 保存到数据库）
func UpdateRTKSetting(newSetting RTKSetting) {
	rtkSetting = newSetting
}

// GetRTKSettingForDB 获取用于保存到数据库的配置选项
func GetRTKSettingForDB() map[string]string {
	return map[string]string{
		"rtk_setting.rtk_enabled":                   fmt.Sprintf("%t", rtkSetting.RTKEnabled),
		"rtk_setting.rtk_compression_level":         fmt.Sprintf("%d", rtkSetting.RTKCompressionLevel),
		"rtk_setting.rtk_min_tokens":                fmt.Sprintf("%d", rtkSetting.RTKMinTokens),
		"rtk_setting.rtk_max_tokens":                fmt.Sprintf("%d", rtkSetting.RTKMaxTokens),
		"rtk_setting.caveman_enabled":               fmt.Sprintf("%t", rtkSetting.CavemanEnabled),
		"rtk_setting.caveman_mode_level":            fmt.Sprintf("%d", rtkSetting.CavemanModeLevel),
		"rtk_setting.caveman_min_tokens":            fmt.Sprintf("%d", rtkSetting.CavemanMinTokens),
		"rtk_setting.enable_tool_call_validation":   fmt.Sprintf("%t", rtkSetting.EnableToolCallValidation),
		"rtk_setting.enable_orphan_tool_fix":        fmt.Sprintf("%t", rtkSetting.EnableOrphanToolFix),
		"rtk_setting.enable_gemini_schema_cleaning": fmt.Sprintf("%t", rtkSetting.EnableGeminiSchemaCleaning),
		"rtk_setting.enable_claude_normalization":   fmt.Sprintf("%t", rtkSetting.EnableClaudeNormalization),
		"rtk_setting.enable_remote_image_fetch":     fmt.Sprintf("%t", rtkSetting.EnableRemoteImageFetch),
	}
}

// RTKCompressionLevelToString 将压缩等级转换为可读字符串
func RTKCompressionLevelToString(level RTKCompressionLevel) string {
	switch level {
	case RTKLevelOff:
		return "关闭"
	case RTKLevelLight:
		return "轻量 (10-20%)"
	case RTKLevelModerate:
		return "中等 (20-40%)"
	case RTKLevelStrong:
		return "强力 (40-60%)"
	case RTKLevelAggressive:
		return "激进 (60-80%)"
	case RTKLevelExtreme:
		return "极限 (80-90%)"
	case RTKLevelMaximum:
		return "最大 (90%+)"
	default:
		return "未知"
	}
}

// CavemanModeLevelToString 将 Caveman 模式等级转换为可读字符串
func CavemanModeLevelToString(level CavemanModeLevel) string {
	switch level {
	case CavemanModeOff:
		return "关闭"
	case CavemanModeLight:
		return "轻量"
	case CavemanModeFull:
		return "完整"
	case CavemanModeUltra:
		return "极限"
	default:
		return "未知"
	}
}
