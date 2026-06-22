package rtk

import (
	"fmt"
	"regexp"
	"strings"
)

// ReadNumberedFilter compresses numbered file content (e.g., "1|code line")
func ReadNumberedFilter(content string) string {
	lines := strings.Split(content, "\n")

	if len(lines) < ReadNumberedMinLines {
		return content
	}

	// Check if lines match numbered pattern
	numberedPattern := regexp.MustCompile(`^\s*\d+\|`)
	matchedCount := 0

	for i := 0; i < min(100, len(lines)); i++ {
		if numberedPattern.MatchString(lines[i]) {
			matchedCount++
		}
	}

	// If not enough lines match, don't compress
	if float64(matchedCount)/float64(min(100, len(lines))) < ReadNumberedMinHitRatio {
		return content
	}

	// Apply smart truncation
	var result []string

	// Keep head
	for i := 0; i < SmartTruncateHead && i < len(lines); i++ {
		result = append(result, lines[i])
	}

	// Add truncation marker
	truncated := len(lines) - SmartTruncateHead - SmartTruncateTail
	if truncated > 0 {
		result = append(result, fmt.Sprintf("... (%d lines truncated)", truncated))
	}

	// Keep tail
	tailStart := len(lines) - SmartTruncateTail
	if tailStart > SmartTruncateHead {
		for i := tailStart; i < len(lines); i++ {
			result = append(result, lines[i])
		}
	}

	return strings.Join(result, "\n")
}
