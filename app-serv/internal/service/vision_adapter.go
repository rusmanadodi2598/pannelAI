// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/vision_adapter.go
// @for       The vision adapter configuration: read it, replace it, validate its
//
//	model list against the catalog, and order it for one request
//	(SPEC-API-001 §7.8).
//
// @uses      internal/domain, internal/repository, context, time.
// @reason    §7.8 requires every adapter model to be a catalog model a vision
//
//	capability check accepts, and the capability data is not in the
//	registry yet. The predicate therefore arrives as a dependency
//	(domain.VisionCapabilityCheck) rather than being guessed here: when
//	the capability table lands, the composition root swaps the
//	predicate and nothing else changes.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// VisionAdapterService implements SPEC-API-001 §7.8.
type VisionAdapterService struct {
	repo    repository.VisionAdapterRepository
	catalog *ModelCatalogService
	capable domain.VisionCapabilityCheck
	clock   func() time.Time
}

// VisionAdapterServiceDeps holds the collaborators the service needs.
//
// Capable is the capability seam: the predicate answering "is this catalog
// model vision-capable". A nil predicate is replaced by one that rejects every
// model, so the API never accepts a model on no evidence — enabling the adapter
// therefore requires the predicate to be supplied, which is exactly the
// dependency the missing capability data represents.
type VisionAdapterServiceDeps struct {
	Repo    repository.VisionAdapterRepository
	Catalog *ModelCatalogService
	Capable domain.VisionCapabilityCheck
}

// AdapterOrder is the adapter's answer to one image-bearing request: the model
// order to try, the rotation state the next request continues from, and whether
// the adapter applies at all.
type AdapterOrder struct {
	Models  []string
	Next    domain.RotationState
	Applies bool
}

// NewVisionAdapterService validates deps and returns a ready service.
func NewVisionAdapterService(deps VisionAdapterServiceDeps) (*VisionAdapterService, error) {
	if deps.Repo == nil {
		return nil, domain.NewValidationError("vision adapter service requires a configuration repository")
	}
	if deps.Catalog == nil {
		return nil, domain.NewValidationError("vision adapter service requires the model catalog service")
	}
	capable := deps.Capable
	if capable == nil {
		capable = RejectAllVisionCapability
	}
	return &VisionAdapterService{repo: deps.Repo, catalog: deps.Catalog, capable: capable, clock: time.Now}, nil
}

// RejectAllVisionCapability is the placeholder predicate the service uses until
// the registry carries capability data: it accepts nothing.
//
// It exists so the seam is explicit and the default is safe. The registry's
// Model.Capabilities currently holds media operations ("text2img", "edit") and
// not input modalities, so inferring "vision" from a model id would be a guess
// the API would then act on — a wrong "yes" routes image content to a model
// that drops it.
func RejectAllVisionCapability(domain.ModelRef) bool { return false }

// Get returns the stored configuration, or the disabled default when nothing has
// been written yet.
func (s *VisionAdapterService) Get(ctx context.Context) (domain.VisionAdapter, error) {
	return s.repo.Get(ctx)
}

// Replace swaps the whole configuration (§7.8 PUT).
//
// Every model must be a catalog model — the panel's picker draws from the
// catalog, so a value outside it is a client bug worth reporting — and the
// capability predicate must accept it. The catalog check runs first so an
// unknown model reports "unknown" rather than "not vision-capable", which is
// the more actionable of the two.
func (s *VisionAdapterService) Replace(ctx context.Context, enabled, roundRobin bool, models []domain.ModelRef) (domain.VisionAdapter, error) {
	for _, ref := range models {
		exists, err := s.catalog.ModelExists(ctx, ref)
		if err != nil {
			return domain.VisionAdapter{}, err
		}
		if !exists {
			return domain.VisionAdapter{}, domain.NewValidationError("unknown model: " + ref.String())
		}
	}
	adapter, err := domain.NewVisionAdapter(enabled, roundRobin, models, s.capable, s.clock())
	if err != nil {
		return domain.VisionAdapter{}, err
	}
	if err := s.repo.Save(ctx, adapter); err != nil {
		return domain.VisionAdapter{}, err
	}
	return adapter, nil
}

// Applicable reports the model order an image-bearing request should try when
// the resolved model cannot see images.
//
// It returns the next rotation state alongside the order rather than keeping it,
// because the state belongs to whoever persists it: the data plane writes it
// next to the request it served, and a caller with nowhere to put it passes the
// zero state and always starts at the configured order.
func (s *VisionAdapterService) Applicable(ctx context.Context, state domain.RotationState) (AdapterOrder, error) {
	adapter, err := s.repo.Get(ctx)
	if err != nil {
		return AdapterOrder{}, err
	}
	if !adapter.Active() {
		return AdapterOrder{Applies: false}, nil
	}
	order, next := adapter.NextOrder(state)
	return AdapterOrder{Models: order, Next: next, Applies: true}, nil
}
