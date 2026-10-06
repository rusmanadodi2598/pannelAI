//go:build integration

// Package redisrepo implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/quota_counter_pending_test.go
// @for       The flush batch's fairness: windows already mirrored do not crowd a dirty endpoint out of Pending's limit.
// @uses      fmt, github.com/redis/go-redis/v9, internal/domain, context, os, testing, time.
// @reason    Pending used to stop at `limit` keys and filter afterwards, so a keyspace whose first SCAN keys were all settled returned an empty batch forever and a busy endpoint's counted usage never reached PostgreSQL (draft 042 R18). Only a live server can prove the scan behaviour, per this package's tagging rule.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-03
package redisrepo

import (
	"fmt"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestQuotaCounterStore_PendingSkipsSettledWindows pins that a batch of limit
// one still reports the endpoint whose counters changed, however many settled
// windows precede it in scan order.
func TestQuotaCounterStore_PendingSkipsSettledWindows(t *testing.T) {
	store, ctx := quotaCounterFixture(t)
	resets := time.Now().Add(time.Hour)

	// One hundred and twenty-eight endpoints record usage, and their windows are
	// settled so every one of them reads as already mirrored. One endpoint then
	// records fresh usage afterwards. One hundred and twenty-eight makes the odds
	// that SCAN happens to name the dirty key first small enough to ignore.
	const settled = 128
	for index := 0; index < settled; index++ {
		id := fmt.Sprintf("ep_settled_%03d", index)
		if err := store.Add(ctx, id, domain.QuotaWindowDaily, 10, resets); err != nil {
			t.Fatalf("Add(%s) error = %v", id, err)
		}
	}
	seeded, err := store.Pending(ctx, settled+1)
	if err != nil {
		t.Fatalf("Pending() after seeding error = %v", err)
	}
	if len(seeded) != settled {
		t.Fatalf("Pending(%d) = %d windows, want the %d seeded ones", settled+1, len(seeded), settled)
	}
	if err := store.Settle(ctx, seeded); err != nil {
		t.Fatalf("Settle() error = %v", err)
	}

	if err := store.Add(ctx, "ep_dirty", domain.QuotaWindowDaily, 7, resets); err != nil {
		t.Fatalf("Add(ep_dirty) error = %v", err)
	}

	pending, err := store.Pending(ctx, 1)
	if err != nil {
		t.Fatalf("Pending(1) error = %v", err)
	}
	if len(pending) != 1 || pending[0].EndpointID() != "ep_dirty" {
		t.Fatalf("Pending(1) = %+v, want exactly the dirty endpoint's window", pending)
	}
}
