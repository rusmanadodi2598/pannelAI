// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/autodetect.go
// @for       Which filter claims a blob, in the reference's detection order.
// @uses      regexp, strings.
// @reason    SPEC-API-002 §5 fixes the detection order because it is
//
//	observable: build output must be claimed before the porcelain check
//	or a cargo run reads as a dirty tree, and git-log must be claimed
//	before git-diff or a log with an embedded diff loses its subjects.
//	Keeping the order in one function is what makes it auditable.
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

var (
	reGitDiff     = regexp.MustCompile(`(?m)^diff --git `)
	reGitDiffHunk = regexp.MustCompile(`(?m)^@@ `)
	reGitStatus   = regexp.MustCompile(`(?m)^On branch |^nothing to commit|^Changes (not |to be )|^Untracked files:`)
	reGitLog      = regexp.MustCompile(`(?m)^[*|/\\ ]*commit [0-9a-f]{7,40}$`)
	rePorcelain   = regexp.MustCompile(`(?m)^[ MADRCU?!][ MADRCU?!] \S`)
	reBuildOutput = regexp.MustCompile(`(?im)^(npm (warn|error|ERR!)|yarn (warn|error)|\s*Compiling\s+\S+|\s*Downloading\s+\S+|added \d+ package|\[ERROR\]|BUILD (SUCCESS|FAILED)|\s*Finished\s+|Successfully (installed|built)|ERROR:)`)
	reTreeGlyph   = regexp.MustCompile(`[├└]──|│  `)
	reLsRow       = regexp.MustCompile(`(?m)^[-dlbcps][rwx-]{9}`)
	reLsTotal     = regexp.MustCompile(`(?m)^total \d+$`)
	reGrepLineNo  = regexp.MustCompile(`^\d+$`)
)

// detect returns the filter that claims a blob, or nothing when the blob is not
// a shape the engine compresses. The allowed set is the configured allowlist:
// a filter outside it is skipped even when it would match, so the configuration
// decides which shapes are worth rewriting (SPEC-API-002 §4).
func detect(text string, allowed func(string) bool) (filter, string, bool) {
	head := text
	if len(head) > DetectWindow {
		head = head[:DetectWindow]
	}
	// A window cut mid-rune would leave an invalid trailing byte; the matchers
	// below only read ASCII markers, so the invalid byte is harmless, and the
	// truncation is documented rather than silently repaired.
	lines := strings.Split(head, "\n")
	nonEmpty := nonEmptyLines(lines)

	if allowed("git-log") && reGitLog.MatchString(head) {
		return claim("git-log")
	}
	if allowed("git-diff") && (reGitDiff.MatchString(head) || reGitDiffHunk.MatchString(head)) {
		return claim("git-diff")
	}
	if allowed("git-status") && reGitStatus.MatchString(head) {
		return claim("git-status")
	}
	// Build output is claimed before the porcelain check: it is what stops a
	// cargo "Compiling" run from reading as a dirty working tree.
	if allowed("build-output") && reBuildOutput.MatchString(head) {
		return claim("build-output")
	}
	if allowed("git-status") && isMostlyPorcelain(nonEmpty) {
		return claim("git-status")
	}
	// The reference's grep rule: any of the first five non-empty lines carries
	// a "file:number:content" match.
	if allowed("grep") && anyGrepLine(firstN(nonEmpty, 5)) {
		return claim("grep")
	}
	// The reference's find rule: every non-empty line is path-like, at least
	// three of them.
	if allowed("find") && len(nonEmpty) >= 3 && everyPathLike(nonEmpty) {
		return claim("find")
	}
	if allowed("tree") && reTreeGlyph.MatchString(head) {
		return claim("tree")
	}
	if allowed("ls") && (reLsTotal.MatchString(head) || countMatches(reLsRow, head) >= 3) {
		return claim("ls")
	}
	if allowed("search-list") && reSearchListHeader.MatchString(head) {
		return claim("search-list")
	}
	// The line-numbered shape is measured over the whole blob rather than the
	// detection window: a numbered file dump is claimed by its line count, and
	// the window cannot hold the 250 lines the threshold names. This is a
	// deliberate deviation from the reference, where the check reads the window
	// and therefore almost never fires (SPEC-API-002 §5).
	if allowed("read-numbered") && strings.Count(text, "\n")+1 >= smartTruncateMinLines && isLineNumbered(lines) {
		return claim("read-numbered")
	}
	// The generic fallback: multi-line noise. It claims anything with five or
	// more non-empty lines, which is why smart-truncate below only sees blobs
	// that are almost all blank.
	if allowed("dedup-log") && len(nonEmpty) >= 5 {
		return claim("dedup-log")
	}
	if allowed("smart-truncate") && len(strings.Split(text, "\n")) >= smartTruncateMinLines {
		return claim("smart-truncate")
	}
	return nil, "", false
}

// claim resolves a detected name, reporting a miss when the registry has no
// implementation for it. The two lists are kept in step by a test, so a miss
// here means a programming error rather than a configuration one.
func claim(name string) (filter, string, bool) {
	fn, ok := resolve(name)
	return fn, name, ok
}
