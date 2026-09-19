// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_health.go
// @for       The health accounting a selection produces: the key circuit a
//
//	served or failed call updates, and its persistence.
//
// @uses      internal/domain, context.
// @reason    Selection and accounting are two jobs: Select decides who serves
//
//	the next request from the endpoint's current state, while these
//	writes change that state afterwards. Keeping them apart also keeps
//	the keyless rule readable, because a no_auth selection has no key
//	circuit to write and must not fail the request on a bookkeeping
//	error (G15 in the P2 register).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// RecordSuccess applies a served request to the key and persists its health, so
// the circuit the domain owns is what changes and nothing else does. A keyless
// selection (a no_auth endpoint) has no key circuit to write and is a no-op.
func (s *Selector) RecordSuccess(ctx context.Context, selection Selection) error {
	if selection.Key.ID() == "" {
		return nil
	}
	updated, err := selection.Endpoint.RecordKeySuccess(selection.Key.ID(), s.clock())
	if err != nil {
		return err
	}
	return s.endpoints.RecordKeyHealth(ctx, updated)
}

// RecordFailure applies a failed attempt to the key and persists its health, so
// the circuit the domain owns is what changes and nothing else does. A keyless
// selection has no key to trip, so the write is skipped rather than failing the
// request on a bookkeeping error.
func (s *Selector) RecordFailure(ctx context.Context, selection Selection, reason string) error {
	if selection.Key.ID() == "" {
		return nil
	}
	updated, err := selection.Endpoint.RecordKeyFailure(selection.Key.ID(), reason, s.clock())
	if err != nil {
		return err
	}
	return s.endpoints.RecordKeyHealth(ctx, updated)
}

// PersistKey records a key whose health was already updated in memory, so a
// failure discovered after the response body was read needs no second mutation.
func (s *Selector) PersistKey(ctx context.Context, key domain.UpstreamKey) error {
	return s.endpoints.RecordKeyHealth(ctx, key)
}
