// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_key_bulk.go
// @for       The bulk actions that address credentials directly: park, retag, and delete the keys one selector resolves to (SPEC-API-001 §7.5, §8.1).
// @uses      context, fmt, slices, time, internal/domain, internal/repository.
// @reason    Keys are reached through the account that owns them, so a bulk action on two hundred credentials is also two hundred looks at the "keep one usable credential" rule. Walking the aggregates in one pass is what enforces that rule over the set the action is making rather than over the snapshot each row was read from: disable two keys of one account and the second refusal is real, not a stale count.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-16
package service

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// bulkKeyRef is one targeted credential, remembered by the account it was found under so
// the write goes through that account's aggregate.
type bulkKeyRef struct {
	account int
	keyID   string
}

// resolveBulkKeys loads the accounts the selector narrows, then the credentials it names
// inside them.
//
// A key selector's tag and id list are read against the credential, not the account: a
// label on a key must not exclude the account that holds it, and the account is the only
// way in. The account count is bounded the same way a bulk action on accounts is, because
// reaching keys means loading their owners whole.
func (s *EndpointService) resolveBulkKeys(ctx context.Context, sel BulkSelector, confirmCount int) ([]domain.UpstreamEndpoint, []bulkKeyRef, error) {
	if !sel.HasTargets() {
		return nil, nil, domain.NewBulkSelectorRequiredError(
			"select credentials by ids, name one tag, or send all=true")
	}
	filter := repository.EndpointFilter{ProviderID: sel.ProviderID, AuthType: string(sel.AuthType)}
	perPage := bulkResolvePerPage
	if len(sel.IDs) > 0 && len(sel.IDs) < perPage {
		perPage = len(sel.IDs)
	}
	page, accounts, err := s.store.List(ctx, filter, repository.PageQuery{Page: 1, PerPage: perPage})
	if err != nil {
		return nil, nil, err
	}
	if accounts > MaxBulkTargets {
		return nil, nil, domain.NewBulkTargetTooLargeError(fmt.Sprintf(
			"this selector reaches %d accounts, and one action may address at most %d; narrow it to a provider",
			accounts, MaxBulkTargets))
	}
	resolved := make([]domain.UpstreamEndpoint, 0, int(accounts))
	seen := make(map[string]struct{}, int(accounts))
	for _, endpoint := range page {
		seen[endpoint.ID()] = struct{}{}
		resolved = append(resolved, endpoint)
	}
	for pageNo := 2; int64(len(resolved)) < accounts; pageNo++ {
		next, _, err := s.store.List(ctx, filter, repository.PageQuery{Page: pageNo, PerPage: perPage})
		if err != nil {
			return nil, nil, err
		}
		if len(next) == 0 {
			break
		}
		for _, endpoint := range next {
			if _, dup := seen[endpoint.ID()]; dup {
				continue
			}
			seen[endpoint.ID()] = struct{}{}
			resolved = append(resolved, endpoint)
		}
	}

	wanted := make(map[string]struct{}, len(sel.IDs))
	for _, id := range sel.IDs {
		wanted[id] = struct{}{}
	}
	refs := make([]bulkKeyRef, 0, len(sel.IDs))
	for i, endpoint := range resolved {
		for _, key := range endpoint.Keys() {
			if !keySelected(key, sel, wanted) {
				continue
			}
			refs = append(refs, bulkKeyRef{account: i, keyID: key.ID()})
		}
	}
	if len(refs) > MaxBulkTargets {
		return nil, nil, domain.NewBulkTargetTooLargeError(fmt.Sprintf(
			"this selector resolves to %d credentials, and one action may address at most %d",
			len(refs), MaxBulkTargets))
	}
	if confirmCount != len(refs) {
		return nil, nil, domain.NewBulkTargetCountMismatchError(fmt.Sprintf(
			"the selector now resolves to %d credentials, not the %d this action confirmed; refresh the list and repeat",
			len(refs), confirmCount))
	}
	return resolved, refs, nil
}

// keySelected answers whether one credential is inside the selector's set.
func keySelected(key domain.UpstreamKey, sel BulkSelector, wanted map[string]struct{}) bool {
	if len(wanted) > 0 {
		_, ok := wanted[key.ID()]
		return ok
	}
	if sel.Tag != "" {
		return slices.Contains(key.Tags(), sel.Tag)
	}
	return sel.All
}

// BulkKeyStatus parks or releases every credential the selector names.
//
// Each key's window is sized by its own account's provider, because a bulk action can
// cross providers and the cadence that releases a parked credential is the vendor's, not
// the click's.
func (s *EndpointService) BulkKeyStatus(ctx context.Context, sel BulkSelector, status string, windowHours int, confirmCount int) (BulkOutcome, error) {
	parsed, err := domain.ParseUpstreamKeyStatus(status)
	if err != nil {
		return BulkOutcome{}, err
	}
	if parsed == domain.UpstreamKeyError {
		return BulkOutcome{}, domain.NewValidationError("health tracking owns the error state, not this action")
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
		before := readKeyState(*endpoint, ref.keyID)
		if parsed == domain.UpstreamKeyDisabled {
			err = endpoint.DisableKey(ref.keyID, s.disableDeadline(ctx, endpoint.ProviderID(), windowHours, now), now)
		} else {
			err = endpoint.EnableKey(ref.keyID, now)
		}
		if err != nil {
			outcome.Results = append(outcome.Results, BulkRowResult{Index: i, ID: ref.keyID, Error: err})
			continue
		}
		outcome.Results = append(outcome.Results, BulkRowResult{Index: i, ID: ref.keyID})
		if readKeyState(*endpoint, ref.keyID) != before {
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

// keyState is what a credential transition can move: which way the switch is set and the
// deadline it carries. Comparing it keeps a bulk action from writing rows already in the
// state asked for.
type keyState struct {
	status domain.UpstreamKeyStatus
	until  time.Time
}

func readKeyState(endpoint domain.UpstreamEndpoint, keyID string) keyState {
	key := keyOf(endpoint, keyID)
	state := keyState{status: key.Status()}
	if until := key.DisabledUntil(); until != nil {
		state.until = *until
	}
	return state
}

// keyOf reads one credential back out of its account.
func keyOf(endpoint domain.UpstreamEndpoint, keyID string) domain.UpstreamKey {
	for _, key := range endpoint.Keys() {
		if key.ID() == keyID {
			return key
		}
	}
	return domain.UpstreamKey{}
}

// changedAccounts projects the accounts a bulk key action moved back into the order the
// store receives them in, without duplicates.
func changedAccounts(accounts []domain.UpstreamEndpoint, touched map[int]struct{}) []domain.UpstreamEndpoint {
	out := make([]domain.UpstreamEndpoint, 0, len(touched))
	for i := range accounts {
		if _, ok := touched[i]; ok {
			out = append(out, accounts[i])
		}
	}
	return out
}
