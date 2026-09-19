// Package repository is the storage boundary app-serv services depend on.
//
// @file      internal/repository/gateway_key.go
// @for       The GatewayKeyRepository contract: load and persist gateway keys.
// @uses      internal/domain.
// @reason    AGENTS.md §1.5 requires services to depend on this interface and
//
//	never on a driver, and §2.2 requires repositories to save
//	aggregate roots (gateway_keys is the aggregate for credentials).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-16
package repository

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// PageQuery is the bounded pagination every list repository accepts
// (AGENTS.md §1.7: no unbounded queries in request-serving paths).
type PageQuery struct {
	Page    int
	PerPage int
}

// Offset is the row offset for the current page.
func (p PageQuery) Offset() int { return (p.Page - 1) * p.PerPage }

// GatewayKeyRepository is the storage boundary for gateway keys (SPEC-API-001
// §7.3). Implementations live under internal/repository/postgres; service code
// depends only on this interface (AGENTS.md §1.5 layer flow).
type GatewayKeyRepository interface {
	// Create persists a newly issued key. It must reject a duplicate name with
	// domain.ErrGatewayKeyExists so the service can map it to CONFLICT.
	Create(ctx context.Context, key domain.GatewayKey) error

	// List returns one page of keys, newest first, with the total count the
	// response meta block needs.
	List(ctx context.Context, q PageQuery) (keys []domain.GatewayKey, total int64, err error)

	// GetByID loads a single key. A missing row must yield
	// domain.ErrGatewayKeyNotFound so callers map it to 404.
	GetByID(ctx context.Context, id string) (domain.GatewayKey, error)

	// GetByValueHash loads a key by its SHA-256 digest. The data plane presents
	// a plaintext bearer token, hashes it, and looks the key up here, so the
	// stored digest is what authenticates a request (SPEC-API-001 §4). A
	// missing row must yield domain.ErrGatewayKeyNotFound.
	GetByValueHash(ctx context.Context, valueHash string) (domain.GatewayKey, error)

	// Update persists mutable field changes (name, status).
	Update(ctx context.Context, key domain.GatewayKey) error

	// RecordUse applies one authenticated data-plane call to the key: the
	// request counter advances by one and last_used_at is stamped (SPEC-API-001
	// §7.3). A missing row must yield domain.ErrGatewayKeyNotFound.
	RecordUse(ctx context.Context, id string, usedAt time.Time) error

	// Revoke applies a terminal revocation: it sets status=revoked and the
	// revoked_at instant, and must no-op cleanly when the key is already
	// revoked (the domain layer rejects a double-revoke first).
	Revoke(ctx context.Context, id string, revokedAt time.Time) error
}
