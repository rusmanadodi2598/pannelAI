// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/capability_commandcode.go
// @for       The Command Code wire's own vision answer: one endpoint for every
//
//	model, so a text-only denylist decides instead of the family
//	patterns.
//
// @uses      strings.
// @reason    SPEC-API-001 §7.6 filters the catalog by capability, and the
//
//	reference answers the two Command Code provider ids from a dedicated
//	branch (capabilities.js:570-583) rather than from its pattern table:
//	a family pattern would claim e.g. deepseek-v4 reads images, while the
//	Command Code CLI cannot send one. The branch is its own file because
//	it is a second resolution rule with its own data, and because
//	capability_resolve.go stays inside the AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-26
package registry

import "strings"

// commandCodeProviders are the provider ids whose wire is one endpoint for
// every model (`/alpha/generate`), so the family patterns must not decide their
// vision: the reference answers those two ids from a dedicated branch that
// reads a text-only denylist (capabilities.js:570-583) instead of the pattern
// table, because a family pattern would claim e.g. deepseek-v4 reads images
// while the Command Code CLI cannot send one.
var commandCodeProviders = map[string]bool{"commandcode": true, "cmc": true}

// commandCodeTextOnly is the reference's own denylist: ids that take no image
// input on the Command Code wire. Everything else on that wire answers vision,
// which is the reference's default there (new models are assumed multimodal).
var commandCodeTextOnly = map[string]bool{
	"deepseek/deepseek-v4-pro":              true,
	"deepseek/deepseek-v4-flash":            true,
	"deepseek/deepseek-v4-flash-fast":       true,
	"zai-org/glm-5.3":                       true,
	"zai-org/glm-5.2":                       true,
	"zai-org/glm-5.2-fast":                  true,
	"zai-org/glm-5.1":                       true,
	"zai-org/glm-5":                         true,
	"minimaxai/minimax-m2.7":                true,
	"minimax/minimax-m2.7-free":             true,
	"minimaxai/minimax-m2.5":                true,
	"xiaomi/mimo-v2.5-pro":                  true,
	"qwen/qwen3.6-max-preview":              true,
	"qwen/qwen3.7-max":                      true,
	"meituan/longcat-2.0:free":              true,
	"stepfun/step-3.5-flash":                true,
	"tencent/hy4-preview":                   true,
	"tencent/hy3":                           true,
	"tencent/hy3-paid":                      true,
	"nvidia/nemotron-3-ultra-550b-a55b":     true,
	"poolside/laguna-s-2.1-free":            true,
	"inclusionai/ling-3.0-flash-free":       true,
	"inclusionai/ling-3.0-flash-sante:free": true,
}

// commandCodeTextOnlyModel reports whether an id is text-only on the Command
// Code wire. The reference matches the full id, then the last path segment,
// then any id ending in that segment, because a client may send either the
// namespaced id or the bare one.
func commandCodeTextOnlyModel(id string) bool {
	if commandCodeTextOnly[id] {
		return true
	}
	for key := range commandCodeTextOnly {
		base := key
		if slash := strings.LastIndex(key, "/"); slash >= 0 {
			base = key[slash+1:]
		}
		if id == base || strings.HasSuffix(id, "/"+base) {
			return true
		}
	}
	return false
}
