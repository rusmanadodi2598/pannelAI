// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/gateway_key.go
// @for       PostgreSQL persistence for the GatewayKey aggregate root.
// @uses      github.com/jackc/pgx/v5, internal/domain, internal/repository.
// @reason    SPEC-API-001 §6 defines gateway_keys (SHA-256 digest lookup, hint,
//
//	soft revocation) and AGENTS.md §1.7 requires explicit pool limits,
//	indexed lookups, and no unbounded queries; this is the only place
//	SQL appears for this aggregate.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtechstore.com>
// @layer     repository
// @stability experimental
// @since     2026-09-16
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// gatewayKeyColumns is the projection every read uses, in scan order.
const gatewayKeyColumns = `id, name, value_hash, key_hint, status,
	last_used_at, request_count, created_at, revoked_at`

// GatewayKeyRepository persists gateway keys in PostgreSQL.
type GatewayKeyRepository struct {
	pool *pgxpool.Pool
}

// NewGatewayKeyRepository binds the repository to a pool whose limits are set
// explicitly at construction (AGENTS.md §1.7).
func NewGatewayKeyRepository(pool *pgxpool.Pool) *GatewayKeyRepository {
	return &GatewayKeyRepository{pool: pool}
}

// Create inserts a newly issued key. A duplicate name maps to
// domain.ErrGatewayKeyExists so callers return 409, not a driver message.
func (r *GatewayKeyRepository) Create(ctx context.Context, key domain.GatewayKey) error {
	const q = `
INSERT INTO gateway_keys
    (id, name, value_hash, key_hint, status, request_count, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)`

	tag, err := r.pool.Exec(ctx, q,
		key.ID(), key.Name(), key.ValueHash(), key.KeyHint(),
		string(key.Status()), key.RequestCount(), key.CreatedAt())
	if err != nil {
		return translatePGError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.NewInternalError("gateway key was not stored")
	}
	return nil
}

// List returns one page of keys newest-first. The total comes from a window
// function in the same statement, so the count and the page always describe the
// same snapshot; running them as two queries would let a concurrent insert make
// `total` disagree with the rows returned (SPEC-API-001 §4).
func (r *GatewayKeyRepository) List(ctx context.Context, q repository.PageQuery) ([]domain.GatewayKey, int64, error) {
	const query = `
SELECT ` + gatewayKeyColumns + `, count(*) OVER() AS total
  FROM gateway_keys
 ORDER BY created_at DESC, id DESC
 LIMIT $1 OFFSET $2`

	rows, err := r.pool.Query(ctx, query, q.PerPage, q.Offset())
	if err != nil {
		return nil, 0, translatePGError(err)
	}
	defer rows.Close()

	keys := make([]domain.GatewayKey, 0, q.PerPage)
	var total int64
	for rows.Next() {
		key, rowTotal, err := scanGatewayKeyWithTotal(rows)
		if err != nil {
			return nil, 0, err
		}
		total = rowTotal
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, translatePGError(err)
	}

	// A page beyond the last row returns no rows, and therefore no window
	// count; the total is then read separately so pagination still reports it.
	if len(keys) == 0 {
		if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM gateway_keys`).Scan(&total); err != nil {
			return nil, 0, translatePGError(err)
		}
	}
	return keys, total, nil
}

// GetByID loads one key. A missing row yields domain.ErrGatewayKeyNotFound.
func (r *GatewayKeyRepository) GetByID(ctx context.Context, id string) (domain.GatewayKey, error) {
	const q = `SELECT ` + gatewayKeyColumns + ` FROM gateway_keys WHERE id = $1`

	key, err := scanGatewayKey(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		return domain.GatewayKey{}, translatePGError(err)
	}
	return key, nil
}

// Update persists the mutable fields a PATCH may change.
func (r *GatewayKeyRepository) Update(ctx context.Context, key domain.GatewayKey) error {
	const q = `
UPDATE gateway_keys
   SET name = $1, status = $2, updated_at = now()
 WHERE id = $3`

	tag, err := r.pool.Exec(ctx, q, key.Name(), string(key.Status()), key.ID())
	if err != nil {
		return translatePGError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrGatewayKeyNotFound
	}
	return nil
}

// Revoke applies a terminal revocation, setting status and revoked_at.
func (r *GatewayKeyRepository) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	const q = `
UPDATE gateway_keys
   SET status = 'revoked', revoked_at = $1, updated_at = now()
 WHERE id = $2`

	tag, err := r.pool.Exec(ctx, q, revokedAt, id)
	if err != nil {
		return translatePGError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrGatewayKeyNotFound
	}
	return nil
}

// scanner is the read surface both QueryRow and Rows satisfy; its variadic
// destination is the pgx decode boundary (AGENTS.md §1.4).
type scanner interface {
	Scan(dest ...any) error
}

// scanGatewayKey reads one row into a rehydrated aggregate.
func scanGatewayKey(s scanner) (domain.GatewayKey, error) {
	var (
		id, name, valueHash, keyHint, status string
		lastUsedAt, revokedAt                *time.Time
		requestCount                         int64
		createdAt                            time.Time
	)
	err := s.Scan(&id, &name, &valueHash, &keyHint, &status,
		&lastUsedAt, &requestCount, &createdAt, &revokedAt)
	if err != nil {
		return domain.GatewayKey{}, err
	}
	return domain.RehydrateGatewayKey(id, name, valueHash, keyHint,
		domain.GatewayKeyStatus(status), lastUsedAt, requestCount, createdAt, revokedAt), nil
}

// scanGatewayKeyWithTotal reads a List row: the key plus the window count.
func scanGatewayKeyWithTotal(s scanner) (domain.GatewayKey, int64, error) {
	var (
		id, name, valueHash, keyHint, status string
		lastUsedAt, revokedAt                *time.Time
		requestCount, total                  int64
		createdAt                            time.Time
	)
	err := s.Scan(&id, &name, &valueHash, &keyHint, &status,
		&lastUsedAt, &requestCount, &createdAt, &revokedAt, &total)
	if err != nil {
		return domain.GatewayKey{}, 0, err
	}
	key := domain.RehydrateGatewayKey(id, name, valueHash, keyHint,
		domain.GatewayKeyStatus(status), lastUsedAt, requestCount, createdAt, revokedAt)
	return key, total, nil
}

// translatePGError maps a driver error to a domain error a caller can act on.
// Unrecognised errors are wrapped with table context; the wrapped text reaches
// logs only, because the handler renders domain.Message, never the chain
// (AGENTS.md §1.3).
func translatePGError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrGatewayKeyNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return fmt.Errorf("%w: %s", domain.ErrGatewayKeyExists, pgErr.ConstraintName)
		}
	}
	return fmt.Errorf("gateway_keys: %w", err)
}
