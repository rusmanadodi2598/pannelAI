// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/model_vision.go
// @for       The one capability question the vision adapter asks the catalog.
//
// @uses      context, internal/domain, internal/registry.
// @reason    SPEC-API-001 §7.8 refuses an adapter model that cannot read images,
//
//	and the data plane asks the same question to decide whether to adapt at
//	all. Both used to be answered by a name pattern that never read the
//	catalog: a custom model an operator declared vision-capable stayed
//	invisible to routing, and a registered model the ported table asserted
//	was never contradicted by what the operator actually saw. Measured live
//	2026-09-29: the configured adapter answered a solid-red image as "gray"
//	while a model the tables call blind answered it correctly. The catalog is
//	the operator's statement, so it answers first.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-29
package service

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// VisionCapable reports whether one catalog model reads images, and whether the
// catalog speaks for it at all.
//
// The answer is additive: a catalog row that declares vision is believed, that is
// the lever no model-id pattern can provide for a model an operator created, and
// otherwise the resolver answers, with the row's provider id so the per-provider
// layer is reached. A row is never read as denying a capability it simply does not
// mention, because every custom node's models are rows that mention nothing about
// images: routing then called a reference-declared vision model blind and handed
// the request to the adapter, which answered a solid-red image "white".
//
// This is deliberately not modelHasCapability, the panel filter's predicate, which
// treats a custom row's declared set as the whole answer and so can veto. The two
// disagree in one direction only, and on purpose: the filter is a screen an operator
// reads, where a declared set is a statement, while a wrong answer here routes real
// image content to a model that cannot see it.
//
// answered is false for a model the catalog does not list, one a pass-through
// provider answers under an id nobody registered, and the caller then falls back
// to the predicate it was wired with.
func (s *ModelCatalogService) VisionCapable(ctx context.Context, ref domain.ModelRef) (capable, answered bool, err error) {
	// The reference view is the one canonicalizer the write path already uses for
	// ModelExists and ChatServable: the catalog stores a row under the provider's
	// id, while a caller may name it by alias or node prefix, and a raw map probe
	// would report the same model as unlisted depending on how it was spelled
	// (draft 024 §3.2).
	view, err := newReferenceView(s, ctx)
	if err != nil {
		return false, false, err
	}
	capable, answered = visionCapableFromView(view, ref)
	return capable, answered, nil
}

// VisionCapableSet answers the same question for many models from ONE catalog
// read. The single-model path builds a reference view each time, and the vision
// augmenter asks about every candidate of every image-bearing request, which is
// the query-per-item shape AGENTS.md §1.7 blocks. Keys are ref.String().
func (s *ModelCatalogService) VisionCapableSet(
	ctx context.Context, refs []domain.ModelRef,
) (map[string]visionAnswer, error) {
	if len(refs) == 0 {
		return map[string]visionAnswer{}, nil
	}
	view, err := newReferenceView(s, ctx)
	if err != nil {
		return nil, err
	}
	answers := make(map[string]visionAnswer, len(refs))
	for _, ref := range refs {
		capable, answered := visionCapableFromView(view, ref)
		answers[ref.String()] = visionAnswer{Capable: capable, Answered: answered}
	}
	return answers, nil
}

// visionAnswer is what the catalog says about one model's sight: whether it can
// see, and whether the catalog spoke at all.
type visionAnswer struct {
	Capable  bool
	Answered bool
}

func visionCapableFromView(view referenceView, ref domain.ModelRef) (capable, answered bool) {
	key, listed := view.catalogKeyFor(ref)
	if !listed {
		return false, false
	}
	row := view.lookups[key]
	// Additive, and only where the row actually speaks. A custom row can turn a
	// capability ON that no pattern knows; a row that simply does not mention
	// vision is not the operator denying it. modelHasCapability, which the panel's
	// filter uses, where a declared set IS the whole answer, vetoes the resolver
	// for any non-registry row, and every custom node's models are non-registry rows
	// that declare nothing about images. Measured live on 2026-09-29: routing then
	// read deepseek-v4.1-flash as blind (the reference itself says vision-capable),
	// handed omp-agent's image to the adapter, and answered "white" for a red image.
	if row.Capabilities().Has(capabilityVision) {
		return true, true
	}
	return registry.Capabilities(row.ProviderID(), row.ModelID()).Vision, true
}
