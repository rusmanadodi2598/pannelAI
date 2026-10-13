// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_ports.go
// @for       The seams selection reads: the rotation cursor and its key space, the candidate bound, the budget gate, and the two policy readers.
// @uses      context, crypto/sha256, encoding/hex, internal/domain.
// @reason    AGENTS.md §1.5 keeps a Redis driver and the quota repository out of the data plane through one-method interfaces. Naming those seams together, with the degradations each one documents, is what makes an unwired or failing seam one read rather than five scattered above the selector.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-13
package dataplane

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// CursorKeyPrefix is the Redis key space for the round-robin endpoint cursor.
// The cursor is the only selection state Redis holds: endpoint availability and
// key health live in the domain and in PostgreSQL, so losing this namespace
// degrades rotation to priority order without changing which account is usable.
const CursorKeyPrefix = "pannelai:dataplane:endpoint-cursor:"

// MaxEndpointsPerProvider bounds the candidate list. A provider with more
// accounts than this is a configuration mistake, and the bound keeps selection
// from reading an unbounded result set (AGENTS.md §1.7).
const MaxEndpointsPerProvider = 100

// CursorStore advances the round-robin position over one provider's endpoints.
// It is an interface because the data plane must not import a Redis client
// (AGENTS.md §1.5 keeps driver access in the repository layer), so the
// composition root adapts the Redis implementation to it.
type CursorStore interface {
	// NextOffset returns the zero-based offset to start selecting at, then moves
	// the cursor on. A sticky budget keeps one offset for a bounded number of
	// calls, so one account serves a short burst instead of the pool
	// alternating on every request.
	NextOffset(ctx context.Context, providerID string, size int, stickyLimit int) (int, error)
}

// CursorKey returns the Redis key the cursor for one provider occupies. The
// provider id is hashed so an id with odd characters cannot collide with
// another key in the namespace.
func CursorKey(providerID string) string {
	digest := sha256.Sum256([]byte(providerID))
	return CursorKeyPrefix + hex.EncodeToString(digest[:])
}

// BudgetGate reports whether an endpoint has already spent its budget. It is a
// one-method seam because selection asks exactly one question, and it keeps the
// router from importing the quota repository (AGENTS.md §1.5).
//
// The second return being false means "allowed", which is also what an endpoint
// with no cap answers: an uncapped endpoint is not an exhausted one.
type BudgetGate interface {
	// Exhausted reports whether the endpoint's month-to-date spend has reached
	// its stored budget cap (SPEC-API-001 §7.12).
	Exhausted(ctx context.Context, endpointID string) (bool, error)
}

// CredentialStrategy answers the credential rotation policy for one provider
// (SPEC-API-001 §7.5): fill-first, or round-robin with a sticky limit. It is a
// one-question seam, and it keeps the data plane from importing the settings
// service (AGENTS.md §1.5). A policy that cannot be read degrades to fill-first
// rather than failing the request, because rotation is an optimisation.
type CredentialStrategy interface {
	RotationPolicy(ctx context.Context, providerID string) (domain.RotationPolicy, error)
}

// KeyHealthPolicyReader answers the parking windows configured for one provider
// (§7.5, §7.14). It is a one-question seam for the same reason the rotation seam
// is: the data plane must not import the settings service (AGENTS.md §1.5).
type KeyHealthPolicyReader interface {
	KeyHealthPolicy(ctx context.Context, providerID string) (domain.KeyHealthPolicy, error)
}
