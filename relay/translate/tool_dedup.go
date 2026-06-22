package translate

import (
	"regexp"
	"strings"
)

// DedupRule defines a tool deduplication rule
type DedupRule struct {
	Triggers []string // tool names that trigger dedup (exact match or regex prefix)
	Strips   []string // tool names to strip when trigger is present
}

// Default dedup rules (ported from 9router toolDeduper.js)
var dedupRules = []DedupRule{
	{
		// Exa MCP present → drop built-in web tools
		Triggers: []string{"mcp__exa__web_search_exa", "mcp__exa__web_fetch_exa"},
		Strips:   []string{"WebSearch", "WebFetch", "mcp__workspace__web_fetch"},
	},
	{
		// Tavily MCP present → drop built-in web tools
		Triggers: []string{"mcp__tavily__tavily_search", "mcp__tavily__tavily_extract"},
		Strips:   []string{"WebSearch", "WebFetch", "mcp__workspace__web_fetch"},
	},
	{
		// Browser MCP present → drop Claude_in_Chrome connector
		Triggers: []string{`^mcp__browsermcp__`},
		Strips:   []string{`^mcp__Claude_in_Chrome__`},
	},
}

// DedupeTools strips built-in/duplicate tools when equivalent MCP tools are present.
// Returns the filtered tools and a list of stripped tool names.
func DedupeTools(tools []interface{}) ([]interface{}, []string) {
	if len(tools) == 0 {
		return tools, nil
	}

	// Extract tool names
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = extractToolName(t)
	}

	// Find tools to strip
	toStrip := make(map[string]bool)
	for _, rule := range dedupRules {
		hasTrigger := false
		for _, name := range names {
			for _, trigger := range rule.Triggers {
				if matchPattern(name, trigger) {
					hasTrigger = true
					break
				}
			}
			if hasTrigger {
				break
			}
		}
		if !hasTrigger {
			continue
		}
		for _, name := range names {
			for _, strip := range rule.Strips {
				if matchPattern(name, strip) {
					toStrip[name] = true
				}
			}
		}
	}

	if len(toStrip) == 0 {
		return tools, nil
	}

	// Filter out stripped tools
	var result []interface{}
	var stripped []string
	for i, t := range tools {
		if toStrip[names[i]] {
			stripped = append(stripped, names[i])
		} else {
			result = append(result, t)
		}
	}

	return result, stripped
}

func extractToolName(tool interface{}) string {
	if toolMap, ok := tool.(map[string]interface{}); ok {
		// Claude format: {name: "..."}
		if name, ok := toolMap["name"].(string); ok {
			return name
		}
		// OpenAI format: {function: {name: "..."}}
		if fn, ok := toolMap["function"].(map[string]interface{}); ok {
			if name, ok := fn["name"].(string); ok {
				return name
			}
		}
	}
	return ""
}

func matchPattern(name, pattern string) bool {
	if strings.HasPrefix(pattern, "^") {
		// Regex pattern
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		return re.MatchString(name)
	}
	return name == pattern
}
