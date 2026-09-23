// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/model_resolve_disabled.go
// @for       The disabled-set reads resolution performs, and the available-pair
//
//	view a set replacement validates against.
//
// @uses      internal/domain, context.
// @reason    The disabled comparison is canonical on both sides (draft 024
//
//	F2): a pair stored under the id form must also hide the alias and
//	prefix spellings of the same model, because the set is a judgement
//	about the model, not about one of its names. Separated from the
//	resolution entry point at the AGENTS.md §1.1 line limit.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-23
package service

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// isDisabled reports whether a pair is in the disabled set. The comparison is
// canonical on both sides — the reference's spellings and the stored pair's
// spellings — because the disabled set is a judgement about the model, not
// about one of its names: a pair stored under a node prefix hides the node-id
// spelling too, and a pair stored under the id hides the alias spelling.
func (s *ModelCatalogService) isDisabled(ctx context.Context, ref domain.ModelRef) (bool, error) {
	disabled, err := s.repo.Disabled(ctx)
	if err != nil {
		return false, err
	}
	if len(disabled) == 0 {
		return false, nil
	}
	names := providerNames(s.index)
	asked := refSpellings(names, ref)
	for _, blocked := range disabled {
		if _, hit := asked[blocked.String()]; hit {
			return true, nil
		}
		stored := refSpellings(names, blocked)
		if _, hit := stored[ref.String()]; hit {
			return true, nil
		}
	}
	return false, nil
}

// refSpellings expands one reference into every key the same model is
// addressable by: its own spelling plus the canonical-id and alias spellings of
// its provider segment. Both directions of the rename are covered by including
// the stored pair's aliases as well, which is why the caller feeds each stored
// pair through this expansion rather than comparing raw strings.
func refSpellings(names map[string]registry.Provider, ref domain.ModelRef) map[string]struct{} {
	spellings := map[string]struct{}{ref.String(): {}}
	entry, ok := names[ref.ProviderID()]
	if ok {
		spellings[entry.ID+"/"+ref.ModelID()] = struct{}{}
		for _, alias := range providerAliasNames(entry) {
			spellings[alias+"/"+ref.ModelID()] = struct{}{}
		}
		return spellings
	}
	// A provider the index no longer holds still has its raw spelling, which is
	// what a stored pair naming it would compare against.
	return spellings
}

// availableRefs returns every pair the catalog holds, disabled rows included.
func (s *ModelCatalogService) availableRefs(ctx context.Context) (map[string]struct{}, error) {
	lookups, err := s.lookups(ctx)
	if err != nil {
		return nil, err
	}
	disabled, err := s.repo.Disabled(ctx)
	if err != nil {
		return nil, err
	}
	available := make(map[string]struct{}, len(lookups)+len(disabled))
	for key := range lookups {
		available[key] = struct{}{}
	}
	for _, ref := range disabled {
		available[ref.String()] = struct{}{}
	}
	return available, nil
}

// isNotFound reports whether an error is a NOT_FOUND AppError, so a caller can
// treat "absent" differently from "the lookup failed".
func isNotFound(err error) bool {
	return err != nil && domain.AsAppError(err).HTTPStatus() == 404
}
