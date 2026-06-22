package rtk

import (
	"regexp"
	"strings"
)

// AutodetectFilter examines content and returns the appropriate filter
func AutodetectFilter(content string) FilterFunc {
	if len(content) < DetectWindow {
		return nil
	}

	window := content
	if len(window) > DetectWindow {
		window = window[:DetectWindow]
	}

	// Git diff detection
	if strings.Contains(window, "diff --git") || strings.Contains(window, "@@ ") {
		return GitDiffFilter
	}

	// Git status detection (porcelain format)
	if matched, _ := regexp.MatchString(`(?m)^[ MADRCU?!][ MADRCU?!] `, window); matched {
		return GitStatusFilter
	}

	// Grep detection
	if matched, _ := regexp.MatchString(`(?m)^[^:]+:\d+:`, window); matched {
		return GrepFilter
	}

	// Find detection (paths with /)
	lines := strings.Split(window, "\n")
	pathCount := 0
	for _, line := range lines[:min(10, len(lines))] {
		if strings.Contains(line, "/") && !strings.Contains(line, ":") {
			pathCount++
		}
	}
	if pathCount > 5 {
		return FindFilter
	}

	// LS detection
	if matched, _ := regexp.MatchString(`(?m)^d[rwx-]{9}`, window); matched {
		return LSFilter
	}

	// Tree detection
	if strings.Contains(window, "├──") || strings.Contains(window, "└──") {
		return TreeFilter
	}

	// Build output detection
	buildPatterns := []string{
		"error:", "ERROR:", "failed", "FAILED",
		"warning:", "WARNING:",
		"Compiling", "Building", "Linking",
	}
	buildCount := 0
	for _, pattern := range buildPatterns {
		if strings.Contains(window, pattern) {
			buildCount++
		}
	}
	if buildCount >= 2 {
		return BuildOutputFilter
	}

	// Large output detection
	if len(content) > 5000 {
		// Check for dedup log patterns
		lineMap := make(map[string]int)
		for _, line := range lines[:min(100, len(lines))] {
			lineMap[line]++
		}
		dupCount := 0
		for _, count := range lineMap {
			if count > 3 {
				dupCount++
			}
		}
		if dupCount > 5 {
			return DedupLogFilter
		}

		// Check for search list patterns
		if matched, _ := regexp.MatchString(`(?m)^\s*\d+\.\s`, window); matched {
			return SearchListFilter
		}

		// Check for read numbered patterns
		if matched, _ := regexp.MatchString(`(?m)^\d+\|`, window); matched {
			return ReadNumberedFilter
		}

		// Default to smart truncate for large outputs
		return SmartTruncateFilter
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// AutodetectFilterNamed returns the filter function and its name
func AutodetectFilterNamed(content string) (func(string) string, string) {
	fn := AutodetectFilter(content)
	if fn == nil {
		return nil, ""
	}

	// Test against known filters to determine name
	testInputs := map[string]string{
		"git-diff":       "diff --git a/test b/test\n@@ -1,3 +1,3 @@",
		"git-status":     "On branch main\n M test.go",
		"grep":           "file.go:10:some match",
		"find":           "dir1/file1\ndir2/file2\ndir3/file3",
		"tree":           "├── dir1\n└── dir2",
		"build-output":   "error: failed to compile\nwarning: unused variable",
		"smart-truncate": strings.Repeat("line\n", 300),
	}

	for name, testInput := range testInputs {
		if knownFn := GetFilter(name); knownFn != nil {
			// Compare results on the same input
			if knownFn(testInput) == fn(testInput) {
				return fn, name
			}
		}
	}

	return fn, "unknown"
}
