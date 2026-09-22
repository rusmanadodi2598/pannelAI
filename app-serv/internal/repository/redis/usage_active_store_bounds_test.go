//go:build integration

// Package redisrepo implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/usage_active_store_bounds_test.go
// @for       Integration tests for the marker store's bound, its ordering, and
//
//	its no-op and refusal paths.
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
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestActiveRequestStore_BoundsAndOrdersTheRead pins the bound and the order:
// a caller asking for two of five gets the two oldest, so the frame's list is
// the part of the set that has been running longest.
func TestActiveRequestStore_BoundsAndOrdersTheRead(t *testing.T) {
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)

	cases := []struct {
		name      string
		written   int
		limit     int
		wantCount int
	}{
		{name: "no marker is written", written: 0, limit: 10, wantCount: 0},
		{name: "one marker", written: 1, limit: 10, wantCount: 1},
		{name: "the limit truncates the set", written: 5, limit: 2, wantCount: 2},
		{name: "a limit above the set size returns the set", written: 3, limit: 20, wantCount: 3},
		{name: "a zero limit returns nothing", written: 3, limit: 0, wantCount: 0},
		{name: "a negative limit returns nothing", written: 3, limit: -1, wantCount: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// A fresh store per case, so the bound is measured against a set this
			// case wrote rather than against whatever the previous one left.
			store, ctx := activeStoreFixture(t)
			var first domain.ActiveRequest
			for i := 0; i < tc.written; i++ {
				marker, err := domain.NewActiveRequest(
					"req_"+string(rune('a'+i)), "openai", "ep_1", "gpt-4o",
					now.Add(-time.Duration(tc.written-i)*time.Second),
				)
				if err != nil {
					t.Fatalf("building marker %d: %v", i, err)
				}
				if i == 0 {
					first = marker
				}
				if err := store.Start(ctx, marker); err != nil {
					t.Fatalf("Start() = %v, want nil", err)
				}
			}
			active, err := store.Active(ctx, now, tc.limit)
			if err != nil {
				t.Fatalf("Active() = %v, want nil", err)
			}
			if len(active) != tc.wantCount {
				t.Fatalf("Active() returned %d markers, want %d", len(active), tc.wantCount)
			}
			if tc.wantCount > 1 {
				if !active[0].StartedAt.Before(active[len(active)-1].StartedAt) {
					t.Fatalf("markers are not oldest first: %+v", active)
				}
				if active[0].MarkerID != first.MarkerID {
					t.Fatalf("the first marker read back is %s, want the oldest %s", active[0].MarkerID, first.MarkerID)
				}
			}
		})
	}
}

// TestActiveRequestStore_NilStoreIsANoOp pins the documented behaviour for a
// deployment that wired no Redis: the data plane keeps serving and the drawing
// shows no active provider, rather than the process refusing to boot.
func TestActiveRequestStore_NilStoreIsANoOp(t *testing.T) {
	ctx := context.Background()
	var store *ActiveRequestStore
	marker, err := domain.NewActiveRequest("req_1", "openai", "ep_1", "gpt-4o", time.Now())
	if err != nil {
		t.Fatalf("building the marker: %v", err)
	}
	if err := store.Start(ctx, marker); err != nil {
		t.Fatalf("a nil store's Start() = %v, want nil", err)
	}
	if err := store.Finish(ctx, marker); err != nil {
		t.Fatalf("a nil store's Finish() = %v, want nil", err)
	}
	active, err := store.Active(ctx, time.Now(), 10)
	if err != nil {
		t.Fatalf("a nil store's Active() = %v, want nil", err)
	}
	if len(active) != 0 {
		t.Fatalf("a nil store's Active() = %+v, want none", active)
	}
}

// TestActiveRequestStore_RefusesAnInvalidMarker pins that the store applies the
// aggregate's own rule on the way in, so a caller cannot write a marker that no
// reader would draw.
func TestActiveRequestStore_RefusesAnInvalidMarker(t *testing.T) {
	store, ctx := activeStoreFixture(t)
	cases := []struct {
		name    string
		marker  domain.ActiveRequest
		wantErr bool
	}{
		{name: "a valid marker is accepted", marker: domain.ActiveRequest{
			MarkerID: "m1", RequestID: "r1", ProviderID: "openai", StartedAt: time.Now(),
		}},
		{name: "a marker with no provider is refused", marker: domain.ActiveRequest{
			MarkerID: "m2", RequestID: "r2", StartedAt: time.Now(),
		}, wantErr: true},
		{name: "a zero marker is refused", marker: domain.ActiveRequest{}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := store.Start(ctx, tc.marker)
			if tc.wantErr && err == nil {
				t.Fatalf("Start(%+v) = nil, want a refusal", tc.marker)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Start(%+v) = %v, want nil", tc.marker, err)
			}
		})
	}
}
