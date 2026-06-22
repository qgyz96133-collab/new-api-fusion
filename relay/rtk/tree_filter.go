package rtk

import (
	"fmt"
	"strings"
)

// TreeFilter compresses tree command output
func TreeFilter(content string) string {
	lines := strings.Split(content, "\n")
	var result []string

	for i, line := range lines {
		if i >= TreeMaxLines {
			result = append(result, fmt.Sprintf("... (%d more lines truncated)", len(lines)-TreeMaxLines))
			break
		}
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}
