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
// capabilities a client can filter on, plus the reasoning decision the thinking
// control reads. It is a value, so it compares with == and a test can state an
// expectation as a literal.
//
// The reference resolves eleven fields (modalities, reasoning, search, limits,
// thinking wire format). Only these are ported, because they are the ones a
// caller reads: vision and tools are the two §7.6 offers as filters, and the
// reasoning trio is what the reasoning control (§7.14) and the relay path need.
type CapabilitySet struct {
	// Vision reports whether the model reads images.
	Vision bool
	// Tools reports whether the model calls functions. It is true for almost
	// every model, which is the reference's floor; only an image-generation
	// model declares false.
	Tools bool
	// Reasoning reports whether the model thinks before answering.
	Reasoning bool
	// ThinkingFormat names the wire shape the model's thinking takes, as the
	// reference's thinkingUnified.js dispatches on it. An empty value means the
	// reference declared none, and the caller falls back to the target wire's
	// own default.
	ThinkingFormat string
	// CanDisable is false for a model that cannot turn thinking off, where a
	// "none" mode clamps to the lowest level instead of disabling.
	CanDisable bool
	// ThinkingRange clamps a budget-format model's budget when the reference
	// declares one; nil means no clamp. It is a pointer so "no clamp" and a
	// clamp at zero stay distinguishable.
	ThinkingRange *ThinkingRange
	// EffortSupported marks the models whose thinking format also reads a
	// reasoning_effort level on top of its thinking object (the reference's
	// thinkingEffortSupported): z.ai from GLM-5.2 onward, DeepSeek V4, and
	// every Command Code model.
	EffortSupported bool
}

// Capabilities answers every question the resolver owns for one model.
//
// The order is the reference's (capabilities.js getCapabilitiesForModel): the
// exact-id layer first, then the ordered pattern table, then the floor. The
// provider argument is accepted because the reference's resolution order takes
// one, and the corpus test asserts that at the pinned revision the provider
// layer changes neither answer for any model the registry declares. When that
// stops being true, that test fails by name and the layer is ported then.
//
// An id nothing matches answers the floor: reads no images, calls tools, and
// does not reason. That direction is deliberate for the same reason
// VisionCapable's is: the caller uses vision to refuse a configuration, so
// guessing "capable" would let an operator wire a text-only model into the
// vision adapter and discover it when an image request fails upstream.
func Capabilities(provider, modelID string) CapabilitySet {
	floor := CapabilitySet{Vision: false, Tools: true, CanDisable: true}
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

	// The one id that declares no tool calling is also the one image model the
	// reference's tables answer false for, so the row decides both fields.
	if known, found := toolsCapableIDs[base]; found {
		return CapabilitySet{Vision: false, Tools: known, CanDisable: true}
	}

	thinking := thinkingFor(provider, base, id)
	return CapabilitySet{
		Vision:          visionAnswer(provider, id, base),
		Tools:           floor.Tools,
		Reasoning:       thinking.Reasoning,
		ThinkingFormat:  thinking.Format,
		CanDisable:      thinking.CanDisable,
		ThinkingRange:   thinking.Range,
		EffortSupported: thinking.Effort,
	}
}

// visionAnswer resolves the vision decision alone, in the reference's order.
func visionAnswer(provider, id, base string) bool {
	// The Command Code wire answers every model from one endpoint, so its
	// vision comes from the reference's own denylist rather than from a family
	// pattern that describes the model's native provider instead.
	if commandCodeProviders[strings.ToLower(strings.TrimSpace(provider))] {
		return !commandCodeTextOnlyModel(id)
	}
	// The provider override is consulted first because the reference consults
	// it first: an entry naming a model that a family pattern would answer
	// differently wins, and codebuddy-cn's deepseek-v4-pro is exactly that case.
	if vision, found := providerVision(provider, base, id); found {
		return vision
	}
	// The name heuristic runs last and only in the true direction, which is the
	// reference's own order and direction (capabilities.js:520): a table that
	// answered false is not overridden by a name, and a model no table knows
	// still accepts an image when its id says so.
	return visionFor(base, id) || looksLikeVisionModel(id)
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
