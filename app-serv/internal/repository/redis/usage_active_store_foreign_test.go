//go:build integration

// Package redisrepo implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/usage_active_store_foreign_test.go
// @for       Integration test for the one read rule that cannot be pinned in memory: what a real sorted set does with a member this build cannot parse.
// @uses      internal/domain, github.com/redis/go-redis/v9, testing, time.
// @reason    The claim is about Redis behaviour and about a value another process wrote, so an in-memory double would prove the arithmetic of the filter but not that the member survives the read. Split from `usage_active_store_test.go` because that file crossed the AGENTS.md §1.1 budget with this row in it, and the seam is the subject: one file reads and writes what this build wrote, this one reads what it did not.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-03
package redisrepo

import (
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestActiveRequestStore_SkipsAMemberItCannotRead pins the second half of the
// read's rule about an unreadable member: it is skipped, and it is left alone.
//
// The member below is what a build carrying a field this one has never seen
// writes, the shape a rolling deploy actually produces. Deleting it would let a
// reader with no idea what it erased suppress another process's live request, so
// the store drops it from the answer and keeps the value; the score prune the
// same read runs collects it once its own window closes.
func TestActiveRequestStore_SkipsAMemberItCannotRead(t *testing.T) {
	store, client, ctx := activeStoreFixture(t)
	now := time.Date(2026, 10, 3, 10, 0, 0, 0, time.UTC)

	foreign := `{"marker_id":"mark_foreign","request_id":"req_foreign","provider_id":"openai",` +
		`"model":"gpt-4o","gateway_key_id":"gky_1","started_at":"2026-10-03T10:00:00Z"}`
	if err := client.ZAdd(ctx, activeRequestKey, redis.Z{
		Score:  float64(now.UnixMilli()),
		Member: foreign,
	}).Err(); err != nil {
		t.Fatalf("planting the foreign member: %v", err)
	}

	live, err := domain.NewActiveRequest(domain.ActiveRequestInput{
		RequestID: "req_live", ProviderID: "anthropic", EndpointID: "ep_1", Model: "claude-3-5-sonnet",
	}, now)
	if err != nil {
		t.Fatalf("building the marker: %v", err)
	}
	if err := store.Start(ctx, live); err != nil {
		t.Fatalf("Start() = %v, want nil", err)
	}

	active, err := store.Active(ctx, now, 20)
	if err != nil {
		t.Fatalf("Active() = %v, want nil", err)
	}
	if len(active) != 1 || active[0].MarkerID != live.MarkerID {
		t.Fatalf("Active() = %+v, want only the marker this build wrote", active)
	}

	remaining, err := client.ZCard(ctx, activeRequestKey).Result()
	if err != nil {
		t.Fatalf("counting the set: %v", err)
	}
	if remaining != 2 {
		t.Fatalf("the set holds %d members, want the foreign one left where it stands alongside the live marker", remaining)
	}
}
