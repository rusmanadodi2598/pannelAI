//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_oauth_batch_integration_test.go
// @for       Integration proof that the OAuth import batch is all-or-nothing.
// @uses      github.com/jackc/pgx/v5/pgxpool, internal/domain, internal/migrations,
//
//	context, errors, os, testing, time.
//
// @reason    SPEC-API-001 §8.1 promises that a refused row leaves no account
//
//	behind. Only a real server can prove it: the guarantee is a property
//	of the transaction, so an in-memory double that shares the same
//	loop proves the loop, not the rollback. A row that collides on the
//	primary key is the failure this file stages, and the assertion is
//	that the row before it is absent afterwards.
//
//	  PANNELAI_TEST_POSTGRES_DSN='postgres://...' \
//	    go test -race -tags=integration ./internal/repository/postgres/
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-18
package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/migrations"
)

// newEndpointRepo connects to the configured database, applies migrations, and
// returns a repository with an empty endpoint table.
func newEndpointRepo(t *testing.T) *EndpointRepository {
	t.Helper()

	dsn := requireTestDSN(t)
	ctx := context.Background()
	if err := migrations.Apply(ctx, dsn); err != nil {
		t.Fatalf("applying migrations: %v", err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(pool.Close)

	// CASCADE reaches upstream_keys, so a leftover key cannot outlive its
	// endpoint and fail the next run on a foreign key.
	if _, err := pool.Exec(ctx, `TRUNCATE upstream_endpoints CASCADE`); err != nil {
		t.Fatalf("truncating upstream_endpoints: %v", err)
	}
	return NewEndpointRepository(pool)
}

// oauthEndpoint builds a stored-shape OAuth endpoint for the batch under test.
func oauthEndpoint(t *testing.T, id, label, email string, now time.Time) domain.UpstreamEndpoint {
	t.Helper()
	endpoint, err := domain.NewUpstreamEndpoint(id, "openai", label, domain.UpstreamAuthOAuth, 1, now)
	if err != nil {
		t.Fatalf("building endpoint %s: %v", id, err)
	}
	endpoint.SetOAuth(&domain.OAuthCredential{
		AccessTokenEncrypted: "sealed-access-token",
		AccountEmail:         email,
	}, now)
	endpoint.SetAccount(domain.EndpointAccount{Email: email}, now)
	return endpoint
}

// TestIntegration_ImportOAuthBatch_IsAllOrNothing stages a batch whose second
// row collides on the primary key: the first row must not survive the rollback,
// and the error must name the second row.
func TestIntegration_ImportOAuthBatch_IsAllOrNothing(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	// The same id twice, so the second INSERT violates the primary key. The
	// first row is otherwise perfectly valid, which is what makes the
	// assertion meaningful: it is written and then rolled back.
	first := oauthEndpoint(t, "ep_batch_first", "first@example.com", "first@example.com", now)
	second := oauthEndpoint(t, "ep_batch_first", "second@example.com", "second@example.com", now)

	err := repo.ImportOAuthBatch(ctx, []domain.UpstreamEndpoint{first, second}, []bool{false, false})
	if err == nil {
		t.Fatal("ImportOAuthBatch() = nil, want the colliding row to be refused")
	}
	var rowErr *BulkRowError
	if !errors.As(err, &rowErr) {
		t.Fatalf("error = %v (%T), want a *BulkRowError", err, err)
	}
	if rowErr.Index != 1 {
		t.Fatalf("refused row index = %d, want 1", rowErr.Index)
	}

	if _, err := repo.GetByID(ctx, "ep_batch_first"); !errors.Is(err, domain.ErrEndpointNotFound) {
		t.Fatalf("GetByID after the refused batch = %v, want ErrEndpointNotFound: the first row must have been rolled back", err)
	}
}

// TestIntegration_ImportOAuthBatch_MixesCreateAndUpdate covers the re-import
// case §8.1 makes an update: one row replaces the credential set of an account
// already stored, the other adds a new account, and both land.
func TestIntegration_ImportOAuthBatch_MixesCreateAndUpdate(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	seeded := oauthEndpoint(t, "ep_existing", "before@example.com", "before@example.com", now)
	if err := repo.ImportOAuthBatch(ctx, []domain.UpstreamEndpoint{seeded}, []bool{false}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	updated := oauthEndpoint(t, "ep_existing", "after@example.com", "after@example.com", now.Add(time.Minute))
	added := oauthEndpoint(t, "ep_added", "added@example.com", "added@example.com", now)

	if err := repo.ImportOAuthBatch(ctx,
		[]domain.UpstreamEndpoint{updated, added}, []bool{true, false}); err != nil {
		t.Fatalf("ImportOAuthBatch() error = %v", err)
	}

	stored, err := repo.GetByID(ctx, "ep_existing")
	if err != nil {
		t.Fatalf("GetByID(updated): %v", err)
	}
	if stored.Label() != "after@example.com" {
		t.Fatalf("label = %q, want the updated one", stored.Label())
	}
	if got := stored.Account().Email; got != "after@example.com" {
		t.Fatalf("account email = %q, want the updated identity", got)
	}
	if _, err := repo.GetByID(ctx, "ep_added"); err != nil {
		t.Fatalf("GetByID(added): %v, want the new account to be stored", err)
	}
}

// TestIntegration_ImportOAuthBatch_AttributesARefusedUpdate covers the other
// failure arm: an update whose target no longer exists is reported against its
// own row rather than as a batch-wide error.
func TestIntegration_ImportOAuthBatch_AttributesARefusedUpdate(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	missing := oauthEndpoint(t, "ep_gone", "gone@example.com", "gone@example.com", now)
	err := repo.ImportOAuthBatch(ctx, []domain.UpstreamEndpoint{missing}, []bool{true})
	if !errors.Is(err, domain.ErrEndpointNotFound) {
		t.Fatalf("error = %v, want ErrEndpointNotFound", err)
	}
	var rowErr *BulkRowError
	if !errors.As(err, &rowErr) || rowErr.Index != 0 {
		t.Fatalf("error = %v, want a *BulkRowError naming row 0", err)
	}
}
