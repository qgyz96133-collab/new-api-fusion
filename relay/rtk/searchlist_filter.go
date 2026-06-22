package rtk

import (
	"fmt"
	"regexp"
	"strings"
)

// SearchListFilter compresses search result lists
func SearchListFilter(content string) string {
	lines := strings.Split(content, "\n")
	var result []string

	// Pattern for numbered search results: "  123. path/to/file"
	searchPattern := regexp.MustCompile(`^\s*(\d+)\.\s+(.+)$`)

	var currentDir string
	var dirResults []string
	var totalResults int

	for _, line := range lines {
		matches := searchPattern.FindStringSubmatch(line)
		if matches == nil {
			result = append(result, line)
			continue
		}

		totalResults++
		path := matches[2]

		// Extract directory
		lastSlash := strings.LastIndex(path, "/")
		var dir string
		if lastSlash >= 0 {
			dir = path[:lastSlash]
		} else {
			dir = "."
		}

		// Check if we're in a new directory
		if dir != currentDir {
			// Output previous directory results
			if currentDir != "" {
				result = append(result, fmt.Sprintf("%s/ (%d results)", currentDir, len(dirResults)))
				shown := 0
				for _, r := range dirResults {
					if shown < SearchListPerDirMax {
						result = append(result, "  "+r)
						shown++
					}
				}
				if len(dirResults) > SearchListPerDirMax {
					result = append(result, fmt.Sprintf("  ... and %d more", len(dirResults)-SearchListPerDirMax))
				}
			}

			currentDir = dir
			dirResults = []string{}
		}

		// Collect result
		filename := path
		if lastSlash >= 0 {
			filename = path[lastSlash+1:]
		}
		dirResults = append(dirResults, filename)
	}

	// Output final directory
	if currentDir != "" && len(dirResults) > 0 {
		result = append(result, fmt.Sprintf("%s/ (%d results)", currentDir, len(dirResults)))
		shown := 0
		for _, r := range dirResults {
			if shown < SearchListPerDirMax {
				result = append(result, "  "+r)
				shown++
			}
		}
		if len(dirResults) > SearchListPerDirMax {
			result = append(result, fmt.Sprintf("  ... and %d more", len(dirResults)-SearchListPerDirMax))
		}
	}

	// Add total summary
	if totalResults > 0 {
		result = append(result, fmt.Sprintf("Total: %d search results", totalResults))
	}

	return strings.Join(result, "\n")
}
