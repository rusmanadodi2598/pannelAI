// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/capability_thinking.go
// @for       The reasoning decision for one model: whether it reasons, the wire
//
//	format its thinking takes, and whether thinking can be turned off.
//
// @uses      strings (the glob matcher's normalisation).
// @reason    The reference derives its thinking-level picker and its
//
//	thinking-format dispatch from three fields of the same capability
//	resolution the port already carries for vision and tools
//	(capabilities.js:562-605). The port reads them from
//	capability_thinking_tables.go, in the reference's own layer order —
//	provider override, exact id, ordered pattern, floor — because a
//	reordered walk answers differently for every id two rows match.
//
//	The answer is a value the data plane and the panel both read: the
//	panel offers the levels §7.6's reasoning control can pick, and the
//	relay path applies the picked mode in the format named here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-26
package registry

import "strings"

// thinkingRule is one row of a reasoning table: the pattern it matches (empty
// for the map-keyed layers, where the key is the id), the wire format the
// model's thinking takes, whether the model can turn thinking off, and whether
// the row reasons at all. A row with reasons false is a stop row: the reference
// stops its walk there, so a later reasoning row must not claim the id.
type thinkingRule struct {
	pattern       string
	format        string
	cannotDisable bool
	reasons       bool
}

// thinkingAnswer is the resolver's answer for one model. CanDisable is true for
// every model the reference's floor covers, because only a row that says so
// clamps "none" to a minimal effort instead of disabling thinking.
type thinkingAnswer struct {
	Reasoning  bool
	Format     string
	CanDisable bool
}

// answer turns a table row into the resolver's value.
func (r thinkingRule) answer() thinkingAnswer {
	return thinkingAnswer{Reasoning: r.reasons, Format: r.format, CanDisable: !r.cannotDisable}
}

// thinkingFor resolves the reasoning answer for one model, in the reference's
// order: the Command Code branch, the provider override, the exact id, the
// ordered pattern walk, then the floor. base is the id with its vendor prefix
// stripped and id is the full id, both already lower-cased by the caller.
func thinkingFor(provider, base, id string) thinkingAnswer {
	// The Command Code wire is one endpoint for every model, and the reference
	// answers it from a dedicated branch before any table lookup
	// (capabilities.js:570-583), so a family pattern must not decide it.
	if commandCodeProviders[strings.ToLower(strings.TrimSpace(provider))] {
		return thinkingAnswer{Reasoning: true, Format: "commandcode", CanDisable: true}
	}
	if overrides, found := providerThinkingIDs[strings.ToLower(strings.TrimSpace(provider))]; found {
		if rule, ok := overrides[id]; ok {
			return rule.answer()
		}
		if rule, ok := overrides[base]; ok {
			return rule.answer()
		}
	}
	for _, candidate := range []string{base, id} {
		if rule, found := thinkingExactIDs[candidate]; found {
			return rule.answer()
		}
	}
	for _, rule := range thinkingRules {
		if matchesGlob(rule.pattern, base) || matchesGlob(rule.pattern, id) {
			return rule.answer()
		}
	}
	return thinkingAnswer{CanDisable: true}
}
