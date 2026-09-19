// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/autodetect_rules.go
// @for       The shape predicates autodetection decides with.
// @uses      regexp, strings.
// @reason    SPEC-API-002 §5 ports each reference predicate as its own function,
//
//	so the detection order in autodetect.go reads as the order and not as
//	a wall of pattern tests.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import (
	"regexp"
	"strings"
)

// isGrepLine reports whether one line is a "file:number:content" match.
func isGrepLine(line string) bool {
	first := strings.Index(line, ":")
	if first < 0 {
		return false
	}
	second := strings.Index(line[first+1:], ":")
	if second < 0 {
		return false
	}
	return reGrepLineNo.MatchString(line[first+1 : first+1+second])
}

// everyPathLike reports whether every line looks like a path: a Windows drive
// prefix, or a line with no colon that starts with a dot or a slash or carries
// one. A trailing grep-style ":10" on a drive path is tolerated.
func everyPathLike(lines []string) bool {
	for _, line := range lines {
		if !isPathLike(line) {
			return false
		}
	}
	return true
}

// isPathLike reports whether one trimmed line looks like a path.
func isPathLike(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if isDrivePath(trimmed) {
		return true
	}
	if strings.Contains(trimmed, ":") {
		return false
	}
	return strings.HasPrefix(trimmed, ".") || strings.HasPrefix(trimmed, "/") || strings.Contains(trimmed, "/")
}

// isDrivePath reports whether a line starts with a Windows drive letter.
func isDrivePath(trimmed string) bool {
	if len(trimmed) < 3 {
		return false
	}
	letter := trimmed[0]
	if (letter < 'A' || letter > 'Z') && (letter < 'a' || letter > 'z') {
		return false
	}
	return trimmed[1] == ':' && (trimmed[2] == '\\' || trimmed[2] == '/')
}

// isMostlyPorcelain reports whether at least 60% of the lines are porcelain
// status rows, which is what a `git status --porcelain` dump looks like.
func isMostlyPorcelain(nonEmpty []string) bool {
	if len(nonEmpty) < 3 {
		return false
	}
	hits := 0
	for _, line := range nonEmpty {
		if rePorcelain.MatchString(line) {
			hits++
		}
	}
	return float64(hits)/float64(len(nonEmpty)) >= 0.6
}

// isLineNumbered reports whether the sampled lines mostly carry the
// "  N|content" shape a Cursor read_file returns.
func isLineNumbered(lines []string) bool {
	hits, nonEmpty := 0, 0
	sample := lines
	if len(sample) > 100 {
		sample = sample[:100]
	}
	for _, line := range sample {
		if line == "" {
			continue
		}
		nonEmpty++
		if reReadNumberedLine.MatchString(line) {
			hits++
		}
	}
	if nonEmpty < 5 {
		return false
	}
	return float64(hits)/float64(nonEmpty) >= readNumberedMinHitRatio
}

// countMatches counts the non-overlapping matches of a pattern in a text.
func countMatches(re *regexp.Regexp, text string) int {
	return len(re.FindAllString(text, -1))
}
