//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/gateway_key_constraint_test.go
// @for       Integration proof that the schema constraints the contract promises exist.
// @uses      github.com/jackc/pgx/v5/pgxpool, internal/domain, internal/repository,
//
//	internal/migrations, context, errors, testing.
//
// @reason    A stub cannot prove a UNIQUE index exists: the earlier P0 schema carried a
//
//	plain index on name, so PostgreSQL accepted duplicates while the
//	in-memory test double rejected them. Only a real server closes that
//	gap, and CREATE and UPDATE are different statements, so both are
//	covered. Run with -tags=integration plus a DSN; a missing DSN fails
//	rather than skipping (AGENTS.md §2.1).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-16
package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestIntegration_Create_RejectsDuplicateName is the regression for the schema
// defect: the API promises CONFLICT, and only a UNIQUE index makes it true.
func TestIntegration_Create_RejectsDuplicateName(t *testing.T) {
	repo := newTestRepo(t)
	seed(t, repo, "ci-runner")

	duplicate := domain.NewGatewayKey("ci-runner", "sk-other-secret-value", "sk-…alue", time.Now().UTC())
	err := repo.Create(context.Background(), duplicate)
	if !errors.Is(err, domain.ErrGatewayKeyExists) {
		t.Fatalf("Create(duplicate) = %v, want %v", err, domain.ErrGatewayKeyExists)
	}

	public := domain.AsAppError(err)
	if public.Code != "CONFLICT" || public.HTTPStatus() != 409 {
		t.Fatalf("duplicate maps to %s/%d, want CONFLICT/409", public.Code, public.HTTPStatus())
	}
}

// TestIntegration_Update_RejectsRenameOntoExistingName proves the UNIQUE index
// guards UPDATE too. A rename is a different statement from INSERT, so the
// create-time check says nothing about it; without this the repository would
// quietly allow two keys to share a name.
func TestIntegration_Update_RejectsRenameOntoExistingName(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	first := seed(t, repo, "owner")
	second := seed(t, repo, "borrower")

	// Rename the second key onto the first key's name.
	collide, err := repo.GetByID(ctx, second.ID())
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if err := collide.Rename(first.Name()); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	err = repo.Update(ctx, collide)
	if !errors.Is(err, domain.ErrGatewayKeyExists) {
		t.Fatalf("Update(colliding rename) = %v, want %v", err, domain.ErrGatewayKeyExists)
	}

	// The failed rename must leave the stored row untouched.
	unchanged, err := repo.GetByID(ctx, second.ID())
	if err != nil {
		t.Fatalf("GetByID after failed rename: %v", err)
	}
	if unchanged.Name() != "borrower" {
		t.Fatalf("name = %q, want the original borrower", unchanged.Name())
	}

	// Renaming a key to its own name is not a collision.
	same, err := repo.GetByID(ctx, first.ID())
	if err != nil {
		t.Fatalf("GetByID first: %v", err)
	}
	if err := repo.Update(ctx, same); err != nil {
		t.Fatalf("Update(rename to own name) = %v, want nil", err)
	}
}
