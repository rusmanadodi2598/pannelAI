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

// Custom returns every custom model.
func (s *ModelCatalogService) Custom(ctx context.Context) ([]domain.CustomModel, error) {
	return s.repo.Custom(ctx)
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
	lookups, err := s.lookups(ctx)
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
		if _, ok := lookups[alias.Target()]; ok {
			continue
		}
		if _, ok := known[alias.Target()]; ok {
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
	for _, ref := range refs {
		if ref.IsZero() {
			return domain.NewValidationError("provider_id and model_id are required")
		}
		if _, ok := available[ref.String()]; !ok {
			return domain.NewValidationError("unknown model: " + ref.String())
		}
	}
	return s.repo.ReplaceDisabled(ctx, refs)
}
