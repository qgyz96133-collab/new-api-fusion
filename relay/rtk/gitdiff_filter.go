package rtk

import (
	"fmt"
	"regexp"
	"strings"
)

// GitDiffFilter compresses git diff output
func GitDiffFilter(content string) string {
	lines := strings.Split(content, "\n")
	var result []string

	var currentFile string
	var inHunk bool
	var hunkLines int

	for _, line := range lines {
		// Detect new file
		if strings.HasPrefix(line, "diff --git") {
			if currentFile != "" {
				result = append(result, currentFile)
				if len(result) > MaxInputSize/100 {
					break
				}
			}
			// Extract filename
			parts := strings.Split(line, " ")
			if len(parts) >= 4 {
				currentFile = parts[3]
			}
			inHunk = false
			hunkLines = 0
			result = append(result, line)
			continue
		}

		// Detect hunk header
		if strings.HasPrefix(line, "@@") {
			inHunk = true
			hunkLines = 0
			result = append(result, line)
			continue
		}

		// Process hunk content
		if inHunk {
			hunkLines++
			if hunkLines <= GitDiffHunkMaxLines {
				result = append(result, line)
			} else if hunkLines == GitDiffHunkMaxLines+1 {
				result = append(result, fmt.Sprintf("... (%d more lines in hunk)", len(lines)))
			}
		} else {
			result = append(result, line)
		}
	}

	if currentFile != "" && (len(result) == 0 || result[len(result)-1] != currentFile) {
		result = append(result, currentFile)
	}

	return strings.Join(result, "\n")
}

// GitStatusFilter compresses git status output (comprehensive parser ported from 9router)
// Output format:
//   * <branch>
//   + Staged: N files
//      path1
//      ... +K more
//   ~ Modified: N files
//   ? Untracked: N files
//   conflicts: N files
//   clean — nothing to commit
func GitStatusFilter(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || (len(lines) == 1 && strings.TrimSpace(lines[0]) == "") {
		return "Clean working tree"
	}

	var branch string
	var stagedFiles, modifiedFiles, untrackedFiles []string
	var staged, modified, untracked, conflicts int

	// Patterns
	longMatchRe := regexp.MustCompile(`^\s*(modified|new file|deleted|renamed|both modified):\s+(.+)$`)
	porcelainRe := regexp.MustCompile(`^[ MADRCU?!][ MADRCU?!] `)

	for _, raw := range lines {
		if strings.TrimSpace(raw) == "" {
			continue
		}

		// Long-form branch detection: "On branch main"
		if strings.HasPrefix(raw, "On branch ") {
			branch = strings.TrimPrefix(raw, "On branch ")
			continue
		}

		// Porcelain branch header: "## main...origin/main"
		if strings.HasPrefix(raw, "##") {
			branch = strings.TrimPrefix(raw, "## ")
			continue
		}

		// Porcelain status (2 chars + space + path)
		if len(raw) >= 3 && porcelainRe.MatchString(raw) {
			x := raw[0]
			y := raw[1]
			file := raw[3:]

			if raw[:2] == "??" {
				untracked++
				untrackedFiles = append(untrackedFiles, file)
				continue
			}

			if strings.Contains("MADRC", string(x)) {
				staged++
				stagedFiles = append(stagedFiles, file)
			} else if x == 'U' {
				conflicts++
			}

			if y == 'M' || y == 'D' {
				modified++
				modifiedFiles = append(modifiedFiles, file)
			}
			continue
		}

		// Long form fallback: "modified:   path", "new file:   path", ...
		if matches := longMatchRe.FindStringSubmatch(raw); len(matches) == 3 {
			kind := matches[1]
			path := strings.TrimSpace(matches[2])
			if kind == "both modified" {
				conflicts++
			} else if kind == "modified" || kind == "deleted" {
				modified++
				modifiedFiles = append(modifiedFiles, path)
			} else if kind == "new file" || kind == "renamed" {
				staged++
				stagedFiles = append(stagedFiles, path)
			}
			continue
		}
	}

	var out strings.Builder
	if branch != "" {
		out.WriteString("* " + branch + "\n")
	}

	if staged > 0 {
		out.WriteString(fmt.Sprintf("+ Staged: %d files\n", staged))
		for _, f := range stagedFiles {
			if len(stagedFiles) > StatusMaxFiles && f == stagedFiles[StatusMaxFiles] {
				break
			}
			out.WriteString("   " + f + "\n")
		}
		if len(stagedFiles) > StatusMaxFiles {
			out.WriteString(fmt.Sprintf("   ... +%d more\n", len(stagedFiles)-StatusMaxFiles))
		}
	}

	if modified > 0 {
		out.WriteString(fmt.Sprintf("~ Modified: %d files\n", modified))
		limit := StatusMaxFiles
		if limit > len(modifiedFiles) {
			limit = len(modifiedFiles)
		}
		for _, f := range modifiedFiles[:limit] {
			out.WriteString("   " + f + "\n")
		}
		if len(modifiedFiles) > StatusMaxFiles {
			out.WriteString(fmt.Sprintf("   ... +%d more\n", len(modifiedFiles)-StatusMaxFiles))
		}
	}

	if untracked > 0 {
		out.WriteString(fmt.Sprintf("? Untracked: %d files\n", untracked))
		limit := StatusMaxUntracked
		if limit > len(untrackedFiles) {
			limit = len(untrackedFiles)
		}
		for _, f := range untrackedFiles[:limit] {
			out.WriteString("   " + f + "\n")
		}
		if len(untrackedFiles) > StatusMaxUntracked {
			out.WriteString(fmt.Sprintf("   ... +%d more\n", len(untrackedFiles)-StatusMaxUntracked))
		}
	}

	if conflicts > 0 {
		out.WriteString(fmt.Sprintf("conflicts: %d files\n", conflicts))
	}

	if staged == 0 && modified == 0 && untracked == 0 && conflicts == 0 {
		out.WriteString("clean — nothing to commit\n")
	}

	return strings.TrimRight(out.String(), "\n")
}
