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
// direction is deliberate for the same reason VisionCapable's is: the caller
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

	if known, found := toolsCapableIDs[base]; found {
		return CapabilitySet{Vision: floor.Vision, Tools: known}
	}
	// The Command Code wire answers every model from one endpoint, so its
	// vision comes from the reference's own denylist rather than from a family
	// pattern that describes the model's native provider instead.
	if commandCodeProviders[strings.ToLower(strings.TrimSpace(provider))] {
		return CapabilitySet{Vision: !commandCodeTextOnlyModel(id), Tools: floor.Tools}
	}
	// The provider override is consulted first because the reference consults
	// it first: an entry naming a model that a family pattern would answer
	// differently wins, and codebuddy-cn's deepseek-v4-pro is exactly that case.
	if vision, found := providerVision(provider, base, id); found {
		return CapabilitySet{Vision: vision, Tools: floor.Tools}
	}
	// The name heuristic runs last and only in the true direction, which is the
	// reference's own order and direction (capabilities.js:520): a table that
	// answered false is not overridden by a name, and a model no table knows
	// still accepts an image when its id says so.
	return CapabilitySet{Vision: visionFor(base, id) || looksLikeVisionModel(id), Tools: floor.Tools}
}

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

// providerVision reads the (provider, model) override layer, trying the
// provider's own spelling before the caller's. It reports false when neither
// key is declared, which is what lets the exact and pattern layers answer.
func providerVision(provider, base, full string) (bool, bool) {
	name := strings.ToLower(strings.TrimSpace(provider))
	if name == "" {
		return false, false
	}
	byModel, found := providerVisionIDs[name]
	if !found {
		return false, false
	}
	for _, candidate := range []string{base, full} {
		if vision, ok := byModel[candidate]; ok {
			return vision, true
		}
	}
	return false, false
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
