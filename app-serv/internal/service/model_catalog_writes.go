// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/model_catalog_writes.go
// @for       The catalog's own write paths: custom models, the alias set, and
//
//	the disabled set (SPEC-API-001 §7.6).
//
// @uses      internal/domain, context, strings.
// @reason    Every write here is validated against the same merged view the read
//
//	serves: an alias target must exist, and a disabled pair must name a
//	catalog model. Keeping the read model and the write rules in one
//	place is what stops them from drifting, which is the failure mode a
//	"just insert it" store invites.
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

// Custom returns every custom model, or one provider's when providerID is set.
//
// The filter accepts every spelling the provider answers to — its id, its
// registry alias, or a node prefix — through the same canonical set the catalog
// filter uses (draft 024 §3.2), so an operator narrowing by alias sees the same
// rows the id form shows. An empty value means "no filter", which is distinct
// from a filter matching nothing.
func (s *ModelCatalogService) Custom(ctx context.Context, providerID string) ([]domain.CustomModel, error) {
	models, err := s.repo.Custom(ctx)
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(providerID)
	if trimmed == "" {
		return models, nil
	}
	// The match is two-way because a custom row may carry either spelling: the
	// write path has accepted the node prefix as a provider_id since custom
	// nodes existed, so rows stored under `corp` must surface when the filter
	// names the node's id, and vice versa. Matching only the filter's names
	// against the row would hide every prefix-stored row from the id form.
	//
	// Both directions read one name table, built from a single index snapshot:
	// in production the index adapter rebuilds the node overlay on every
	// Provider() call, so a per-row lookup would be one node-list query per row.
	names := providerNames(s.index)
	target, known := names[trimmed]
	if !known {
		return nil, nil
	}
	matched := make([]domain.CustomModel, 0, len(models))
	for _, model := range models {
		if providerAnswersTo(target, model.ProviderID()) {
			matched = append(matched, model)
			continue
		}
		row, ok := names[model.ProviderID()]
		if ok && providerAnswersTo(row, trimmed) {
			matched = append(matched, model)
		}
	}
	return matched, nil
}

// AddCustom registers a user-added model (§7.6 POST). The provider must exist in
// the index the service was handed (including a synthesized provider node),
// because a model under an unknown provider is unroutable and §6 makes an
// unknown provider id a VALIDATION_ERROR.
//
// The row is constructed before the registry is consulted so a request that is
// missing a required field is told which field is missing, rather than being told
// the empty provider is unknown. Both answers are VALIDATION_ERROR; only one of
// them is actionable.
func (s *ModelCatalogService) AddCustom(ctx context.Context, providerID, modelID, displayName string, capabilities domain.ModelCapabilities) (domain.CustomModel, error) {
	now := s.clock()
	model, err := domain.NewCustomModel(domain.NewULID(now), providerID, modelID, displayName, capabilities, now)
	if err != nil {
		return domain.CustomModel{}, err
	}
	if _, ok := s.index.Provider(model.ProviderID()); !ok {
		return domain.CustomModel{}, domain.NewValidationError("unknown provider_id: " + model.ProviderID())
	}
	if err := s.repo.AddCustom(ctx, model); err != nil {
		return domain.CustomModel{}, err
	}
	return model, nil
}

// RemoveCustom deletes one custom model row.
func (s *ModelCatalogService) RemoveCustom(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return domain.NewValidationError("id is required")
	}
	return s.repo.RemoveCustom(ctx, id)
}

// Aliases returns the whole alias set.
func (s *ModelCatalogService) Aliases(ctx context.Context) ([]domain.ModelAlias, error) {
	return s.repo.Aliases(ctx)
}

// ReplaceAliases swaps the whole set after validating every target (§7.6 PUT).
//
// The set is validated in full before anything is written, so a request with one
// bad row leaves the previous set untouched: §8.1's all-or-nothing rule for bulk
// writes applies here for the same reason — a half-replaced mapping is harder to
// reason about than a refused one.
func (s *ModelCatalogService) ReplaceAliases(ctx context.Context, aliases []domain.ModelAlias) error {
	view, err := newReferenceView(s, ctx)
	if err != nil {
		return err
	}
	comboNames, err := s.combos.Names(ctx)
	if err != nil {
		return err
	}
	known := make(map[string]struct{}, len(comboNames))
	for _, name := range comboNames {
		known[name] = struct{}{}
	}
	for _, alias := range aliases {
		if _, ok := known[alias.Alias()]; ok {
			return domain.NewValidationError("alias " + alias.Alias() + " is already a combo name")
		}
		if _, ok := known[alias.Target()]; ok {
			continue
		}
		parsed, err := domain.ParseModelRef(alias.Target())
		if err == nil && view.resolvesIn(parsed) {
			continue
		}
		return domain.NewValidationError("alias " + alias.Alias() + " targets an unknown model or combo: " + alias.Target())
	}
	return s.repo.ReplaceAliases(ctx, aliases)
}

// Disabled returns every disabled model pair.
func (s *ModelCatalogService) Disabled(ctx context.Context) ([]domain.ModelRef, error) {
	return s.repo.Disabled(ctx)
}

// ReplaceDisabled swaps the whole disabled set after validating that every pair
// names something the catalog holds. Disabling a model the catalog cannot see
// would store a rule with nothing to hide, and the panel would render a row it
// cannot explain.
func (s *ModelCatalogService) ReplaceDisabled(ctx context.Context, refs []domain.ModelRef) error {
	// Validation reads the disabled-inclusive view: a model that is already
	// disabled is absent from the offered catalog by definition.
	available, err := s.availableRefs(ctx)
	if err != nil {
		return err
	}
	view, err := newReferenceView(s, ctx)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if ref.IsZero() {
			return domain.NewValidationError("provider_id and model_id are required")
		}
		if _, ok := available[ref.String()]; ok {
			continue
		}
		// A pair spelled with an alias or a node prefix names the same model as
		// the id form, so it is accepted the same way every other write accepts
		// it (draft 024 §3.2). The stored pair keeps the operator's spelling.
		if view.resolvesIn(ref) {
			continue
		}
		return domain.NewValidationError("unknown model: " + ref.String())
	}
	return s.repo.ReplaceDisabled(ctx, refs)
}
