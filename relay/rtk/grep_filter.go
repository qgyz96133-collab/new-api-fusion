package rtk

import (
	"fmt"
	"regexp"
	"strings"
)

// GrepFilter compresses grep output by limiting matches per file
func GrepFilter(content string) string {
	lines := strings.Split(content, "\n")
	var result []string

	fileMatches := make(map[string]int)
	var currentFile string

	grepPattern := regexp.MustCompile(`^([^:]+):(\d+):(.*)$`)

	for _, line := range lines {
		matches := grepPattern.FindStringSubmatch(line)
		if matches == nil {
			result = append(result, line)
			continue
		}

		file := matches[1]
		if file != currentFile {
			currentFile = file
			fileMatches[file] = 0
		}

		fileMatches[file]++
		if fileMatches[file] <= GrepPerFileMax {
			result = append(result, line)
		}
	}

	// Add summary for truncated files
	for file, count := range fileMatches {
		if count > GrepPerFileMax {
			result = append(result, fmt.Sprintf("... and %d more matches in %s", count-GrepPerFileMax, file))
		}
	}

	return strings.Join(result, "\n")
}
