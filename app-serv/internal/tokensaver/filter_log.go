// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/filter_log.go
// @for       The generic log filters: dedup-log, smart-truncate, and
//
//	read-numbered.
//
// @uses      fmt, regexp, strings.
// @reason    SPEC-API-002 §5 ports the reference's three fallbacks. They are
//
//	what catches the tool results no specific filter claims: repeated
//	log noise, an oversized blob, and a line-numbered file dump.
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

// reReadNumberedLine matches the "  N|content" shape of a Cursor read_file.
var reReadNumberedLine = regexp.MustCompile(`^\s*\d+\|`)

// dedupLogFilter collapses consecutive duplicate lines, caps blank runs, and
// truncates the result at the reference's line cap.
func dedupLogFilter(input string) string {
	out := make([]string, 0, 64)
	prev := ""
	runCount, blankStreak := 0, 0

	flushRun := func() {
		if runCount > 1 {
			out = append(out, fmt.Sprintf("  ... (%d duplicate lines)", runCount-1))
		}
	}

	for _, line := range strings.Split(input, "\n") {
		if strings.TrimSpace(line) == "" {
			if blankStreak < 1 {
				out = append(out, line)
			}
			blankStreak++
			flushRun()
			prev, runCount = "", 0
			continue
		}
		blankStreak = 0
		if line == prev {
			runCount++
			continue
		}
		flushRun()
		out = append(out, line)
		prev, runCount = line, 1
		if len(out) >= dedupLogLineMax {
			out = append(out, fmt.Sprintf("... (truncated at %d lines)", dedupLogLineMax))
			return strings.Join(out, "\n")
		}
	}
	flushRun()
	return strings.Join(out, "\n")
}

// smartTruncateFilter keeps the head and the tail of an oversized blob and
// replaces the middle with a count.
func smartTruncateFilter(input string) string {
	return truncateMiddle(input, func(cut int) string { return fmt.Sprintf("... +%d lines truncated", cut) })
}

// readNumberedFilter truncates a line-numbered file dump the same way, naming
// what was dropped so the model knows the file continues.
func readNumberedFilter(input string) string {
	return truncateMiddle(input, func(cut int) string { return fmt.Sprintf("... +%d lines truncated (file continues)", cut) })
}

// truncateMiddle keeps the configured head and tail line counts and replaces
// the middle with the marker for the dropped line count.
func truncateMiddle(input string, marker func(int) string) string {
	lines := strings.Split(input, "\n")
	if len(lines) < smartTruncateMinLines {
		return input
	}
	head := lines[:smartTruncateHead]
	tail := lines[len(lines)-smartTruncateTail:]
	cut := len(lines) - len(head) - len(tail)

	result := make([]string, 0, len(head)+len(tail)+1)
	result = append(result, head...)
	result = append(result, marker(cut))
	result = append(result, tail...)
	return strings.Join(result, "\n")
}
