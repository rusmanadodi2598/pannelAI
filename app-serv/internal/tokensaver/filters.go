// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/filters.go
// @for       The filter registry: the twelve canonical names and their
//
//	implementations.
//
// @uses      internal/domain (the canonical name list).
// @reason    SPEC-API-002 §4 makes the name list the contract between the
//
//	configuration and the engine, so the registry is a map keyed by
//	exactly domain.TokenSaverFilters. A test pins that the two agree,
//	which is what stops a name from being configurable but unimplemented
//	or implemented but unreachable.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

// filter rewrites one tool result's text. A filter never returns an error: a
// transformation that cannot do better than the input returns the input, and
// the engine's own never-empty and never-grow checks are the backstop.
type filter func(string) string

// registry maps each canonical filter name onto its implementation. The keys are
// domain.TokenSaverFilters, asserted by a test rather than by construction so a
// renamed constant fails loudly.
var registry = map[string]filter{
	"git-diff":       gitDiffFilter,
	"git-status":     gitStatusFilter,
	"git-log":        gitLogFilter,
	"grep":           grepFilter,
	"find":           findFilter,
	"ls":             lsFilter,
	"tree":           treeFilter,
	"dedup-log":      dedupLogFilter,
	"smart-truncate": smartTruncateFilter,
	"read-numbered":  readNumberedFilter,
	"search-list":    searchListFilter,
	"build-output":   buildOutputFilter,
}

// resolve returns the filter for a configured name. An unknown name resolves to
// nothing: the configuration layer rejects those, and a stored row that predates
// a name must not break a request.
func resolve(name string) (filter, bool) {
	fn, ok := registry[name]
	return fn, ok
}

// allowedSet turns the configured allowlist into a membership test. An empty
// list means every filter is eligible, which is the documented meaning of an
// empty `rtk.filters` (SPEC-API-002 §4).
func allowedSet(names []string) func(string) bool {
	if len(names) == 0 {
		return func(string) bool { return true }
	}
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	return func(name string) bool { return allowed[name] }
}
