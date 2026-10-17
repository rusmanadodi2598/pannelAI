// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_bulk_delete.go
// @for       The bulk delete and bulk retag actions over the set one selector resolves to (SPEC-API-001 §7.5, §8.1).
// @uses      context, time, internal/domain.
// @reason    Deleting and retagging are the two bulk actions that cannot be undone by the panel, so they resolve through the same counted guard the park uses and write through the aggregate rather than a column patch: the tags an operator adds and removes are the endpoint's own state, and a rule that lives on the root stops working the moment a second caller writes the column beside it.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-16
package service

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// BulkDelete removes every account the selector names, keys and all, in one
// transaction.
//
// The set is resolved and counted before the first row goes, so a selector that no longer
// matches what the caller confirmed deletes nothing at all rather than deleting the rows
// that happen to remain. Keys follow the cascade the schema declares; no history is
// touched, which is the rule the single-delete route already holds and a batch cannot
// renegotiate.
func (s *EndpointService) BulkDelete(ctx context.Context, sel BulkSelector, confirmCount int) (BulkOutcome, error) {
	endpoints, err := s.resolveBulk(ctx, sel, confirmCount)
	if err != nil {
		return BulkOutcome{}, err
	}
	ids := make([]string, len(endpoints))
	outcome := BulkOutcome{Matched: len(endpoints), Changed: len(endpoints), Results: make([]BulkRowResult, 0, len(endpoints))}
	for i, endpoint := range endpoints {
		ids[i] = endpoint.ID()
		outcome.Results = append(outcome.Results, BulkRowResult{Index: i, ID: endpoint.ID()})
	}
	if len(ids) == 0 {
		return outcome, nil
	}
	if err := s.store.DeleteBulk(ctx, ids); err != nil {
		return BulkOutcome{}, err
	}
	return outcome, nil
}

// BulkTags adds, removes, or clears tags across the resolved set in one transaction.
//
// `clear` is a separate intent from an empty remove list because clearing every label and
// naming none to remove are different instructions, and a caller should not have to send
// a sentinel to say which. A row already carrying a tag is answered as an unchanged row:
// the write skips it, so the count the operator sees is the count that moved.
func (s *EndpointService) BulkTags(ctx context.Context, sel BulkSelector, add, remove []string, clear bool, confirmCount int) (BulkOutcome, error) {
	if !clear && len(add) == 0 && len(remove) == 0 {
		return BulkOutcome{}, domain.NewValidationError("add, remove, or clear names the tag change to make")
	}
	endpoints, err := s.resolveBulk(ctx, sel, confirmCount)
	if err != nil {
		return BulkOutcome{}, err
	}

	now := s.clock()
	changed := make([]domain.UpstreamEndpoint, 0, len(endpoints))
	outcome := BulkOutcome{Matched: len(endpoints), Results: make([]BulkRowResult, 0, len(endpoints))}
	for i, endpoint := range endpoints {
		before := len(endpoint.Tags())
		if clear {
			endpoint.ClearTags(now)
		}
		changedRow, err := applyTagDiff(&endpoint, add, remove, now)
		if err != nil {
			outcome.Results = append(outcome.Results, BulkRowResult{Index: i, ID: endpoint.ID(), Error: err})
			continue
		}
		outcome.Results = append(outcome.Results, BulkRowResult{Index: i, ID: endpoint.ID()})
		if changedRow || len(endpoint.Tags()) != before {
			changed = append(changed, endpoint)
		}
	}
	if len(changed) > 0 {
		if err := s.store.UpdateBulk(ctx, changed); err != nil {
			return BulkOutcome{}, err
		}
	}
	outcome.Changed = len(changed)
	return outcome, nil
}

// applyTagDiff runs one row's tag changes and answers whether any of them moved the set.
// A malformed tag is refused for that row alone: the operator named a label that cannot
// be stored, and the other rows in the set are not theirs to lose.
func applyTagDiff(endpoint *domain.UpstreamEndpoint, add, remove []string, now time.Time) (bool, error) {
	moved := false
	for _, tag := range add {
		added, err := endpoint.AddTag(tag, now)
		if err != nil {
			return false, err
		}
		moved = moved || added
	}
	for _, tag := range remove {
		removed, err := endpoint.RemoveTag(tag, now)
		if err != nil {
			return false, err
		}
		moved = moved || removed
	}
	return moved, nil
}
