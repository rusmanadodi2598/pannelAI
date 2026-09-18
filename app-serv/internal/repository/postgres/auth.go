// Package postgres implements PostgreSQL repositories for app-serv.
//
// @file      internal/repository/postgres/auth.go
// @for       Persists and atomically updates the singleton dashboard password hash.
// @uses      pgx/v5/pgxpool, internal/domain, internal/repository.
// @reason    Dashboard authentication needs durable bcrypt hash storage with a
//
//	compare-and-set update so concurrent password changes are safe.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-09-17
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// AuthRepository stores the singleton dashboard authentication row.
type AuthRepository struct {
	pool *pgxpool.Pool
}

// NewAuthRepository constructs a PostgreSQL auth repository.
func NewAuthRepository(pool *pgxpool.Pool) *AuthRepository {
	return &AuthRepository{pool: pool}
}

// PasswordHash returns the configured bcrypt hash, or an empty string before bootstrap.
func (r *AuthRepository) PasswordHash(ctx context.Context) (string, error) {
	var hash *string
	if err := r.pool.QueryRow(ctx, `SELECT password_hash FROM panel_auth WHERE id = 1`).Scan(&hash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("auth row missing: %w", domain.ErrPasswordNotConfigured)
		}
		return "", fmt.Errorf("reading dashboard password: %w", err)
	}
	if hash == nil {
		return "", nil
	}
	return *hash, nil
}

// BootstrapPassword stores the first hash only and reports whether it won the race.
func (r *AuthRepository) BootstrapPassword(ctx context.Context, hash string) (bool, error) {
	var id int16
	err := r.pool.QueryRow(ctx, `
		UPDATE panel_auth
		SET password_hash = $1, updated_at = now()
		WHERE id = 1 AND password_hash IS NULL
		RETURNING id`, hash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("bootstrapping dashboard password: %w", err)
	}
	return id == 1, nil
}

// ChangePassword updates the hash only if it still equals expectedHash.
func (r *AuthRepository) ChangePassword(ctx context.Context, expectedHash, newHash string) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE panel_auth
		SET password_hash = $1, updated_at = now()
		WHERE id = 1 AND password_hash = $2`, newHash, expectedHash)
	if err != nil {
		return fmt.Errorf("changing dashboard password: %w", err)
	}
	if result.RowsAffected() != 1 {
		return domain.ErrPasswordChanged
	}
	return nil
}
