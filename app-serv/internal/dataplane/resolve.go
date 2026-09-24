// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/resolve.go
// @for       Model-string resolution: combo name, then alias, then
//
//	provider/model, and the routability gate.
//
// @uses      internal/domain, internal/registry.
// @reason    SPEC-API-001 §7.15 fixes the order and the failure code
//
//	(MODEL_NOT_FOUND), and §8 adds PROVIDER_NOT_ROUTABLE for a
//	provider whose protocol has no translator: both answers decide
//	whether a request is served at all, so they are computed here
//	once instead of being re-derived by each caller.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// ModelLookup is the read path resolution and the catalog listing need from the
// combo and catalog boundaries. It is declared here rather than depending on four
// repository contracts so the resolver asks narrow questions, and so a question it
// does not ask cannot leak into this package. Every set it returns is the whole
// set: the alias, disabled, and combo tables are small and fully rewritten
// (SPEC-API-001 §7.6 replaces them wholesale), so reading one set per request is
// the read path rather than an unbounded query.
type ModelLookup interface {
	// Combo returns a combo by name, and whether the name addresses one. The
	// aggregate travels whole rather than as a bare reference list because the
	// fusion strategy is executed from it, and a second read per request would
	// let the two answers disagree.
	Combo(ctx context.Context, name string) (combo domain.Combo, found bool, err error)
	// Alias returns an alias's target, and whether the alias exists.
	Alias(ctx context.Context, name string) (target string, found bool, err error)
	// Disabled reports whether one model is hidden from routing (§7.6).
	Disabled(ctx context.Context, providerID, modelID string) (bool, error)
	// DisabledPairs returns the whole disabled set, which is what the catalog
	// listing filters with in one read.
	DisabledPairs(ctx context.Context) ([]domain.ModelRef, error)
	// ComboNames returns every combo name, because a combo is addressed by name
	// as a model string.
	ComboNames(ctx context.Context) ([]string, error)
}

// Resolution is a model string resolved to everything routing needs.
type Resolution struct {
	// Provider is the registry entry that will answer.
	Provider registry.Provider
	// Model is the registry model when the provider declares one; a provider
	// that passes model ids through, or a custom node, yields the client's id.
	Model registry.Model
	// ModelID is the model the client named, after alias dereferencing.
	ModelID string
	// UpstreamID is the id the upstream expects, with any registry override
	// applied.
	UpstreamID string
	// Target is the wire format to translate into.
	Target string
	// Combo is the combo a model string addressed, or the zero combo. Its
	// references, strategy, judge, and sticky limit travel whole because the
	// strategy rules live in the aggregate, and reading them from four fields
	// copied out of it is how the two answers could drift.
	Combo domain.Combo
}

// IsCombo reports whether a combo answered the model string. A combo always
// carries a name (the domain refuses one without), so the name is the test.
func (r Resolution) IsCombo() bool { return r.Combo.Name() != "" }

// ProviderRegistry is the registry surface the resolver needs: one provider
// lookup by id, alias, or node prefix, one declared-model lookup inside it, and
// the full list the models endpoint enumerates.
//
// It is an interface rather than *registry.Index because the composition root
// overlays the stored custom nodes on the embedded registry. A node created
// through POST /provider-nodes must be routable by the next request, and a
// boot-frozen index would accept the create and then refuse every request aimed
// at it — which reads as a routing bug rather than as a stale registry.
type ProviderRegistry interface {
	// Provider resolves an id, alias, or node prefix to its entry.
	Provider(name string) (registry.Provider, bool)
	// Model resolves a declared model inside a provider.
	Model(providerName, modelID string) (registry.Model, bool)
	// All returns every entry, embedded and custom.
	All() []registry.Provider
}

// Resolver turns a client model string into a routable provider.
type Resolver struct {
	index  ProviderRegistry
	lookup ModelLookup
}

// NewResolver binds the resolver to the loaded registry and the catalog read
// path. Both are required: without the registry it cannot tell whether a
// provider is routable, and without the catalog it cannot see combos or aliases.
func NewResolver(index ProviderRegistry, lookup ModelLookup) (*Resolver, error) {
	if index == nil {
		return nil, domain.NewValidationError("provider registry is required")
	}
	if lookup == nil {
		return nil, domain.NewValidationError("model lookup is required")
	}
	return &Resolver{index: index, lookup: lookup}, nil
}

// Resolve applies the documented order: combo name, then alias, then
// provider/model, then MODEL_NOT_FOUND (SPEC-API-001 §7.15).
func (r *Resolver) Resolve(ctx context.Context, model string) (Resolution, error) {
	return r.resolveWithin(ctx, model, nil, 0)
}

// resolveWithin is Resolve carrying the resolution's working state and the
// alias hops already followed, so a combo member or an alias target re-enters
// resolution with the memo and the guards the outer call already built. A nil
// state is the entry point's own start.
//
// The alias hop count guards the one cycle the write path refuses but a direct
// database row can still hold: an alias whose target is another alias. The
// guard is a count rather than a visited set because an alias chain is a line,
// not a graph — one target per name — so a count answers the same question with
// no map to build.
func (r *Resolver) resolveWithin(ctx context.Context, model string, state *resolveState, aliasHops int) (Resolution, error) {
	if model == "" {
		return Resolution{}, dataPlaneError(CodeModelNotFound, "the model field is required")
	}
	if aliasHops >= aliasHopLimit {
		return Resolution{}, dataPlaneError(CodeModelNotFound,
			"the model chain has more alias hops than the resolver follows")
	}

	// A combo is addressed by a bare name, so a string carrying "/" cannot be
	// one; that also stops a provider whose id collides with a combo name from
	// being shadowed.
	if !hasSlash(model) {
		combo, found, err := r.lookup.Combo(ctx, model)
		if err != nil {
			return Resolution{}, err
		}
		if found && len(combo.Refs()) > 0 {
			return r.resolveCombo(ctx, combo, state)
		}

		target, found, err := r.lookup.Alias(ctx, model)
		if err != nil {
			return Resolution{}, err
		}
		if found {
			return r.resolveWithin(ctx, target, state, aliasHops+1)
		}
		return Resolution{}, dataPlaneError(CodeModelNotFound,
			"model "+model+" is not a known model, alias, or combo")
	}
	return r.resolveReference(ctx, model)
}

// ResolveParts resolves an already-split provider identifier and model id, so a
// combo entry, an alias target, and a client's model string all run through one
// implementation.
func (r *Resolver) ResolveParts(_ context.Context, providerName, modelID string) (Resolution, error) {
	entry, ok := r.index.Provider(providerName)
	if !ok {
		return Resolution{}, dataPlaneError(CodeModelNotFound,
			"provider "+providerName+" is not in the registry")
	}
	// A provider the gateway cannot translate is refused by name rather than
	// attempted and reported as an upstream failure (SPEC-API-001 §8).
	if !entry.IsChatRoutable() {
		return Resolution{}, dataPlaneError(CodeProviderNotRoutable,
			"provider "+entry.ID+" speaks a wire format the gateway does not translate")
	}

	resolution := Resolution{Provider: entry, ModelID: modelID, Target: targetFormat(entry.Transport.Format)}
	if resolution.Target == "" {
		return Resolution{}, dataPlaneError(CodeProviderNotRoutable,
			"provider "+entry.ID+" speaks "+entry.Transport.Format+", which has no translator yet")
	}

	// A declared model wins; a provider that passes model ids through, or a
	// user-defined node with no model list, accepts the client's id as-is.
	if declared, found := r.index.Model(entry.ID, modelID); found {
		// A declared model that is not a chat model is refused by name rather
		// than served: its payload is a different vocabulary (the reference's
		// `kind: "systemone"` decision models), so a chat body sent to it is a
		// request the upstream cannot parse. The refusal is MODEL_NOT_FOUND
		// because the chat plane genuinely has no such model, and the message
		// names the kind so the reason is readable.
		if !declared.IsChat() {
			return Resolution{}, dataPlaneError(CodeModelNotFound,
				"model "+entry.ID+"/"+modelID+" is not a chat model (kind "+declared.Kind+")")
		}
		resolution.Model = declared
		resolution.UpstreamID = declared.UpstreamID()
		if declared.TargetFormat != "" {
			if target := targetFormat(declared.TargetFormat); target != "" {
				resolution.Target = target
			}
		}
		return resolution, nil
	}
	if !entry.PassthroughModels && !entry.Custom {
		return Resolution{}, dataPlaneError(CodeModelNotFound,
			"model "+entry.ID+"/"+modelID+" is not available")
	}
	resolution.Model = registry.Model{ID: modelID}
	resolution.UpstreamID = modelID
	return resolution, nil
}

// Allowed reports whether a model is routable, applying the disabled set the
// catalog owns. It is separate from Resolve so the models list endpoint can ask
// exactly the question the router asks.
func (r *Resolver) Allowed(ctx context.Context, providerID, modelID string) bool {
	disabled, err := r.lookup.Disabled(ctx, providerID, modelID)
	if err != nil {
		return false
	}
	return !disabled
}
