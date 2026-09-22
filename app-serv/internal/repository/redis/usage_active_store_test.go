//go:build integration

// Package redisrepo implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/usage_active_store_test.go
// @for       Integration tests for the in-flight marker store against a real
//
//	Redis: the round trip, the staleness prune, and the bound.
//
// @uses      github.com/redis/go-redis/v9, internal/domain, context, os,
//
//	testing, time.
//
// @reason    The store's whole contract is Redis behaviour: a sorted set scored
//
//	by the start instant, a range read bounded by a cutoff, and a prune
//	that runs with the read. An in-memory double would prove the
//	arithmetic but not that ZRANGEBYSCORE applies the score bound or that
//	ZREMRANGEBYSCORE removes what it matched, and those are the two
//	operations whose failure is silent: the set simply grows and the
//	drawing lights stale nodes.
//
//	The file carries an `integration` build tag, so the default
//	`go test ./...` stays hermetic on a machine with no Redis (AGENTS.md
//	§2.1 forbids t.Skip, and a tagged file is not compiled rather than
//	skipped at runtime). With the tag active the address is required.
//
//	  PANNELAI_TEST_REDIS_ADDR='[user:password@]host:port' \
//	    go test -race -tags=integration ./internal/repository/redis/
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-22
package redisrepo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// activeStoreFixture returns a store over a flushed Redis, so each case reads a
// keyspace that belongs to it alone.
func activeStoreFixture(t *testing.T) (*ActiveRequestStore, context.Context) {
	t.Helper()
	raw := os.Getenv(testRedisEnv)
	if raw == "" {
		t.Fatalf("%s must be set to run the active request store tests", testRedisEnv)
	}
	client := redis.NewClient(redisOptions(t, raw))
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("closing redis client: %v", err)
		}
	})
	ctx := context.Background()
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flushing Redis: %v", err)
	}
	return NewActiveRequestStore(client), ctx
}

// TestActiveRequestStore_RoundTrip covers the write, read, and remove lifecycle
// through one table, including the boundary cases TDD.md §2.5 requires: a marker
// at the cutoff instant is stale, one just inside it is live, and removing a
// marker that is already gone is not a failure.
func TestActiveRequestStore_RoundTrip(t *testing.T) {
	store, ctx := activeStoreFixture(t)
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)

	cases := []struct {
		name      string
		requestID string
		provider  string
		startedAt time.Time
		wantLive  bool
	}{
		{name: "a call that just started is live", requestID: "req_now", provider: "openai", startedAt: now, wantLive: true},
		{
			name: "a call one second short of the window is live", requestID: "req_edge", provider: "anthropic",
			startedAt: now.Add(-domain.ActiveRequestStaleAfter + time.Second), wantLive: true,
		},
		{
			name: "a call exactly at the window is stale", requestID: "req_stale", provider: "gemini",
			startedAt: now.Add(-domain.ActiveRequestStaleAfter), wantLive: false,
		},
		{
			name: "a call well past the window is stale", requestID: "req_ancient", provider: "mistral",
			startedAt: now.Add(-24 * time.Hour), wantLive: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			marker, err := domain.NewActiveRequest(tc.requestID, tc.provider, "ep_active", "gpt-4o", tc.startedAt)
			if err != nil {
				t.Fatalf("building the marker: %v", err)
			}
			if err := store.Start(ctx, marker); err != nil {
				t.Fatalf("Start() = %v, want nil", err)
			}

			active, err := store.Active(ctx, now, 20)
			if err != nil {
				t.Fatalf("Active() = %v, want nil", err)
			}
			found := false
			for _, got := range active {
				if got.MarkerID == marker.MarkerID {
					found = true
					if got.ProviderID != tc.provider || got.RequestID != tc.requestID {
						t.Fatalf("marker read back = %+v, want provider %q and request %q", got, tc.provider, tc.requestID)
					}
				}
			}
			if found != tc.wantLive {
				t.Fatalf("marker present = %v, want %v (active = %+v)", found, tc.wantLive, active)
			}

			if err := store.Finish(ctx, marker); err != nil {
				t.Fatalf("Finish() = %v, want nil", err)
			}
			// Removing it twice is the documented no-op: the staleness window or
			// a restart may already have removed it.
			if err := store.Finish(ctx, marker); err != nil {
				t.Fatalf("Finish() on a missing marker = %v, want nil", err)
			}
			after, err := store.Active(ctx, now, 20)
			if err != nil {
				t.Fatalf("Active() = %v, want nil", err)
			}
			for _, got := range after {
				if got.MarkerID == marker.MarkerID {
					t.Fatalf("a finished marker is still active: %+v", got)
				}
			}
		})
	}
}

// TestActiveRequestStore_PrunesStaleMarkersOnRead pins the second half of the
// bound: the read removes what it found stale, so a marker left behind by a
// process that died cannot accumulate forever.
func TestActiveRequestStore_PrunesStaleMarkersOnRead(t *testing.T) {
	store, ctx := activeStoreFixture(t)
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)

	stale, err := domain.NewActiveRequest("req_stale", "openai", "ep_1", "gpt-4o", now.Add(-2*time.Hour))
	if err != nil {
		t.Fatalf("building the stale marker: %v", err)
	}
	fresh, err := domain.NewActiveRequest("req_fresh", "openai", "ep_1", "gpt-4o", now)
	if err != nil {
		t.Fatalf("building the fresh marker: %v", err)
	}
	for _, marker := range []domain.ActiveRequest{stale, fresh} {
		if err := store.Start(ctx, marker); err != nil {
			t.Fatalf("Start() = %v, want nil", err)
		}
	}

	active, err := store.Active(ctx, now, 20)
	if err != nil {
		t.Fatalf("Active() = %v, want nil", err)
	}
	if len(active) != 1 || active[0].MarkerID != fresh.MarkerID {
		t.Fatalf("Active() = %+v, want only the fresh marker", active)
	}

	// The prune has to be durable, not just a filter on this one answer: a
	// second read at the same instant must find the same single marker.
	again, err := store.Active(ctx, now, 20)
	if err != nil {
		t.Fatalf("Active() = %v, want nil", err)
	}
	if len(again) != 1 {
		t.Fatalf("the second read = %+v, want one marker", again)
	}
	// And a read far in the future sees the fresh marker stale, so the window is
	// applied to whatever the set holds rather than to what was written.
	later, err := store.Active(ctx, now.Add(time.Hour), 20)
	if err != nil {
		t.Fatalf("Active() = %v, want nil", err)
	}
	if len(later) != 0 {
		t.Fatalf("a read an hour later = %+v, want none", later)
	}
}
