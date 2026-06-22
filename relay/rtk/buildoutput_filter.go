package rtk

import (
	"regexp"
	"strings"
)

// BuildOutputFilter compresses build/compile output
func BuildOutputFilter(content string) string {
	lines := strings.Split(content, "\n")
	var result []string

	errorPattern := regexp.MustCompile(`(?i)(error|failed|fatal)`)
	warningPattern := regexp.MustCompile(`(?i)(warning)`)
	compilePattern := regexp.MustCompile(`(?i)(compiling|building|linking)`)

	var errors, warnings, compiles []string

	for _, line := range lines {
		if errorPattern.MatchString(line) {
			errors = append(errors, line)
		} else if warningPattern.MatchString(line) {
			if len(warnings) < BuildOutputWarningMax {
				warnings = append(warnings, line)
			}
		} else if compilePattern.MatchString(line) {
			compiles = append(compiles, line)
		} else if strings.TrimSpace(line) != "" {
			// Keep other non-empty lines
			result = append(result, line)
		}
	}

	// Reconstruct with summary
	var output []string
	output = append(output, result...)

	if len(compiles) > 0 {
		output = append(output, "Compiling:")
		for _, c := range compiles {
			output = append(output, "  "+c)
		}
	}

	if len(warnings) > 0 {
		output = append(output, "Warnings:")
		for _, w := range warnings {
			output = append(output, "  "+w)
		}
	}

	if len(errors) > 0 {
		output = append(output, "Errors:")
		for _, e := range errors {
			output = append(output, "  "+e)
		}
	}

	return strings.Join(output, "\n")
}
