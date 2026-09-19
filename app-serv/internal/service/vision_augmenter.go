// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/vision_augmenter.go
// @for       The data plane's vision augmentation seam, answered by the adapter
//
//	configuration, the capability predicate, and the rotation store.
//
// @uses      internal/dataplane, internal/domain, internal/repository, context.
// @reason    SPEC-API-001 §7.8 puts the augmentation policy in the adapter while
//
//	the request pipeline only knows the seam, so the two halves need
//	one adapter between them. It lives in the service layer because it
//	orchestrates the adapter use case and a repository — not transport,
//	not SQL — and the composition root hands it to the engine as the
//	seam's implementation.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// VisionAugmenter implements dataplane.VisionAugmenter over the §7.8
// configuration: it declines for a model that reads images itself, and
// otherwise reports the adapter's order with the rotation state advanced.
type VisionAugmenter struct {
	adapter  *VisionAdapterService
	capable  domain.VisionCapabilityCheck
	rotation repository.VisionRotationStore
}

// VisionAugmenterDeps holds the collaborators the augmenter needs. Rotation is
// optional: a nil store means every image-bearing request starts at the
// configured order, which is the documented behaviour of a caller with nowhere
// to persist state.
type VisionAugmenterDeps struct {
	Adapter  *VisionAdapterService
	Capable  domain.VisionCapabilityCheck
	Rotation repository.VisionRotationStore
}

// NewVisionAugmenter validates deps and returns the seam's implementation.
func NewVisionAugmenter(deps VisionAugmenterDeps) (*VisionAugmenter, error) {
	if deps.Adapter == nil {
		return nil, domain.NewValidationError("vision augmenter requires the adapter service")
	}
	capable := deps.Capable
	if capable == nil {
		capable = RejectAllVisionCapability
	}
	return &VisionAugmenter{adapter: deps.Adapter, capable: capable, rotation: deps.Rotation}, nil
}

// Augment answers the data plane's one question: should this request try other
// models first, and which.
func (a *VisionAugmenter) Augment(ctx context.Context, providerID, modelID string) ([]string, bool, error) {
	// The judgement uses the id the client's model carries in the catalog, the
	// same reading the panel's picker and the capability table agree on.
	ref, err := domain.NewModelRef(providerID, modelID)
	if err != nil {
		return nil, false, err
	}
	if a.capable(ref) {
		return nil, false, nil
	}

	state := domain.RotationState{}
	if a.rotation != nil {
		if stored, storeErr := a.rotation.Get(ctx); storeErr == nil {
			state = stored
		}
	}
	order, err := a.adapter.Applicable(ctx, state)
	if err != nil {
		return nil, false, err
	}
	if !order.Applies || len(order.Models) == 0 {
		return nil, false, nil
	}
	// The adapter's enabled and round-robin rules already ran, so this request
	// owns the state it produced. A persistence failure costs one request of
	// skew, which is exactly what the store's contract says is acceptable.
	if a.rotation != nil {
		// reason: the rotation is advisory, and failing a served request over a
		// lost advisory position would be worse than the skew it prevents.
		_ = a.rotation.Save(ctx, order.Next)
	}
	return order.Models, true, nil
}
