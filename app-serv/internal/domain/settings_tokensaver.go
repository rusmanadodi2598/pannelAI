// Package domain holds the entities and value objects of the gateway.
//
// @file      internal/domain/settings_tokensaver.go
// @for       The §7.9 token-saver group and the two vocabularies it is written
//
//	with: the RTK filter names and the ponytail levels.
//
// @uses      none.
// @reason    SPEC-API-002 §4 and §7 fix the twelve filter names and the three
//
//	levels, and both are pinned by tests against the packages that
//	implement them, so they are value-object vocabulary rather than
//	configuration. They live in their own file because settings.go holds
//	the document these groups sit in, and AGENTS.md §1.1 keeps the two
//	apart.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-26
package domain

// TokenSaverFilters is the canonical filter vocabulary of the native RTK engine
// (SPEC-API-002 §4). It is the single source for the twelve names: the schema's
// oneof tag is pinned against it by a test, and the engine's registry resolves
// exactly these.
var TokenSaverFilters = []string{
	"git-diff",
	"git-status",
	"git-log",
	"grep",
	"find",
	"ls",
	"tree",
	"dedup-log",
	"smart-truncate",
	"read-numbered",
	"search-list",
	"build-output",
}

// ValidTokenSaverFilter reports whether a name is one the engine implements.
// Aliases the reference accepts on a command line (rg, fd) are not configuration
// values: the panel picks from the canonical list, so the stored document
// carries only names the registry resolves by itself.
func ValidTokenSaverFilter(name string) bool {
	for _, known := range TokenSaverFilters {
		if name == known {
			return true
		}
	}
	return false
}

// TokenSaverLevels is the strength vocabulary the ponytail saver accepts, in
// the order the levels escalate. The native engine's prompt table is keyed by
// exactly these words, and a test in that package fails when the two drift
// (SPEC-API-002 §7).
var TokenSaverLevels = []string{"lite", "full", "ultra"}

// ValidTokenSaverLevel reports whether a level is one the engine implements.
func ValidTokenSaverLevel(level string) bool {
	for _, known := range TokenSaverLevels {
		if level == known {
			return true
		}
	}
	return false
}

// TokenSaverRTK is the native engine's group (§7.9): an enable flag and the
// filter allowlist. An empty list means every filter is eligible; the engine
// still autodetects which one applies to a given tool result (SPEC-API-002 §4).
type TokenSaverRTK struct {
	Enabled bool     `json:"enabled"`
	Filters []string `json:"filters"`
}

// TokenSaverToggle is one enable/level saver group (§7.9).
type TokenSaverToggle struct {
	Enabled bool   `json:"enabled"`
	Level   string `json:"level"`
}

// TokenSaverHeadroom is the external compression saver group (§7.9).
type TokenSaverHeadroom struct {
	Enabled              bool   `json:"enabled"`
	URL                  string `json:"url"`
	CompressUserMessages bool   `json:"compress_user_messages"`
}

// TokenSaverSettings is the §7.9 saver configuration, minus caveman.
type TokenSaverSettings struct {
	RTK      TokenSaverRTK      `json:"rtk"`
	Headroom TokenSaverHeadroom `json:"headroom"`
	Ponytail TokenSaverToggle   `json:"ponytail"`
}
