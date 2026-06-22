package rtk

import (
	"fmt"
	"strings"
)

// SmartTruncateFilter keeps head and tail, removes middle
func SmartTruncateFilter(content string) string {
	lines := strings.Split(content, "\n")

	if len(lines) < SmartTruncateMinLines {
		return content
	}

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
