// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/helpers.go
// @for       The small slice helpers the filters and the detector share.
// @uses      strings.
// @reason    SPEC-API-002 §5 caps every list a filter renders. One capping
//
//	helper is what keeps a cap from being written as an off-by-one in
//	each filter that has one.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import "strings"

// nonEmptyLines returns the lines that carry anything but whitespace.
func nonEmptyLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

// firstN returns at most n entries.
func firstN(lines []string, n int) []string {
	if len(lines) <= n {
		return lines
	}
	return lines[:n]
}

// anyGrepLine reports whether a line carries the reference's grep shape:
// "file:number:content", with the number between the first two colons.
func anyGrepLine(lines []string) bool {
	for _, line := range lines {
		if isGrepLine(line) {
			return true
		}
	}
	return false
}
