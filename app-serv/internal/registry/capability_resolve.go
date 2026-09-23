// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/capability_resolve.go
// @for       The one entry point every capability question goes through:
//
//	whether a model reads images, and whether it calls tools.
//
// @uses      strings.
// @reason    SPEC-API-001 §7.6 filters the catalog by `?capability=vision|tools`
//
//	and §7.8 refuses a vision adapter whose model cannot read images. Those
//	are two consumers asking one question, and the defect draft 017 §4.4
//	measured was that only one of them could answer it: the catalog read
//	`registry.yaml`'s capability strings (three media operations, no
//	modality) while the vision adapter read a pattern table. One resolver
//	here is what makes the two agree by construction.
//
//	The table itself stays in capability.go, which is the port of the
//	reference's PATTERN_CAPABILITIES vision decision; this file adds the
//	tools half and the provider/model normalisation the reference applies
//	before it looks anything up.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package registry

import "strings"

// CapabilitySet is what the resolver answers for one model: the two
// capabilities a client can filter on. It is a value, so it compares with ==
// and a test can state an expectation as a literal.
//
// The reference resolves eleven fields (modalities, reasoning, search, limits,
// thinking wire format). Only these two are ported, because they are the two
// §7.6 offers as filters; the rest would be carried data no caller reads, which
// is the state `transport.force_stream` is already in (draft 011 §2).
type CapabilitySet struct {
	// Vision reports whether the model reads images.
	Vision bool
	// Tools reports whether the model calls functions. It is true for almost
	// every model, which is the reference's floor; only an image-generation
	// model declares false.
	Tools bool
}

// toolsCapableIDs are the ids that answer false for tools. The reference's only
// two sources of a false tools answer are MODEL_CAPABILITIES["gpt-image-1"] and
// the dashboard service-kind mapping for an embedding model — and the latter is
// applied by the caller that knows a row is an embedding, not here.
var toolsCapableIDs = map[string]bool{
	"gpt-image-1": false,
}

// Capabilities answers both questions for one model.
//
// The order is the reference's (capabilities.js getCapabilitiesForModel): the
// exact-id layer first, then the ordered pattern table, then the floor. The
// provider argument is accepted because the reference's resolution order takes
// one, and the corpus test asserts that at the pinned revision the provider
// layer changes neither answer for any model the registry declares. When that
// stops being true, that test fails by name and the layer is ported then.
//
// An id nothing matches answers the floor: reads no images, calls tools. That
// direction is deliberate for the same reason VisionCapable's is — the caller
// uses vision to refuse a configuration, so guessing "capable" would let an
// operator wire a text-only model into the vision adapter and discover it when
// an image request fails upstream.
func Capabilities(provider, modelID string) CapabilitySet {
	floor := CapabilitySet{Vision: false, Tools: true}
	id := strings.ToLower(strings.TrimSpace(modelID))
	if id == "" {
		return floor
	}
	// The reference strips a vendor prefix before its exact and pattern
	// lookups: "anthropic/claude-opus-4.8" resolves as "claude-opus-4.8".
	base := id
	if slash := strings.LastIndex(id, "/"); slash >= 0 {
		base = id[slash+1:]
	}
	if base == "" {
		return floor
	}
	_ = provider

	if known, found := toolsCapableIDs[base]; found {
		return CapabilitySet{Vision: floor.Vision, Tools: known}
	}
	return CapabilitySet{Vision: visionFor(base, id), Tools: floor.Tools}
}

// visionFor applies the exact-id layer then the ordered pattern table, on the
// stripped id first and the full id second, which is the reference's order
// inside its pattern step.
func visionFor(base, full string) bool {
	for _, candidate := range []string{base, full} {
		if known, found := visionExactIDs[candidate]; found {
			return known
		}
	}
	for _, rule := range visionRules {
		if matchesGlob(rule.pattern, base) || matchesGlob(rule.pattern, full) {
			return rule.vision
		}
	}
	return false
}

// Has reports whether the set carries a capability, by the names §7.6 offers.
//
// A name outside the set answers false rather than erroring: the catalog's
// capability parameter is an open string (app-ui/src/lib/schemas/model.ts:52-55
// states the API's vocabulary is any string a model declares), and the media
// strings `edit`, `mask`, and `text2img` come from the document rather than from
// this resolver, so the caller checks those against the declared set.
func (c CapabilitySet) Has(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "vision":
		return c.Vision
	case "tools":
		return c.Tools
	default:
		return false
	}
}
