//go:build integration

// Package redisrepo implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/quota_counter_test.go
// @for       Integration tests for the quota counter against a real Redis: the
//
//	running window total, its rollover, and the settle rule.
//
// @uses      github.com/redis/go-redis/v9, internal/domain, context, os,
//
//	testing, time.
//
// @reason    The counter's whole contract is Redis behaviour: an atomic
//
//	rollover and a conditional retire, neither of which an in-memory
//	double can prove. A double would pass while the real script
//	compared the wrong field or deleted a live window, and the loss is
//	silent: the window simply reads low.
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
// @since     2026-09-23
package redisrepo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// quotaCounterFixture returns a store over a flushed Redis, so each case reads a
// keyspace that belongs to it alone.
func quotaCounterFixture(t *testing.T) (*QuotaCounterStore, context.Context) {
	t.Helper()
	raw := os.Getenv(testRedisEnv)
	if raw == "" {
		t.Fatalf("%s must be set to run the quota counter tests", testRedisEnv)
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
	return NewQuotaCounterStore(client), ctx
}

// pendingFor reads one window kind back, reporting whether the flush would
// write it, so a case can assert absence rather than a zero.
func pendingFor(t *testing.T, store *QuotaCounterStore, ctx context.Context, endpointID string, kind domain.QuotaWindowKind) (domain.QuotaWindow, bool) {
	t.Helper()
	pending, err := store.Pending(ctx, 10)
	if err != nil {
		t.Fatalf("Pending() error = %v", err)
	}
	for _, window := range pending {
		if window.EndpointID() == endpointID && window.Window() == kind {
			return window, true
		}
	}
	return domain.QuotaWindow{}, false
}

// TestQuotaCounterStore_AccumulatesAcrossFlushes is the defect this file exists
// for: a flush must not lose the running total. The counter is the window's
// total in Redis and PostgreSQL mirrors it, so settling a flushed batch cannot
// reset the number the next batch continues from.
func TestQuotaCounterStore_AccumulatesAcrossFlushes(t *testing.T) {
	cases := []struct {
		name   string
		units  []int64
		wantAt []int64
	}{
		{name: "two flushes add up", units: []int64{65, 68}, wantAt: []int64{65, 133}},
		{name: "three flushes add up", units: []int64{65, 68, 67}, wantAt: []int64{65, 133, 200}},
		{name: "a single token still advances the total", units: []int64{1, 1}, wantAt: []int64{1, 2}},
		{name: "a large spend is not clamped", units: []int64{1 << 40, 1 << 40}, wantAt: []int64{1 << 40, 1 << 41}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, ctx := quotaCounterFixture(t)
			for i, units := range tc.units {
				if err := store.Add(ctx, "ep_a", domain.QuotaWindowDaily, units, time.Now().Add(domain.QuotaWindowDaily.Duration())); err != nil {
					t.Fatalf("Add(%d) error = %v", units, err)
				}
				window, ok := pendingFor(t, store, ctx, "ep_a", domain.QuotaWindowDaily)
				if !ok {
					t.Fatalf("after Add(%d) the window is not pending", units)
				}
				if got := window.Used(); got != tc.wantAt[i] {
					t.Fatalf("after Add(%d) the running total is %d, want %d", units, got, tc.wantAt[i])
				}
				// The flush writes the total, then settles what it wrote. The
				// total must survive that settle.
				if err := store.Settle(ctx, []domain.QuotaWindow{window}); err != nil {
					t.Fatalf("Settle() error = %v", err)
				}
			}
		})
	}
}

// TestQuotaCounterStore_RollsOverAnExpiredWindow pins that a window whose reset
// instant has passed starts over rather than carrying the closed window's total
// into the new one.
func TestQuotaCounterStore_RollsOverAnExpiredWindow(t *testing.T) {
	cases := []struct {
		name       string
		first      int64
		firstReset time.Duration
		second     int64
		want       int64
	}{
		{name: "an expired window restarts at the new units", first: 900, firstReset: -time.Minute, second: 5, want: 5},
		{name: "an open window keeps adding", first: 900, firstReset: time.Hour, second: 5, want: 905},
		{name: "an expired window restarts at zero usage", first: 900, firstReset: -time.Minute, second: 1, want: 1},
		{name: "the boundary instant counts as expired", first: 900, firstReset: 0, second: 7, want: 7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, ctx := quotaCounterFixture(t)
			now := time.Now()
			if err := store.Add(ctx, "ep_a", domain.QuotaWindowFiveHour, tc.first, now.Add(tc.firstReset)); err != nil {
				t.Fatalf("Add(first) error = %v", err)
			}
			if err := store.Add(ctx, "ep_a", domain.QuotaWindowFiveHour, tc.second, now.Add(time.Hour)); err != nil {
				t.Fatalf("Add(second) error = %v", err)
			}
			window, ok := pendingFor(t, store, ctx, "ep_a", domain.QuotaWindowFiveHour)
			if !ok {
				t.Fatalf("the window is not pending after two adds")
			}
			if got := window.Used(); got != tc.want {
				t.Fatalf("Used() = %d, want %d", got, tc.want)
			}
		})
	}
}
