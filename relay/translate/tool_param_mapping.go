package translate

import (
	"encoding/json"
)

// ClaudeToolParamMapping maps OpenAI tool parameter names to Claude-specific names.
// This fixes critical issues where Claude Code CLI expects different parameter names.
// Source: AIClient2API provider analysis.
var ClaudeToolParamMapping = map[string]map[string]string{
	"Grep":  {"paths": "path", "description": "pattern"},
	"Glob":  {"paths": "path", "description": "pattern"},
	"Read":  {"paths": "path"},
	"Edit":  {"paths": "path"},
	"Write": {"paths": "path"},
}

// FixClaudeToolParameters applies Claude-specific parameter mapping fixes.
// This is critical for Claude Code CLI compatibility — it rewrites tool
// parameter names (e.g. paths→path) so that Claude receives the schema it
// expects.  Called from ApplyAllFixes in rtk_integration.go when the
// request format is "claude".
func FixClaudeToolParameters(tools []interface{}) []interface{} {
	if tools == nil {
		return tools
	}

	fixedTools := make([]interface{}, 0, len(tools))

	for _, tool := range tools {
		toolMap, ok := tool.(map[string]interface{})
		if !ok {
			fixedTools = append(fixedTools, tool)
			continue
		}

		toolType, _ := toolMap["type"].(string)
		if toolType != "function" {
			fixedTools = append(fixedTools, tool)
			continue
		}

		function, ok := toolMap["function"].(map[string]interface{})
		if !ok {
			fixedTools = append(fixedTools, tool)
			continue
		}

		toolName, _ := function["name"].(string)
		if toolName == "" {
			fixedTools = append(fixedTools, tool)
			continue
		}

		if mapping, exists := ClaudeToolParamMapping[toolName]; exists {
			function = applyParameterMapping(function, mapping)
			toolMap["function"] = function
		}

		fixedTools = append(fixedTools, toolMap)
	}

	return fixedTools
}

// applyParameterMapping rewrites property names and required-field entries
// according to the provided mapping table.
func applyParameterMapping(function map[string]interface{}, mapping map[string]string) map[string]interface{} {
	result := deepCopyMap(function)

	params, ok := result["parameters"].(map[string]interface{})
	if !ok {
		return result
	}

	properties, ok := params["properties"].(map[string]interface{})
	if !ok {
		return result
	}

	for oldName, newName := range mapping {
		if prop, exists := properties[oldName]; exists {
			properties[newName] = prop
			delete(properties, oldName)
		}
	}

	if required, ok := params["required"].([]interface{}); ok {
		newRequired := make([]interface{}, 0, len(required))
		for _, req := range required {
			reqStr, ok := req.(string)
			if !ok {
				newRequired = append(newRequired, req)
				continue
			}
			if newName, exists := mapping[reqStr]; exists {
				newRequired = append(newRequired, newName)
			} else {
				newRequired = append(newRequired, reqStr)
			}
		}
		params["required"] = newRequired
	}

	result["parameters"] = params
	return result
}

// deepCopyMap creates a deep copy of a map via JSON round-trip.
func deepCopyMap(original map[string]interface{}) map[string]interface{} {
	data, _ := json.Marshal(original)
	var result map[string]interface{}
	_ = json.Unmarshal(data, &result)
	return result
}
