//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/quota_integration_test.go
// @for       Quota window persistence against a real server: the set-based
//
//	upsert, the conflict target, and the nil-limit distinction.
//
// @uses      testing, context, math, time, internal/domain.
// @reason    The flush statement names a reserved word as a column, so only a
//
//	real parser proves it is quoted correctly, and only the primary key
//	proves the ON CONFLICT target matches the migration. The service
//	tests stub UpsertWindows, so a broken statement stays green there
//	(audit-001 finding 15).
//
//	Run with:
//	  PANNELAI_TEST_POSTGRES_DSN='postgres://...' \
//	    go test -race -tags=integration ./internal/repository/postgres/
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-23
package postgres

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// newTestQuotaRepo returns a repository over a clean window table.
func newTestQuotaRepo(t *testing.T) *QuotaRepository {
	t.Helper()
	pool := newTestPool(t)
	if _, err := pool.Exec(context.Background(), `TRUNCATE quota_windows`); err != nil {
		t.Fatalf("truncating: %v", err)
	}
	return NewQuotaRepository(pool)
}

// mustWindow opens one window with the given usage, or fails the test.
func mustWindow(t *testing.T, endpointID string, kind domain.QuotaWindowKind, used int64, limit *int64) domain.QuotaWindow {
	t.Helper()
	window, err := domain.NewQuotaWindow(endpointID, "", kind, limit, nil, time.Now())
	if err != nil {
		t.Fatalf("NewQuotaWindow(%s/%s) error = %v", endpointID, kind, err)
	}
	window.Add(used, time.Now())
	return window
}

// TestQuotaRepository_UpsertThenListRoundTrip drives one flush-shaped batch
// through the statement the worker actually runs: four windows, two endpoints,
// limits at a typical value, zero, nil, and the int64 ceiling.
func TestQuotaRepository_UpsertThenListRoundTrip(t *testing.T) {
	repo := newTestQuotaRepo(t)
	ctx := context.Background()

	typical, zero, huge := int64(1000), int64(0), int64(math.MaxInt64)
	batch := []domain.QuotaWindow{
		mustWindow(t, "ep_a", domain.QuotaWindowFiveHour, 0, &typical),
		mustWindow(t, "ep_a", domain.QuotaWindowDaily, 42, &zero),
		mustWindow(t, "ep_a", domain.QuotaWindowWeekly, 7, nil),
		mustWindow(t, "ep_a", domain.QuotaWindowMonthly, math.MaxInt64, &huge),
		mustWindow(t, "ep_b", domain.QuotaWindowDaily, 1, &typical),
	}
	if err := repo.UpsertWindows(ctx, batch); err != nil {
		t.Fatalf("UpsertWindows() error = %v", err)
	}

	all, err := repo.ListWindows(ctx, "")
	if err != nil {
		t.Fatalf("ListWindows(all) error = %v", err)
	}
	if len(all) != len(batch) {
		t.Fatalf("ListWindows(all) returned %d rows, want %d", len(all), len(batch))
	}

	// Ordered by endpoint then window text: 5h, daily, monthly, weekly.
	wantOrder := []domain.QuotaWindowKind{
		domain.QuotaWindowFiveHour, domain.QuotaWindowDaily,
		domain.QuotaWindowMonthly, domain.QuotaWindowWeekly,
	}
	for i, want := range wantOrder {
		if all[i].EndpointID() != "ep_a" || all[i].Window() != want {
			t.Fatalf("ListWindows(all)[%d] = %s/%s, want ep_a/%s", i, all[i].EndpointID(), all[i].Window(), want)
		}
	}

	byKind := make(map[domain.QuotaWindowKind]domain.QuotaWindow, len(all))
	for _, window := range all {
		if window.EndpointID() == "ep_a" {
			byKind[window.Window()] = window
		}
	}
	monthly := byKind[domain.QuotaWindowMonthly]
	if limit, _ := monthly.Limit(); limit != math.MaxInt64 {
		t.Fatalf("monthly Limit() = %d, want the int64 ceiling", limit)
	}
	if got := monthly.Used(); got != math.MaxInt64 {
		t.Fatalf("monthly Used() = %d, want %d", got, int64(math.MaxInt64))
	}

	// A nil limit must read back as "uncapped", not as a cap of zero: the two
	// are different rules for the router.
	if _, ok := byKind[domain.QuotaWindowWeekly].Limit(); ok {
		t.Fatalf("weekly Limit() reported capped, want uncapped for a NULL limit")
	}
	if limit, ok := byKind[domain.QuotaWindowDaily].Limit(); !ok || limit != 0 {
		t.Fatalf("daily Limit() = %d/%v, want a stored cap of zero", limit, ok)
	}

	// The endpoint filter must narrow the same query rather than post-filter.
	scoped, err := repo.ListWindows(ctx, "ep_b")
	if err != nil {
		t.Fatalf("ListWindows(ep_b) error = %v", err)
	}
	if len(scoped) != 1 || scoped[0].EndpointID() != "ep_b" {
		t.Fatalf("ListWindows(ep_b) = %+v, want only ep_b", scoped)
	}
}

// TestQuotaRepository_UpsertReplacesRatherThanAdds pins the conflict path: a
// retried flush writes the same key again and must replace the counter, because
// the Redis counter is authoritative until it is cleared.
func TestQuotaRepository_UpsertReplacesRatherThanAdds(t *testing.T) {
	repo := newTestQuotaRepo(t)
	ctx := context.Background()
	limit := int64(500)

	if err := repo.UpsertWindows(ctx, []domain.QuotaWindow{
		mustWindow(t, "ep_a", domain.QuotaWindowDaily, 3, &limit),
	}); err != nil {
		t.Fatalf("UpsertWindows() error = %v", err)
	}
	if err := repo.UpsertWindows(ctx, []domain.QuotaWindow{
		mustWindow(t, "ep_a", domain.QuotaWindowDaily, 7, &limit),
	}); err != nil {
		t.Fatalf("UpsertWindows() again error = %v", err)
	}

	windows, err := repo.ListWindows(ctx, "ep_a")
	if err != nil {
		t.Fatalf("ListWindows() error = %v", err)
	}
	if len(windows) != 1 {
		t.Fatalf("ListWindows() returned %d rows, want 1: the upsert must replace", len(windows))
	}
	if got := windows[0].Used(); got != 7 {
		t.Fatalf("Used() = %d, want 7", got)
	}
}

// TestQuotaRepository_UpsertEmptyBatchIsANoOp pins that a batch with nothing to
// flush costs no round trip and no error.
func TestQuotaRepository_UpsertEmptyBatchIsANoOp(t *testing.T) {
	repo := newTestQuotaRepo(t)
	if err := repo.UpsertWindows(context.Background(), nil); err != nil {
		t.Fatalf("UpsertWindows(nil) error = %v", err)
	}
}
