package rtk

import (
	"fmt"
	"strings"
)

// DedupLogFilter removes duplicate consecutive log lines
func DedupLogFilter(content string) string {
	lines := strings.Split(content, "\n")
	var result []string

	lineCount := make(map[string]int)
	var lastLine string

	for _, line := range lines {
		if len(result) >= DedupLogMax {
			result = append(result, fmt.Sprintf("... (%d more lines truncated)", len(lines)-DedupLogMax))
			break
		}

		if line == lastLine {
			lineCount[line]++
		} else {
			// Output previous line with count if duplicated
			if count, exists := lineCount[lastLine]; exists && count > 1 {
				result = append(result, fmt.Sprintf("... (%d duplicate lines)", count))
			}
			result = append(result, line)
			lastLine = line
			lineCount[line] = 1
		}
	}

	// Handle final line
	if count, exists := lineCount[lastLine]; exists && count > 1 {
		result = append(result, fmt.Sprintf("... (%d duplicate lines)", count))
	}

	return strings.Join(result, "\n")
}
