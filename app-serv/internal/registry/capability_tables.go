// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/capability_tables.go
// @for       The capability tables the resolver reads: which models call tools,
//
//	which ids answer vision directly, and the per-provider vision overrides.
//
// @uses      (none; the tables are data read by capability_resolve.go).
// @reason    Each table is a transcription of one layer in the reference's
//
//	capabilities.js, in the reference's own order. Keeping them apart
//	from the resolver is what holds capability_resolve.go inside the
//	AGENTS.md §1.1 budget, and it makes the port reviewable: a reader
//	diffing this file against the reference sees one table per layer.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-24
package registry

// toolsCapableIDs are the ids that answer false for tools. The reference's only
// two sources of a false tools answer are MODEL_CAPABILITIES["gpt-image-1"] and
// the dashboard service-kind mapping for an embedding model — and the latter is
// applied by the caller that knows a row is an embedding, not here.
var toolsCapableIDs = map[string]bool{
	"gpt-image-1": false,
}

// providerVisionIDs is the reference's PROVIDER_CAPABILITIES layer reduced to
// its vision decision: an override keyed by (provider, model) that wins over
// the exact and pattern layers, because the reference consults it first
// (capabilities.js:586-590).
//
// The port carries it because the pinned revision proves it is load-bearing:
// four models the embedded registry declares answer differently with it than
// without (codebuddy-cn's deepseek-v4-pro and the two MiniMax entries under
// nvidia and commandcode), and the corpus test fails by name if that changes.
// Only the rows that declare a vision answer are listed; the reference's other
// provider overrides set thinking and limit fields the port does not model.
var providerVisionIDs = map[string]map[string]bool{
	"nvidia": {
		"minimaxai/minimax-m2.7":        false,
		"minimaxai/minimax-m3":          true,
		"z-ai/glm-5.2":                  false,
		"deepseek-ai/deepseek-v4-pro":   false,
		"deepseek-ai/deepseek-v4-flash": false,
	},
	"opencode-go": {
		"glm-5.3-flash": true,
	},
	"codex": {
		"gpt-6-astra":          true,
		"gpt-5.6-sol":          true,
		"gpt-5.6-sol-review":   true,
		"gpt-5.6-terra":        true,
		"gpt-5.6-terra-review": true,
		"gpt-5.6-luna":         true,
		"gpt-5.6-luna-review":  true,
	},
	"kiro": {
		"gpt-5.6-sol":                    true,
		"gpt-5.6-terra":                  true,
		"gpt-5.6-luna":                   true,
		"gpt-5.6-sol-thinking":           true,
		"gpt-5.6-terra-thinking":         true,
		"gpt-5.6-luna-thinking":          true,
		"gpt-5.6-sol-agentic":            true,
		"gpt-5.6-terra-agentic":          true,
		"gpt-5.6-luna-agentic":           true,
		"gpt-5.6-sol-thinking-agentic":   true,
		"gpt-5.6-terra-thinking-agentic": true,
		"gpt-5.6-luna-thinking-agentic":  true,
	},
	"codebuddy-cn": {
		"glm-5.2":             true,
		"glm-5.1":             true,
		"glm-5.0":             false,
		"glm-5v-turbo":        true,
		"glm-4.7":             false,
		"minimax-m3":          true,
		"kimi-k2.7":           true,
		"kimi-k2.6":           true,
		"kimi-k2.5":           true,
		"hy3-preview":         true,
		"deepseek-v4-flash":   false,
		"deepseek-v3-2-volc":  false,
		"hy3":                 true,
		"hy4-preview":         true,
		"glm-5.3":             true,
		"glm-5.3-flash":       true,
		"kimi-k3-1":           true,
		"deepseek-v4-pro":     true,
		"deepseek-v4.1-flash": true,
	},
	"poolside": {
		"laguna-s-2.1":  false,
		"laguna-xs-2.1": false,
	},
	"ollama": {
		"deepseek-v4.1-flash:cloud": true,
	},
}
