// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/vision_augmenter.go
// @for       The data plane's vision augmentation seam, answered by the adapter configuration, the catalog's capability judgement, and the rotation store.
// @uses      internal/dataplane, internal/domain, internal/repository, context.
// @reason    SPEC-API-001 §7.8 puts the augmentation policy in the adapter while the request pipeline only knows the seam, so the two halves need one adapter between them. It lives in the service layer because it orchestrates the adapter use case and a repository, not transport, not SQL, and the composition root hands it to the engine as the seam's implementation.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-19
package service

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/logx"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// VisionAugmenter implements dataplane.VisionAugmenter over the §7.8
// configuration: it orders a request's candidates so nothing that reads images
// itself is displaced, and reports the adapter's own models alongside the order.
type VisionAugmenter struct {
	adapter  *VisionAdapterService
	rotation repository.VisionRotationStore
}

// VisionAugmenterDeps holds the collaborators the seam needs. Rotation is
// optional: a nil store means every image-bearing request starts at the
// configured order, which is the documented behaviour of a caller with nowhere
// to persist state.
type VisionAugmenterDeps struct {
	Adapter *VisionAdapterService
	// Rotation is the adapter's round-robin position, kept in Redis.
	Rotation repository.VisionRotationStore
}

// NewVisionAugmenter validates deps and returns the seam's implementation.
func NewVisionAugmenter(deps VisionAugmenterDeps) (*VisionAugmenter, error) {
	if deps.Adapter == nil {
		return nil, domain.NewValidationError("vision augmenter requires the adapter service")
	}
	return &VisionAugmenter{adapter: deps.Adapter, rotation: deps.Rotation}, nil
}

// Augment answers the data plane's one question: in what order should this
// image-bearing request try its candidates, and which of them is the adapter's
// doing. The adapter is inserted after every candidate that can see and before every
// candidate that cannot. Putting it first, as this once did, means a combo whose own
// member reads images never gets to run; leaving it out entirely would strand the
// request that has no capable member at all, which is the case the adapter exists for.
func (a *VisionAugmenter) Augment(ctx context.Context, candidates []string) ([]string, []string, error) {
	seeing, blind, err := a.splitBySight(ctx, candidates)
	if err != nil {
		return nil, nil, err
	}
	if len(blind) == 0 {
		return candidates, nil, nil
	}

	state := domain.RotationState{}
	if a.rotation != nil {
		stored, storeErr := a.rotation.Get(ctx)
		if storeErr != nil {
			// The rotation restarts, which is a degraded answer rather than a
			// failure, and it has to be visible as one: an adapter that silently
			// stops advancing looks like a provider that stopped being used.
			logx.Degraded(nil, "vision rotation state unreadable, rotation restarts", storeErr)
		} else {
			state = stored
		}
	}
	order, err := a.adapter.Applicable(ctx, state)
	if err != nil {
		return nil, nil, err
	}
	if !order.Applies || len(order.Models) == 0 {
		return candidates, nil, nil
	}
	// The adapter's enabled and round-robin rules already ran, so this request
	// owns the state it produced. A persistence failure costs one request of
	// skew, which is exactly what the store's contract says is acceptable.
	if a.rotation != nil {
		// reason: the rotation is advisory, and failing a served request over a
		// lost advisory position would be worse than the skew it prevents.
		_ = a.rotation.Save(ctx, order.Next)
	}

	refs := make([]string, 0, len(seeing)+len(order.Models)+len(blind))
	refs = append(refs, seeing...)
	refs = append(refs, order.Models...)
	refs = append(refs, blind...)
	return refs, order.Models, nil
}

// splitBySight divides the candidates into the ones that read images and the ones
// that do not, each keeping the order it was given.
//
// A reference the catalog cannot be asked about, a nested combo name or an alias,
// which the walk resolves in its own step, is no evidence of blindness, so it
// stays where the request put it rather than pulling the adapter in front of it.
func (a *VisionAugmenter) splitBySight(ctx context.Context, candidates []string) (seeing, blind []string, err error) {
	seeing = make([]string, 0, len(candidates))
	blind = make([]string, 0, len(candidates))

	// One catalog read for the whole candidate list. Asking per candidate rebuilt
	// the reference view each time, which is two table reads per image-bearing
	// candidate on the request path (§1.7).
	asked := make([]domain.ModelRef, len(candidates))
	refs := make([]domain.ModelRef, 0, len(candidates))
	for index, candidate := range candidates {
		ref, parseErr := domain.ParseModelRef(candidate)
		if parseErr != nil {
			// The zero ref marks "the catalog cannot be asked about it", decided in
			// the pass below so it keeps its place in the list rather than the pass
			// it happened to be readable in.
			continue
		}
		asked[index] = ref
		refs = append(refs, ref)
	}

	capable, err := a.adapter.VisionCapableSet(ctx, refs)
	if err != nil {
		return nil, nil, err
	}
	for index, candidate := range candidates {
		ref := asked[index]
		// A reference the catalog cannot be asked about (a nested combo name, an
		// alias the walk resolves in its own step) is no evidence of blindness.
		if ref.String() == "" || capable[ref.String()] {
			seeing = append(seeing, candidate)
			continue
		}
		blind = append(blind, candidate)
	}
	return seeing, blind, nil
}
