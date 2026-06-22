package rtk

import (
	"fmt"
	"regexp"
	"strings"
)

// LSFilter compresses ls -la output
func LSFilter(content string) string {
	lines := strings.Split(content, "\n")
	var result []string

	// Pattern for ls -la output: drwxr-xr-x  2 user group 4096 Jan  1 12:00 dirname
	lsPattern := regexp.MustCompile(`^([dlrwx-]{10})\s+\d+\s+\S+\s+\S+\s+\d+\s+\w+\s+\d+\s+[\d:]+\s+(.+)$`)

	var dirs, files []string
	extCount := make(map[string]int)

	for _, line := range lines {
		// Skip total line
		if strings.HasPrefix(line, "total ") {
			result = append(result, line)
			continue
		}

		matches := lsPattern.FindStringSubmatch(line)
		if matches == nil {
			if strings.TrimSpace(line) != "" {
				result = append(result, line)
			}
			continue
		}

		perms := matches[1]
		name := matches[2]

		// Skip noise directories
		if isNoiseDir(name) {
			continue
		}

		if strings.HasPrefix(perms, "d") {
			dirs = append(dirs, name)
		} else {
			files = append(files, name)
			// Count extensions
			if idx := strings.LastIndex(name, "."); idx > 0 {
				ext := name[idx:]
				extCount[ext]++
			}
		}
	}

	// Output directories first
	for _, dir := range dirs {
		result = append(result, dir+"/")
	}

	// Output files
	for _, file := range files {
		result = append(result, file)
	}

	// Add extension summary if we have files
	if len(files) > 0 {
		var topExts []string
		for ext, count := range extCount {
			topExts = append(topExts, fmt.Sprintf("%d %s", count, ext))
			if len(topExts) >= LSExtSummaryTop {
				break
			}
		}
		if len(topExts) > 0 {
			result = append(result, fmt.Sprintf("Summary: %d files, %d dirs (top extensions: %s)",
				len(files), len(dirs), strings.Join(topExts, ", ")))
		}
	}

	return strings.Join(result, "\n")
}
