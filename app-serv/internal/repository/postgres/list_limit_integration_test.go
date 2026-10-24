//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/list_limit_integration_test.go
// @for       The safety cap on the three hand-maintained list reads.
// @uses      context, testing.
// @reason    AGENTS.md §1.7 refuses a fetch-all read, and a cap that no test crosses is a comment rather than a guarantee; each table gets one row past its bound so the truncation is observed from the server, not inferred from the query text.
//
//	Run with:
//	  PANNELAI_TEST_POSTGRES_DSN='postgres://...' \
//	    go test -race -tags=integration ./internal/repository/postgres/
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-24
package postgres

import (
	"context"
	"testing"
)

func TestProxyRepository_ListStopsAtTheCap(t *testing.T) {
	repo := newTestProxyRepo(t)
	ctx := context.Background()
	const insert = `
INSERT INTO proxies (id, label, protocol, host, port, username, password_encrypted, enabled, status, created_at, updated_at)
SELECT 'prx_' || g, 'pool ' || g, 'http', 'pool.example.com', 8080, '', '', true, '{}'::jsonb, now(), now()
  FROM generate_series(1, $1) g`
	if _, err := repo.pool.Exec(ctx, insert, proxyListLimit+1); err != nil {
		t.Fatalf("seeding past the cap: %v", err)
	}

	listed, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != proxyListLimit {
		t.Fatalf("List() returned %d rows, want the cap of %d", len(listed), proxyListLimit)
	}
}

func TestNodeRepository_ListStopsAtTheCap(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `TRUNCATE provider_nodes`); err != nil {
		t.Fatalf("truncating: %v", err)
	}
	const insert = `
INSERT INTO provider_nodes (id, type, name, prefix, api_type, base_url, created_at, updated_at)
SELECT 'node_' || g, 'openai-compatible', 'node ' || g, 'prefix' || g, 'openai', 'https://node.example.com', now(), now()
  FROM generate_series(1, $1) g`
	if _, err := pool.Exec(ctx, insert, nodeListLimit+1); err != nil {
		t.Fatalf("seeding past the cap: %v", err)
	}

	listed, err := NewNodeRepository(pool).List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != nodeListLimit {
		t.Fatalf("List() returned %d rows, want the cap of %d", len(listed), nodeListLimit)
	}
}

func TestComboRepository_NamesStopsAtTheCap(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `TRUNCATE combos`); err != nil {
		t.Fatalf("truncating: %v", err)
	}
	const insert = `
INSERT INTO combos (id, name, strategy, sticky_limit, models, created_at, updated_at)
SELECT 'cmb_' || g, 'combo-' || g, 'fallback', 1, '[]'::jsonb, now(), now()
  FROM generate_series(1, $1) g`
	if _, err := pool.Exec(ctx, insert, comboNamesLimit+1); err != nil {
		t.Fatalf("seeding past the cap: %v", err)
	}

	names, err := NewComboRepository(pool).Names(ctx)
	if err != nil {
		t.Fatalf("Names() error = %v", err)
	}
	if len(names) != comboNamesLimit {
		t.Fatalf("Names() returned %d names, want the cap of %d", len(names), comboNamesLimit)
	}
}
