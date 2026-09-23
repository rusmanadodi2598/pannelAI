// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/model_catalog.go
// @for       The merged model catalog: the embedded registry combined with the
//
//	models_custom and models_disabled rows (SPEC-API-001 §7.6).
//
// @uses      internal/domain, internal/repository, internal/registry, context,
//
//	time.
//
// @reason    §7.6 serves one catalog from three sources that disagree by
//
//	design: the registry is immutable config, custom rows are the
//	operator's additions, and disabled rows are subtractions. Merging
//	them here — in one place, in one order — is what lets the panel
//	and the data plane ask the same question and get the same answer.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"sort"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// CatalogFilter narrows the catalog list. A zero value means "no filter",
// which is distinct from a filter matching nothing.
type CatalogFilter struct {
	ProviderID string
	Capability string
	Query      string
}

// CatalogIndex is the registry view the catalog merges: every provider it can
// route to, and the provider behind one model string's namespace.
//
// It is an interface rather than *registry.Index because the composition root
// hands over its runtime overlay, which resolves stored custom nodes alongside
// the embedded document. Taking the concrete type here made a node invisible to
// the catalog even though the same node was routable everywhere else, so
// registering a model under it answered "unknown provider_id".
type CatalogIndex interface {
	Provider(name string) (registry.Provider, bool)
	All() []registry.Provider
}

// ModelCatalogService implements SPEC-API-001 §7.6.
type ModelCatalogService struct {
	index  CatalogIndex
	repo   repository.ModelCatalogRepository
	combos repository.ComboRepository
	clock  func() time.Time
}

// ModelCatalogServiceDeps holds the collaborators the service needs.
type ModelCatalogServiceDeps struct {
	Index  CatalogIndex
	Repo   repository.ModelCatalogRepository
	Combos repository.ComboRepository
}

// NewModelCatalogService validates deps and returns a ready service.
func NewModelCatalogService(deps ModelCatalogServiceDeps) (*ModelCatalogService, error) {
	if deps.Index == nil {
		return nil, domain.NewValidationError("model catalog service requires the provider registry")
	}
	if deps.Repo == nil {
		return nil, domain.NewValidationError("model catalog service requires a catalog repository")
	}
	if deps.Combos == nil {
		return nil, domain.NewValidationError("model catalog service requires a combo repository")
	}
	return &ModelCatalogService{index: deps.Index, repo: deps.Repo, combos: deps.Combos, clock: time.Now}, nil
}

// Catalog returns the merged catalog, disabled models excluded, filtered by
// provider, capability, and free text, ordered by provider then model so two
// identical requests produce identical output.
//
// The list is not paginated: every row here comes from an immutable in-memory
// index or from tables that hold one row per model an operator curated, so the
// bound is the registry's size rather than a customer's data (§1.7 constrains
// unbounded *queries*, and there is no query behind the registry half).
func (s *ModelCatalogService) Catalog(ctx context.Context, filter CatalogFilter) ([]domain.CatalogModel, error) {
	lookups, err := s.lookups(ctx)
	if err != nil {
		return nil, err
	}
	matched := make([]domain.CatalogModel, 0, len(lookups))
	for _, model := range lookups {
		if matchesCatalogFilter(model, filter) {
			matched = append(matched, model)
		}
	}
	sort.Slice(matched, func(a, b int) bool {
		left, right := matched[a].Ref().String(), matched[b].Ref().String()
		return left < right
	})
	return matched, nil
}

// lookups builds the merged catalog keyed by "provider/model". Custom rows
// override a registry row with the same pair: the registry entry is the port's
// opinion about a model, and a custom row is the operator's explicit statement
// about it.
func (s *ModelCatalogService) lookups(ctx context.Context) (map[string]domain.CatalogModel, error) {
	disabled, err := s.repo.Disabled(ctx)
	if err != nil {
		return nil, err
	}
	custom, err := s.repo.Custom(ctx)
	if err != nil {
		return nil, err
	}
	blocked := make(map[string]struct{}, len(disabled))
	for _, ref := range disabled {
		blocked[ref.String()] = struct{}{}
	}

	merged := s.registryModels(blocked)
	for _, model := range custom {
		key := model.Ref().String()
		if _, hidden := blocked[key]; hidden {
			continue
		}
		merged[key] = domain.NewCatalogModel(model.Ref(), model.DisplayName(), "", model.Capabilities(), domain.CatalogSourceCustom)
	}
	return merged, nil
}

// registryModels projects the index into catalog rows, skipping the disabled
// set.
func (s *ModelCatalogService) registryModels(blocked map[string]struct{}) map[string]domain.CatalogModel {
	providers := s.index.All()
	merged := make(map[string]domain.CatalogModel, len(providers)*4)
	for _, provider := range providers {
		for _, model := range provider.Models {
			ref, err := domain.NewModelRef(provider.ID, model.ID)
			if err != nil {
				// reason: a registry entry whose id cannot form a reference was
				// rejected at load time, so this is unreachable; skipping keeps
				// one malformed entry from failing the whole catalog.
				continue
			}
			if _, hidden := blocked[ref.String()]; hidden {
				continue
			}
			merged[ref.String()] = domain.NewCatalogModel(ref, registryDisplayName(model),
				model.Kind, registryCapabilityNames(provider.ID, model), domain.CatalogSourceRegistry)
		}
	}
	return merged
}
