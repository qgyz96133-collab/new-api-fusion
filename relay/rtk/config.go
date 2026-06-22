package rtk

import (
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// IsEnabled checks if RTK compression is enabled (reads from operation_setting)
func IsEnabled() bool {
	return operation_setting.IsRTKEnabled()
}

// GetCavemanLevel returns the Caveman mode level (reads from operation_setting)
func GetCavemanLevel() CavemanPromptLevel {
	opLevel := operation_setting.GetCavemanModeLevel()
	switch opLevel {
	case operation_setting.CavemanModeOff:
		return CavemanOff
	case operation_setting.CavemanModeLight:
		return CavemanLite
	case operation_setting.CavemanModeFull:
		return CavemanFull
	case operation_setting.CavemanModeUltra:
		return CavemanUltra
	case operation_setting.CavemanModeWenyanLite:
		return CavemanWenyanLite
	case operation_setting.CavemanModeWenyan:
		return CavemanWenyan
	case operation_setting.CavemanModeWenyanUltra:
		return CavemanWenyanUltra
	default:
		return CavemanOff
	}
}

// ShouldValidateToolCalls delegates to operation_setting
func ShouldValidateToolCalls() bool {
	return operation_setting.ShouldValidateToolCalls()
}

// ShouldFixOrphanTools delegates to operation_setting
func ShouldFixOrphanTools() bool {
	return operation_setting.ShouldFixOrphanTools()
}

// ShouldCleanGeminiSchema delegates to operation_setting
func ShouldCleanGeminiSchema() bool {
	return operation_setting.ShouldCleanGeminiSchema()
}

// ShouldNormalizeClaude delegates to operation_setting
func ShouldNormalizeClaude() bool {
	return operation_setting.ShouldNormalizeClaude()
}

// ShouldFetchRemoteImages delegates to operation_setting
func ShouldFetchRemoteImages() bool {
	return operation_setting.ShouldFetchRemoteImages()
}
