// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_detail.go
// @for       The provider detail read: one entry, its stored state roll-up, and
//
//	the custom rows the operator declared for it.
//
// @uses      internal/domain, internal/registry, context, strings.
// @reason    SPEC-API-001 §7.4 serves one provider's detail, and §7.14 makes
//
//	that body the union of the levels its models accept. A synthesized
//	custom node carries no registry models, so the union needs the rows
//	the operator declared, which live behind the model catalog's own
//	store; reading them here is what keeps the schema layer a projection
//	and the handler a decode-call-encode, and it is why this read sits in
//	its own file rather than in provider.go, whose list and search reads
//	are a separate concern and whose budget §1.1 caps.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-27
package service

import (
	"context"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// CustomModelLister answers the custom model rows declared for one provider.
// *ModelCatalogService satisfies it directly, so the wiring writes no adapter
// and the two services share one read of the rows instead of one each.
type CustomModelLister interface {
	Custom(ctx context.Context, providerID string) ([]domain.CustomModel, error)
}

// Detail returns one provider's entry, roll-up, and declared rows.
//
// The rows are read for every provider rather than only for a node: a registry
// provider's declared rows are models a client can call, so they belong in the
// same level union its catalog contributes to (§7.14). A lister that is not
// wired answers no rows, which is the state before custom models existed.
func (s *ProviderService) Detail(ctx context.Context, providerID string) (ProviderRow, error) {
	entry, ok := s.index.Provider(strings.TrimSpace(providerID))
	if !ok {
		return ProviderRow{}, domain.NewNotFoundError("provider is not in the registry")
	}
	summaries, err := s.summariesFor(ctx, []registry.Provider{entry})
	if err != nil {
		return ProviderRow{}, err
	}
	declared, err := s.declaredModels(ctx, entry)
	if err != nil {
		return ProviderRow{}, err
	}
	return ProviderRow{Entry: entry, Summary: summaries[entry.ID], CustomModels: declared}, nil
}

// declaredModels reads the rows the operator declared for this provider.
//
// A read failure is returned rather than degraded to an empty list: the detail
// route already fails when its endpoint roll-up cannot be read, and an empty
// list is indistinguishable from "this provider declares none", which would
// hide the reasoning picker and report the provider as having no custom models
// at all. An empty answer is a fact; a failed read is not.
func (s *ProviderService) declaredModels(ctx context.Context, entry registry.Provider) ([]domain.CustomModel, error) {
	if s.custom == nil {
		return nil, nil
	}
	return s.custom.Custom(ctx, entry.ID)
}
