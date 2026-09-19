// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/rotation.go
// @for       The round_robin combo strategy's distribution: which member leads
//
//	this request (SPEC-API-001 §7.7).
//
// @uses      internal/domain, context.
// @reason    The distribution rule lives in the domain and the atomic counter in
//
//	Redis, so the engine's part is only to ask for the order and to fall
//	back when the store cannot answer. Keeping that in one function is
//	what makes "rotation is an optimisation, not a correctness input"
//	true for every caller instead of a promise each one re-implements.
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

// ComboOrderer returns the model order one request should try for a combo. The
// round-robin rule and the state it advances live behind it, so the engine
// neither reads the rotation state nor knows where it is stored. It is an
// interface because the data plane must not import the management service that
// owns the rule (AGENTS.md §1.5), and *service.ComboService satisfies it
// directly — no adapter is needed.
type ComboOrderer interface {
	// Order returns the order this request uses, which for a round_robin combo
	// advances the counter every replica shares.
	Order(ctx context.Context, combo domain.Combo) ([]string, error)
}

// rotate applies the round_robin strategy's distribution to a combo's members.
//
// A failure falls back to the stored priority order rather than failing the
// request: rotation is an optimisation, and a Redis outage must not take the
// data plane down. An order of a different length is treated the same way,
// because it cannot address this combo's members.
func (e *Engine) rotate(ctx context.Context, resolution Resolution, refs []string) []string {
	if e.orders == nil || !resolution.Combo.Strategy().UsesRotation() || len(refs) < 2 {
		return refs
	}
	order, err := e.orders.Order(ctx, resolution.Combo)
	if err != nil || len(order) != len(refs) {
		return refs
	}
	return order
}
