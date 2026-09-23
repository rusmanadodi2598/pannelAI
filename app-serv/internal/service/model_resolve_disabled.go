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
)

// isDisabled reports whether a pair is in the disabled set. The comparison is
// canonical on both sides: a pair stored under the id form must also hide the
// alias and prefix spellings of the same model, because the disabled set is a
// judgement about the model, not about one of its names.
func (s *ModelCatalogService) isDisabled(ctx context.Context, ref domain.ModelRef) (bool, error) {
	disabled, err := s.repo.Disabled(ctx)
	if err != nil {
		return false, err
	}
	canonical, _ := canonicalRef(s.index, ref)
	for _, blocked := range disabled {
		if blocked.String() == ref.String() || blocked.String() == canonical.String() {
			return true, nil
		}
	}
	return false, nil
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
