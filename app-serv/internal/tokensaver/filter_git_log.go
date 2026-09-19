// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/filter_git_log.go
// @for       The git-log filter: commit headers, subjects, authors, and stats,
//
//	with the bodies and embedded diffs dropped.
//
// @uses      fmt, regexp, strings.
// @reason    SPEC-API-002 §5 ports the reference's git-log filter. A log is
//
//	read for "what happened", which the headers and subjects carry; the
//	per-commit bodies are what makes the result expensive.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reLogCommitHeader  = regexp.MustCompile(`^[*|/\\ ]*commit [0-9a-f]{7,40}$`)
	reLogCommitPrefix  = regexp.MustCompile(`^[*|/\\ ]+commit [0-9a-f]{7,40}`)
	reLogAuthorDate    = regexp.MustCompile(`^[*|/\\ ]*(Author|Date):`)
	reLogIndentSubject = regexp.MustCompile(`^[*|/\\ ]*    \S`)
	reLogStatSummary   = regexp.MustCompile(`^\d+ file\w* changed`)
	reLogGraphLine     = regexp.MustCompile(`^[*|/\\ ]+([0-9a-f]{7,40}\s+.+)`)
	reLogOneline       = regexp.MustCompile(`^[0-9a-f]{7,40}\s+`)
	reLogDecoration    = regexp.MustCompile(`^[*|/\\ ]+$`)
)

// gitLogFilter compacts a `git log` dump, in the long form, the --oneline form,
// or the --graph form. It never returns something longer than its input.
func gitLogFilter(text string) string {
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	skipped := 0
	inCommit, subjectSeen := false, false

	push := func(line string) {
		if len(out) < gitLogMaxLines {
			out = append(out, line)
			return
		}
		skipped++
	}

	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t\r\v\f\u00a0")
		trimmed := strings.TrimSpace(line)

		if reLogCommitHeader.MatchString(trimmed) || reLogCommitPrefix.MatchString(trimmed) {
			inCommit = true
			subjectSeen = false
			push(line)
			continue
		}

		if inCommit {
			switch {
			case reLogAuthorDate.MatchString(trimmed):
				push(trimmed)
			case trimmed == "":
			case !subjectSeen && reLogIndentSubject.MatchString(line):
				push("  Subject: " + trimmed)
				subjectSeen = true
			case reLogStatSummary.MatchString(trimmed):
				push("  " + trimmed)
			case strings.HasPrefix(trimmed, "diff --git "):
				push("  ... diff body omitted")
			}
			continue
		}

		switch {
		case reLogGraphLine.MatchString(trimmed):
			push(reLogGraphLine.FindStringSubmatch(trimmed)[1])
		case reLogOneline.MatchString(trimmed):
			push(trimmed)
		case reLogDecoration.MatchString(trimmed) && strings.ContainsAny(trimmed, `*|/\`):
		default:
			push(trimmed)
		}
	}

	if skipped > 0 {
		out = append(out, fmt.Sprintf("... (%d more lines)", skipped))
	}
	result := strings.Join(out, "\n")
	if result == "" || len(result) > len(text) {
		return text
	}
	return result
}
