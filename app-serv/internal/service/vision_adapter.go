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
// @stability stable
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
// model, so the API never accepts a model on no evidence, enabling the adapter
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
// the API would then act on, a wrong "yes" routes image content to a model
// that drops it.
func RejectAllVisionCapability(domain.ModelRef) bool { return false }

// VisionCapable answers §7.8's one capability question for one model, asked the
// same way by the write path that validates a configuration and by the data plane
// that decides whether to adapt a request.
//
// The catalog answers first because it holds the operator's own statement: a
// custom model row declaring vision is believed, and one that omits it is not
// overridden by a name pattern. The injected predicate answers only for a model no
// catalog row speaks for, which is what a pass-through provider's id is.
func (s *VisionAdapterService) VisionCapable(ctx context.Context, ref domain.ModelRef) (bool, error) {
	capable, found, err := s.catalog.VisionCapable(ctx, ref)
	if err != nil || found {
		return capable, err
	}
	return s.capable(ref), nil
}

// VisionCapableSet answers the same question for a set of models in one catalog
// read, for the request path that must classify every candidate of an
// image-bearing request. A model the catalog does not answer for falls back to the
// injected predicate exactly as the single-model path does. Keys are ref.String().
func (s *VisionAdapterService) VisionCapableSet(
	ctx context.Context, refs []domain.ModelRef,
) (map[string]bool, error) {
	answers, err := s.catalog.VisionCapableSet(ctx, refs)
	if err != nil {
		return nil, err
	}
	capable := make(map[string]bool, len(refs))
	for _, ref := range refs {
		answer, found := answers[ref.String()]
		if found && answer.Answered {
			capable[ref.String()] = answer.Capable
			continue
		}
		capable[ref.String()] = s.capable(ref)
	}
	return capable, nil
}

// Get returns the stored configuration, or the disabled default when nothing has
// been written yet.
func (s *VisionAdapterService) Get(ctx context.Context) (domain.VisionAdapter, error) {
	return s.repo.Get(ctx)
}

// Replace swaps the whole configuration. Every model must be a catalog model, since
// the panel's picker draws from the catalog and a value outside it is a client bug
// worth reporting, and the capability predicate must accept it. The catalog check
// runs first so an unknown model reports "unknown" rather than "not
// vision-capable", the more actionable of the two. A model the chat plane cannot
// serve is refused before the capability check: the adapter prepends its models to
// an image-bearing request, so an entry on a provider with no chat translator, or
// on a media row, turns every image request it meant to save into a routing failure.
func (s *VisionAdapterService) Replace(ctx context.Context, enabled, roundRobin bool, models []domain.ModelRef) (domain.VisionAdapter, error) {
	for _, ref := range models {
		exists, err := s.catalog.ModelExists(ctx, ref)
		if err != nil {
			return domain.VisionAdapter{}, err
		}
		if !exists {
			return domain.VisionAdapter{}, domain.NewValidationError("unknown model: " + ref.String())
		}
		if err := s.catalog.ChatServable(ctx, ref); err != nil {
			return domain.VisionAdapter{}, err
		}
	}
	// The domain aggregate takes a synchronous predicate, so the catalog read this
	// request owns is closed over rather than threaded through the domain's
	// signature. A failed read answers "not capable": refusing a save is the safe
	// direction on a write path, where the data plane's opposite choice (serve
	// un-adapted) is safe on a request path.
	capable := func(ref domain.ModelRef) bool {
		answer, err := s.VisionCapable(ctx, ref)
		return err == nil && answer
	}
	adapter, err := domain.NewVisionAdapter(enabled, roundRobin, models, capable, s.clock())
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
