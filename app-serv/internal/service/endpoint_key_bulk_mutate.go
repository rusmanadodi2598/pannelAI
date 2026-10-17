// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_key_bulk_mutate.go
// @for       The bulk retag and bulk delete over the credentials one selector resolves to (SPEC-API-001 §7.5, §8.1).
// @uses      context, time, internal/domain.
// @reason    Deleting a credential is the one bulk action that can strand an account with nothing to authenticate with, and the rule that prevents it lives on the account, not the key. Applying each row through the account root, in the order the operator sees them, is what makes the refusal land on the row that would have caused it rather than on the last one to arrive.
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

// BulkKeyTags adds, removes, or clears tags across the resolved credentials in one
// transaction.
func (s *EndpointService) BulkKeyTags(ctx context.Context, sel BulkSelector, add, remove []string, clear bool, confirmCount int) (BulkOutcome, error) {
	if !clear && len(add) == 0 && len(remove) == 0 {
		return BulkOutcome{}, domain.NewValidationError("add, remove, or clear names the tag change to make")
	}
	accounts, refs, err := s.resolveBulkKeys(ctx, sel, confirmCount)
	if err != nil {
		return BulkOutcome{}, err
	}

	now := s.clock()
	touched := make(map[int]struct{}, len(refs))
	keys := make([]domain.UpstreamKey, 0, len(refs))
	outcome := BulkOutcome{Matched: len(refs), Results: make([]BulkRowResult, 0, len(refs))}
	for i, ref := range refs {
		endpoint := &accounts[ref.account]
		before := len(keyOf(*endpoint, ref.keyID).Tags())
		moved, err := applyKeyTagDiff(endpoint, ref.keyID, add, remove, clear, now)
		if err != nil {
			outcome.Results = append(outcome.Results, BulkRowResult{Index: i, ID: ref.keyID, Error: err})
			continue
		}
		outcome.Results = append(outcome.Results, BulkRowResult{Index: i, ID: ref.keyID})
		if moved || len(keyOf(*endpoint, ref.keyID).Tags()) != before {
			touched[ref.account] = struct{}{}
			keys = append(keys, keyOf(*endpoint, ref.keyID))
		}
	}
	outcome.Changed = len(keys)
	if len(keys) == 0 {
		return outcome, nil
	}
	if err := s.store.ApplyKeyChanges(ctx, changedAccounts(accounts, touched), keys, nil); err != nil {
		return BulkOutcome{}, err
	}
	return outcome, nil
}

// applyKeyTagDiff runs one credential's tag changes through its account and answers
// whether any of them moved the set.
func applyKeyTagDiff(endpoint *domain.UpstreamEndpoint, keyID string, add, remove []string, clear bool, now time.Time) (bool, error) {
	if clear {
		if err := endpoint.ClearKeyTags(keyID, now); err != nil {
			return false, err
		}
	}
	moved := false
	for _, tag := range add {
		added, err := endpoint.AddKeyTag(keyID, tag, now)
		if err != nil {
			return false, err
		}
		moved = moved || added
	}
	for _, tag := range remove {
		removed, err := endpoint.RemoveKeyTag(keyID, tag, now)
		if err != nil {
			return false, err
		}
		moved = moved || removed
	}
	return moved, nil
}

// BulkKeyDelete removes every credential the selector names, in one transaction.
//
// The rows go in the order the selector resolved them, and each removal is decided by the
// account that owns the credential: a set that would leave a key-backed account with no
// usable credential is refused on the row that does it, and every other row in that
// account still lands because the transaction only aborts on a store failure, not on a
// refusal the operator can read.
func (s *EndpointService) BulkKeyDelete(ctx context.Context, sel BulkSelector, confirmCount int) (BulkOutcome, error) {
	accounts, refs, err := s.resolveBulkKeys(ctx, sel, confirmCount)
	if err != nil {
		return BulkOutcome{}, err
	}

	now := s.clock()
	touched := make(map[int]struct{}, len(refs))
	deleted := make([]string, 0, len(refs))
	outcome := BulkOutcome{Matched: len(refs), Results: make([]BulkRowResult, 0, len(refs))}
	for i, ref := range refs {
		endpoint := &accounts[ref.account]
		if err := endpoint.RemoveKey(ref.keyID, now); err != nil {
			outcome.Results = append(outcome.Results, BulkRowResult{Index: i, ID: ref.keyID, Error: err})
			continue
		}
		outcome.Results = append(outcome.Results, BulkRowResult{Index: i, ID: ref.keyID})
		touched[ref.account] = struct{}{}
		deleted = append(deleted, ref.keyID)
	}
	outcome.Changed = len(deleted)
	if len(deleted) == 0 {
		return outcome, nil
	}
	if err := s.store.ApplyKeyChanges(ctx, changedAccounts(accounts, touched), nil, deleted); err != nil {
		return BulkOutcome{}, err
	}
	return outcome, nil
}
