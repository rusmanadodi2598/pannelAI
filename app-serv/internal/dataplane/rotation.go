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

// RotationStore advances a round-robin combo's position. It is an interface
// because the data plane must not import a Redis client (AGENTS.md §1.5), so
// the composition root adapts the stored implementation to it.
type RotationStore interface {
	// Next returns the member order this request uses, advancing the counter
	// every replica shares.
	Next(ctx context.Context, comboKey string, models []string, stickyLimit int) ([]string, error)
}

// rotate applies the round_robin strategy's distribution to a combo's members.
//
// A failure falls back to the stored priority order rather than failing the
// request: rotation is an optimisation, and a Redis outage must not take the
// data plane down. A store answering with a different length is treated the
// same way, because an order of another length cannot address this combo's
// members.
func (e *Engine) rotate(ctx context.Context, resolution Resolution, refs []string) []string {
	if e.rotation == nil || resolution.ComboStrategy != domain.ComboRoundRobin || len(refs) < 2 {
		return refs
	}
	order, err := e.rotation.Next(ctx, resolution.Combo, refs, resolution.ComboStickyLimit)
	if err != nil || len(order) != len(refs) {
		return refs
	}
	return order
}
