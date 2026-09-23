// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/model_resolve.go
// @for       Model-string resolution and the existence check every reference
//
//	inside a write goes through (SPEC-API-001 §7.15, §7.6).
//
// @uses      internal/domain, context, strings.
// @reason    §7.15 fixes one resolution order — combo name, then alias, then
//
//	provider/model — and §7.6 requires an alias target to exist. Both
//	answers come from the same merged view, so they live together: an
//	alias validated against one view and resolved against another is
//	exactly the drift this file prevents.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// RefKind names what a model string resolved to, so a caller can tell a combo
// from a model without re-parsing the string.
type RefKind string

const (
	RefKindModel RefKind = "model"
	RefKindCombo RefKind = "combo"
	RefKindAlias RefKind = "alias"
)

// RefTarget is the answer to "what does this model string name". Exactly one of
// the three shapes is populated, selected by Kind.
type RefTarget struct {
	Kind      RefKind
	Model     domain.ModelRef
	ComboID   string
	ComboName string
	Alias     domain.ModelAlias
}

// ModelExists reports whether the catalog can name a model. A combo model ref, a
// judge model, and a vision adapter entry all validate through this, so they
// cannot disagree about what "exists" means.
//
// The reference is accepted in every form the router resolves (draft 024 §3.2):
// the provider's id, its registry alias, or a node prefix. The first segment is
// canonicalized through the index, which is the same lookup the data plane
// performs, so a name the router would route is never refused here.
func (s *ModelCatalogService) ModelExists(ctx context.Context, ref domain.ModelRef) (bool, error) {
	if ref.IsZero() {
		return false, nil
	}
	lookups, err := s.lookups(ctx)
	if err != nil {
		return false, err
	}
	return resolvesIn(lookups, s.index, ref), nil
}

// ChatServable reports whether the chat data plane could serve a reference,
// and why not when it cannot. It is the write-time half of the property the
// §7.15 model list holds — listed and answerable are one property — so a
// reference the router would refuse (a provider with no chat translator, a
// media model) is refused here with the reason named, before it is saved as
// part of a combo or the vision adapter (draft 024 §3.4).
func (s *ModelCatalogService) ChatServable(ctx context.Context, ref domain.ModelRef) error {
	lookups, err := s.lookups(ctx)
	if err != nil {
		return err
	}
	return chatServable(lookups, s.index, ref)
}

// Resolve maps a model string to what it names, in the order §7.15 fixes: combo
// name, then alias, then provider/model. The second value is false when nothing
// matches, which the data plane answers as MODEL_NOT_FOUND.
//
// An alias resolves one level only: its target is a provider/model reference or
// a combo name, never another alias, so a cycle cannot exist and a typo'd chain
// cannot loop. A disabled model does not resolve, because §7.6 hides it from
// routing as well as from the catalog.
func (s *ModelCatalogService) Resolve(ctx context.Context, model string) (RefTarget, bool, error) {
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return RefTarget{}, false, domain.NewValidationError("model is required")
	}
	combo, err := s.combos.GetByName(ctx, trimmed)
	switch {
	case err == nil:
		return RefTarget{Kind: RefKindCombo, ComboID: combo.ID(), ComboName: combo.Name()}, true, nil
	case !isNotFound(err):
		return RefTarget{}, false, err
	}

	alias, found, err := s.lookupAlias(ctx, trimmed)
	if err != nil {
		return RefTarget{}, false, err
	}
	if found {
		return s.resolveAliasTarget(ctx, alias)
	}

	ref, err := domain.ParseModelRef(trimmed)
	if err != nil {
		//nolint:nilerr // reason: a string that is not a provider/model reference is an unresolvable target, not a failure; the caller answers "not found" for it.
		return RefTarget{}, false, nil
	}
	hidden, err := s.isDisabled(ctx, ref)
	if err != nil {
		return RefTarget{}, false, err
	}
	if hidden {
		return RefTarget{}, false, nil
	}
	exists, err := s.ModelExists(ctx, ref)
	if err != nil {
		return RefTarget{}, false, err
	}
	if !exists {
		return RefTarget{}, false, nil
	}
	// The reported reference carries the canonical id, because the caller may
	// store or display it: a ref typed with an alias or a prefix is the
	// operator's spelling of one model, and the id is the spelling every other
	// surface agrees on.
	if canonical, ok := canonicalRef(s.index, ref); ok {
		ref = canonical
	}
	return RefTarget{Kind: RefKindModel, Model: ref}, true, nil
}

// canonicalRef maps a reference onto the provider's canonical id spelling.
// A name the index does not hold is returned unchanged: the caller's own
// validation decides what an unknown namespace means.
func canonicalRef(index CatalogIndex, ref domain.ModelRef) (domain.ModelRef, bool) {
	if ref.IsZero() {
		return ref, false
	}
	entry, ok := index.Provider(ref.ProviderID())
	if !ok || entry.ID == ref.ProviderID() {
		return ref, false
	}
	canonical, err := domain.NewModelRef(entry.ID, ref.ModelID())
	if err != nil {
		return ref, false
	}
	return canonical, true
}

// lookupAlias finds one alias by name.
func (s *ModelCatalogService) lookupAlias(ctx context.Context, name string) (domain.ModelAlias, bool, error) {
	aliases, err := s.repo.Aliases(ctx)
	if err != nil {
		return domain.ModelAlias{}, false, err
	}
	for _, alias := range aliases {
		if alias.Alias() == name {
			return alias, true, nil
		}
	}
	return domain.ModelAlias{}, false, nil
}

// resolveAliasTarget follows one alias to its target, which is either a
// provider/model reference or a combo name. A target that names neither is an
// error rather than a miss: a stored alias that cannot resolve means the row was
// written outside this service, and reporting "not found" would hide that.
func (s *ModelCatalogService) resolveAliasTarget(ctx context.Context, alias domain.ModelAlias) (RefTarget, bool, error) {
	if strings.Contains(alias.Target(), "/") {
		ref, err := domain.ParseModelRef(alias.Target())
		if err != nil {
			return RefTarget{}, false, err
		}
		return RefTarget{Kind: RefKindAlias, Alias: alias, Model: ref}, true, nil
	}
	combo, err := s.combos.GetByName(ctx, alias.Target())
	if err != nil {
		return RefTarget{}, false, err
	}
	return RefTarget{Kind: RefKindAlias, Alias: alias, ComboID: combo.ID(), ComboName: combo.Name()}, true, nil
}
