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
// catalog speaks for it at all. The answer is additive: a row declaring vision is
// believed, otherwise the resolver answers with the row's provider id so the
// per-provider layer is reached. A row never denies a capability it does not
// mention, because every custom node's models mention nothing about images. This
// is deliberately not modelHasCapability, the panel filter's predicate, which can
// veto: the filter is a screen, while a wrong answer here routes image content to
// a model that cannot see. answered is false for a model the catalog does not
// list, such as a pass-through provider's unregistered id, so the caller falls
// back to the predicate it was wired with.
func (s *ModelCatalogService) VisionCapable(ctx context.Context, ref domain.ModelRef) (capable, answered bool, err error) {
	// The reference view is the one canonicalizer the write path already uses for
	// ModelExists and ChatServable: the catalog stores a row under the provider's
	// id, while a caller may name it by alias or node prefix, and a raw map probe
	// would report the same model as unlisted depending on how it was spelled.
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
	// Additive, and only where the row actually speaks: a custom row can turn a
	// capability ON that no pattern knows, and a row that does not mention vision
	// is not the operator denying it. modelHasCapability, the panel filter, treats a
	// declared set as the whole answer and vetoes the resolver for any non-registry
	// row, which is the behaviour this path must not share.
	if row.Capabilities().Has(capabilityVision) {
		return true, true
	}
	return registry.Capabilities(row.ProviderID(), row.ModelID()).Vision, true
}
