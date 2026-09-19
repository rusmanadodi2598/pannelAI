//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/gateway_key_crud_test.go
// @for       Integration coverage for the gateway key CRUD, pagination round
//
//	trip, and use counter.
//
// @uses      internal/domain, internal/repository, context, errors, testing, time.
// @reason    AGENTS.md §2.1 requires repository logic be tested, and the pagination
//
//	contract (page and total describing one snapshot) is enforced by
//	the SQL, not by Go. Run with -tags=integration plus a DSN; a missing
//	DSN fails rather than skipping (AGENTS.md §2.1).
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
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// TestIntegration_RoundTrip covers create, read, update, and revoke against a
// real database, including the fields the wire shape depends on.
func TestIntegration_RoundTrip(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	created := seed(t, repo, "round-trip")

	got, err := repo.GetByID(ctx, created.ID())
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name() != "round-trip" || got.Status() != domain.GatewayKeyActive {
		t.Fatalf("round trip = %q/%q, want round-trip/active", got.Name(), got.Status())
	}
	if got.ValueHash() != created.ValueHash() {
		t.Fatal("stored digest differs from the issued one")
	}
	if got.CreatedAt().UTC().Truncate(time.Second) != created.CreatedAt().UTC().Truncate(time.Second) {
		t.Fatalf("created_at = %v, want %v", got.CreatedAt(), created.CreatedAt())
	}

	if err := got.Rename("renamed"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	after, err := repo.GetByID(ctx, created.ID())
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if after.Name() != "renamed" {
		t.Fatalf("name = %q, want renamed", after.Name())
	}

	revokedAt := time.Now().UTC().Truncate(time.Millisecond)
	if err := repo.Revoke(ctx, created.ID(), revokedAt); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	revoked, err := repo.GetByID(ctx, created.ID())
	if err != nil {
		t.Fatalf("GetByID after revoke: %v", err)
	}
	if revoked.Status() != domain.GatewayKeyRevoked {
		t.Fatalf("status = %q, want revoked", revoked.Status())
	}
	if revoked.RevokedAt() == nil {
		t.Fatal("revoked_at must be persisted")
	}
}

// TestIntegration_GetByID_NotFound maps a missing row to the sentinel.
func TestIntegration_GetByID_NotFound(t *testing.T) {
	repo := newTestRepo(t)

	_, err := repo.GetByID(context.Background(), "gky_does_not_exist")
	if !errors.Is(err, domain.ErrGatewayKeyNotFound) {
		t.Fatalf("GetByID(missing) = %v, want %v", err, domain.ErrGatewayKeyNotFound)
	}
}

// TestIntegration_List_PaginationAndTotal pins the contract the window-function
// query exists to satisfy: the page and the total come from one snapshot, and
// offsets walk the set without repeating or skipping a row.
func TestIntegration_List_PaginationAndTotal(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	const seeded = 7
	wantIDs := make(map[string]struct{}, seeded)
	for i := 0; i < seeded; i++ {
		wantIDs[seed(t, repo, "key-"+string(rune('a'+i))).ID()] = struct{}{}
	}

	cases := []struct {
		name      string
		page      int
		perPage   int
		wantCount int
	}{
		{"first page", 1, 3, 3},
		{"middle page", 2, 3, 3},
		{"last partial page", 3, 3, 1},
		{"page past the end still reports the total", 9, 3, 0},
		{"single row pages", 4, 2, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			keys, total, err := repo.List(ctx, repository.PageQuery{Page: tc.page, PerPage: tc.perPage})
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if len(keys) != tc.wantCount {
				t.Fatalf("len(keys) = %d, want %d", len(keys), tc.wantCount)
			}
			if total != seeded {
				t.Fatalf("total = %d, want %d", total, seeded)
			}
		})
	}

	// Walking every page must visit each seeded key exactly once.
	seen := map[string]int{}
	for page := 1; page <= 3; page++ {
		keys, _, err := repo.List(ctx, repository.PageQuery{Page: page, PerPage: 3})
		if err != nil {
			t.Fatalf("List page %d: %v", page, err)
		}
		for _, k := range keys {
			seen[k.ID()]++
		}
	}
	if len(seen) != len(wantIDs) {
		t.Fatalf("pagination visited %d distinct keys, want %d", len(seen), len(wantIDs))
	}
	for id, count := range seen {
		if _, expected := wantIDs[id]; !expected {
			t.Fatalf("pagination returned an unseeded key %s", id)
		}
		if count != 1 {
			t.Fatalf("key %s appeared %d times across pages, want 1", id, count)
		}
	}
	for id := range wantIDs {
		if _, visited := seen[id]; !visited {
			t.Fatalf("pagination never returned seeded key %s", id)
		}
	}
}

// TestIntegration_RecordUse pins the two columns one authenticated call
// advances: the counter climbs by one per call and last_used_at is stamped. The
// arithmetic is the database's own, which is what keeps two concurrent calls on
// one key from losing a count.
func TestIntegration_RecordUse(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	created := seed(t, repo, "record-use")

	if created.RequestCount() != 0 || created.LastUsedAt() != nil {
		t.Fatalf("fresh key = %d/%v, want 0/nil", created.RequestCount(), created.LastUsedAt())
	}

	usedAt := time.Now().UTC().Truncate(time.Millisecond)
	const calls = 3
	for i := 0; i < calls; i++ {
		if err := repo.RecordUse(ctx, created.ID(), usedAt); err != nil {
			t.Fatalf("RecordUse(%d): %v", i, err)
		}
	}
	got, err := repo.GetByID(ctx, created.ID())
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.RequestCount() != calls {
		t.Fatalf("request_count = %d, want %d", got.RequestCount(), calls)
	}
	if got.LastUsedAt() == nil {
		t.Fatal("last_used_at must be stamped")
	}

	if err := repo.RecordUse(ctx, "gky_does_not_exist", usedAt); !errors.Is(err, domain.ErrGatewayKeyNotFound) {
		t.Fatalf("RecordUse(missing) = %v, want %v", err, domain.ErrGatewayKeyNotFound)
	}
}
