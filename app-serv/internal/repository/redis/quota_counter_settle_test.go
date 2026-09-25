//go:build integration

// Package redisrepo implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/quota_counter_settle_test.go
// @for       The settle half of the quota counter: which windows a flush
//
//	rewrites, which it marks mirrored, and which it retires.
//
// @uses      github.com/redis/go-redis/v9, internal/domain, context, os,
//
//	testing, time.
//
// @reason    Settle is the operation whose wrong shape lost window totals
//
//	live (the 2026-09-23 defect): a flush must mark what it mirrored
//	without dropping a total still in use, and retire only what nothing
//	would read again. Split from quota_counter_test.go to hold the
//	AGENTS.md §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-23
package redisrepo

import (
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestQuotaCounterStore_SettleMirrorsOpenWindows pins the write-amplification
// rule: a total the flush has already mirrored is not written again, and a total
// that changed since the flush still is.
func TestQuotaCounterStore_SettleMirrorsOpenWindows(t *testing.T) {
	store, ctx := quotaCounterFixture(t)
	now := time.Now()
	open := now.Add(time.Hour)

	if err := store.Add(ctx, "ep_a", domain.QuotaWindowDaily, 65, open); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	window, _ := pendingFor(t, store, ctx, "ep_a", domain.QuotaWindowDaily)
	if err := store.Settle(ctx, []domain.QuotaWindow{window}); err != nil {
		t.Fatalf("Settle() error = %v", err)
	}

	// An unchanged, still-open window has nothing new to write.
	if _, ok := pendingFor(t, store, ctx, "ep_a", domain.QuotaWindowDaily); ok {
		t.Fatalf("a mirrored open window is pending, want nothing to write")
	}

	// The total is still there: the next request continues from it.
	if err := store.Add(ctx, "ep_a", domain.QuotaWindowDaily, 10, open); err != nil {
		t.Fatalf("Add(10) error = %v", err)
	}
	changed, ok := pendingFor(t, store, ctx, "ep_a", domain.QuotaWindowDaily)
	if !ok {
		t.Fatalf("a window changed after the flush is not pending, want a rewrite")
	}
	if got := changed.Used(); got != 75 {
		t.Fatalf("Used() = %d, want 75: the settled total must survive", got)
	}
}

// TestQuotaCounterStore_RetiresSettledClosedWindows pins that a durable write
// retires a closed window's hot copy, and that a window holding units the flush
// has not mirrored is kept whichever side of the reset it is on.
func TestQuotaCounterStore_RetiresSettledClosedWindows(t *testing.T) {
	store, ctx := quotaCounterFixture(t)
	now := time.Now()

	// A closed window with nothing new: settled, then retired. The proof of
	// retirement is that the next request restarts from its own units rather
	// than continuing a total that was still stored.
	if err := store.Add(ctx, "ep_closed", domain.QuotaWindowWeekly, 65, now.Add(-time.Minute)); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	closed, _ := pendingFor(t, store, ctx, "ep_closed", domain.QuotaWindowWeekly)
	if err := store.Settle(ctx, []domain.QuotaWindow{closed}); err != nil {
		t.Fatalf("Settle() error = %v", err)
	}
	if err := store.Add(ctx, "ep_closed", domain.QuotaWindowWeekly, 1, now.Add(time.Hour)); err != nil {
		t.Fatalf("Add(1) error = %v", err)
	}
	restarted, ok := pendingFor(t, store, ctx, "ep_closed", domain.QuotaWindowWeekly)
	if !ok {
		t.Fatalf("the restarted window is not pending")
	}
	if got := restarted.Used(); got != 1 {
		t.Fatalf("Used() = %d, want 1: a settled closed window must be retired, not continued", got)
	}

	// A window that changed after the flush, then closed before the settle:
	// those units are not durable, so it is kept and rewritten.
	const gap = 200 * time.Millisecond
	if err := store.Add(ctx, "ep_raced", domain.QuotaWindowWeekly, 65, now.Add(gap)); err != nil {
		t.Fatalf("Add(65) error = %v", err)
	}
	if err := store.Add(ctx, "ep_raced", domain.QuotaWindowWeekly, 10, now.Add(gap)); err != nil {
		t.Fatalf("Add(10) error = %v", err)
	}
	time.Sleep(gap + 50*time.Millisecond)

	stale, _ := domain.NewQuotaWindow("ep_raced", "", domain.QuotaWindowWeekly, nil, nil, now)
	stale.Add(65, now) // the value the flush read, before the second request
	if err := store.Settle(ctx, []domain.QuotaWindow{stale}); err != nil {
		t.Fatalf("Settle(stale) error = %v", err)
	}
	kept, ok := pendingFor(t, store, ctx, "ep_raced", domain.QuotaWindowWeekly)
	if !ok {
		t.Fatalf("a window changed since the flush is not pending, want a rewrite")
	}
	if got := kept.Used(); got != 75 {
		t.Fatalf("Used() = %d, want 75", got)
	}
}

// TestQuotaCounterStore_KeepsEndpointsAndWindowsApart pins that the running
// total is per endpoint and per window kind: one endpoint's traffic must not
// advance another's, and a five-hour total must not move the daily one.
func TestQuotaCounterStore_KeepsEndpointsAndWindowsApart(t *testing.T) {
	store, ctx := quotaCounterFixture(t)
	now := time.Now()

	if err := store.Add(ctx, "ep_a", domain.QuotaWindowFiveHour, 10, now.Add(time.Hour)); err != nil {
		t.Fatalf("Add(ep_a/5h) error = %v", err)
	}
	if err := store.Add(ctx, "ep_b", domain.QuotaWindowDaily, 20, now.Add(time.Hour)); err != nil {
		t.Fatalf("Add(ep_b/daily) error = %v", err)
	}

	if window, ok := pendingFor(t, store, ctx, "ep_a", domain.QuotaWindowFiveHour); !ok || window.Used() != 10 {
		t.Fatalf("ep_a/5h = %+v/%v, want 10", window, ok)
	}
	if window, ok := pendingFor(t, store, ctx, "ep_b", domain.QuotaWindowDaily); !ok || window.Used() != 20 {
		t.Fatalf("ep_b/daily = %+v/%v, want 20", window, ok)
	}
	if _, ok := pendingFor(t, store, ctx, "ep_a", domain.QuotaWindowDaily); ok {
		t.Fatalf("ep_a/daily is pending, want no counter for a window never added")
	}
}
