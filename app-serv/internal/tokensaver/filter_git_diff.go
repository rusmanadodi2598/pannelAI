// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/filter_git_diff.go
// @for       The git-diff filter: a unified diff compacted to file headers,
//
//	hunk starts, and a changed-line count per file.
//
// @uses      fmt, strings.
// @reason    SPEC-API-002 §5 ports the reference's git::compact_diff. A diff is
//
//	the single most expensive tool result a coding agent produces, and
//	the model needs its shape (which files, which hunks) far more than
//	it needs every context line.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import (
	"fmt"
	"strings"
)

// gitDiffFilter compacts a unified diff: 500 result lines overall, 100 kept
// lines per hunk, and one changed-line count per file.
func gitDiffFilter(diff string) string {
	result := make([]string, 0, 64)
	currentFile := ""
	added, removed := 0, 0
	inHunk := false
	hunkShown, hunkSkipped := 0, 0
	truncated := false

	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git"):
			if hunkSkipped > 0 {
				result = append(result, fmt.Sprintf("  ... (%d lines truncated)", hunkSkipped))
				truncated = true
				hunkSkipped = 0
			}
			if currentFile != "" && (added > 0 || removed > 0) {
				result = append(result, fmt.Sprintf("  +%d -%d", added, removed))
			}
			currentFile = diffFileName(line)
			result = append(result, "\n"+currentFile)
			added, removed = 0, 0
			inHunk = false
			hunkShown = 0
		case strings.HasPrefix(line, "@@"):
			if hunkSkipped > 0 {
				result = append(result, fmt.Sprintf("  ... (%d lines truncated)", hunkSkipped))
				truncated = true
				hunkSkipped = 0
			}
			inHunk = true
			hunkShown = 0
			result = append(result, "  "+line)
		case inHunk && strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			added++
			if hunkShown < gitDiffHunkMaxLines {
				result = append(result, "  "+line)
				hunkShown++
			} else {
				hunkSkipped++
			}
		case inHunk && strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			removed++
			if hunkShown < gitDiffHunkMaxLines {
				result = append(result, "  "+line)
				hunkShown++
			} else {
				hunkSkipped++
			}
		case inHunk && hunkShown < gitDiffHunkMaxLines && hunkShown > 0 && !strings.HasPrefix(line, `\`):
			result = append(result, "  "+line)
			hunkShown++
		}
		if len(result) >= gitDiffMaxLines {
			result = append(result, "\n... (more changes truncated)")
			truncated = true
			break
		}
	}

	if hunkSkipped > 0 {
		result = append(result, fmt.Sprintf("  ... (%d lines truncated)", hunkSkipped))
		truncated = true
	}
	if currentFile != "" && (added > 0 || removed > 0) {
		result = append(result, fmt.Sprintf("  +%d -%d", added, removed))
	}
	if truncated {
		result = append(result, "[full diff: rtk git diff --no-compact]")
	}
	return strings.Join(result, "\n")
}

// diffFileName reads the target path from a "diff --git a/x b/x" header,
// keeping the reference's tolerance for a path that itself contains " b/".
func diffFileName(header string) string {
	parts := strings.Split(header, " b/")
	if len(parts) > 1 {
		return strings.Join(parts[1:], " b/")
	}
	return "unknown"
}
