// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/constants.go
// @for       The engine's caps, all of them ports of the reference's constants.
// @uses      (none).
// @reason    SPEC-API-002 §5 makes every cap a documented number. Keeping them
//
//	here is what lets the filters read as logic instead of as a wall of
//	literals, and what makes a cap change one edit rather than a hunt.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import "time"

const (
	// RawCap is the largest blob the engine will hand to a filter. Above it the
	// blob is passed through untouched, because a tool result that large is
	// usually a file the model needs whole (reference RAW_CAP).
	RawCap = 10 * 1024 * 1024
	// MinCompressSize is the floor below which a blob is not worth rewriting:
	// the filter's own markers would cost more than they save.
	MinCompressSize = 500
	// DetectWindow is how much of a blob autodetection reads.
	DetectWindow = 1024

	// gitDiffMaxLines caps the whole compacted diff.
	gitDiffMaxLines = 500
	// gitDiffHunkMaxLines caps one hunk's kept lines.
	gitDiffHunkMaxLines = 100
	// gitLogMaxLines caps a compacted log.
	gitLogMaxLines = 200
	// dedupLogLineMax caps the deduplicated log.
	dedupLogLineMax = 2000
	// grepPerFileMax caps the matches shown per file.
	grepPerFileMax = 10
	// findPerDirMax caps the files shown per directory.
	findPerDirMax = 10
	// findTotalDirMax caps the directories shown.
	findTotalDirMax = 20
	// statusMaxFiles caps the files listed per status group.
	statusMaxFiles = 10
	// statusMaxUntracked caps the untracked files listed.
	statusMaxUntracked = 10
	// lsExtSummaryTop is how many extensions the ls summary names.
	lsExtSummaryTop = 5
	// treeMaxLines caps a tree listing.
	treeMaxLines = 200
	// searchListPerDirMax caps the files shown per directory.
	searchListPerDirMax = 10
	// searchListTotalDirMax caps the directories shown.
	searchListTotalDirMax = 20
	// smartTruncateHead and smartTruncateTail are the line counts the head/tail
	// truncation keeps.
	smartTruncateHead = 120
	smartTruncateTail = 60
	// smartTruncateMinLines is the line count above which truncation applies.
	smartTruncateMinLines = 250
	// readNumberedMinHitRatio is the fraction of sampled lines that must look
	// like "  N|content" before read-numbered claims a blob.
	readNumberedMinHitRatio = 0.7
	// buildOutputDeprecationKeep is how many deprecation notices stay verbatim.
	buildOutputDeprecationKeep = 3
	// buildOutputWarningKeep is how many warnings stay verbatim.
	buildOutputWarningKeep = 5

	// HeadroomDefaultTimeout is how long the external compression call may take
	// before the request is abandoned and the body goes upstream uncompressed.
	// SPEC-API-001 §7.9 fixes the value; the reference's own default is 3s, and
	// SPEC-API-002 §7 records the difference.
	HeadroomDefaultTimeout = 5 * time.Second
	// headroomMaxResponseBytes bounds what the proxy may hand back. The client
	// is the one place the gateway reads a body it did not build, so the bound
	// lives with the read rather than with the caller.
	headroomMaxResponseBytes = 16 * 1024 * 1024
)

// lsNoiseDirs are the directory names the ls filter drops. The reference keeps
// this list beside its caps, and the dotenv `.env` file is deliberately not in
// it: only the listed names are skipped.
var lsNoiseDirs = map[string]bool{
	"node_modules":  true,
	".git":          true,
	"target":        true,
	"__pycache__":   true,
	".next":         true,
	"dist":          true,
	"build":         true,
	".cache":        true,
	".turbo":        true,
	".vercel":       true,
	".pytest_cache": true,
	".mypy_cache":   true,
	".tox":          true,
	".venv":         true,
	"venv":          true,
	"env":           true,
	"coverage":      true,
	".nyc_output":   true,
	".DS_Store":     true,
	"Thumbs.db":     true,
	".idea":         true,
	".vscode":       true,
	".vs":           true,
	"*.egg-info":    true,
	".eggs":         true,
}
