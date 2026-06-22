package rtk

import (
	"fmt"
	"path/filepath"
	"strings"
)

// FindFilter compresses find output by grouping by directory
func FindFilter(content string) string {
	lines := strings.Split(content, "\n")
	var result []string

	dirFiles := make(map[string][]string)
	var dirs []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip noise directories
		base := filepath.Base(line)
		if isNoiseDir(base) {
			continue
		}

		dir := filepath.Dir(line)
		if _, exists := dirFiles[dir]; !exists {
			dirs = append(dirs, dir)
		}

		files := dirFiles[dir]
		if len(files) < FindPerDirMax {
			dirFiles[dir] = append(files, line)
		}
	}

	// Output grouped by directory
	outputDirs := 0
	for _, dir := range dirs {
		if outputDirs >= FindTotalDirMax {
			break
		}

		files := dirFiles[dir]
		result = append(result, fmt.Sprintf("%s/ (%d files)", dir, len(files)))
		for _, file := range files {
			result = append(result, "  "+filepath.Base(file))
		}
		outputDirs++
	}

	if len(dirs) > FindTotalDirMax {
		result = append(result, fmt.Sprintf("... and %d more directories", len(dirs)-FindTotalDirMax))
	}

	return strings.Join(result, "\n")
}

func isNoiseDir(name string) bool {
	for _, noise := range NoiseDirs {
		if name == noise {
			return true
		}
	}
	return false
}
