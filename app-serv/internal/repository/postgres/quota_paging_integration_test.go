//go:build integration

// Integration test for the quota collection's paging reads.
//
// @file      internal/repository/postgres/quota_paging_integration_test.go
// @for       The provider-grouped paging of quota windows and accounts against a real database.
// @uses      context, strconv, strings, testing, time, pgxpool, internal/domain, migrations.
// @reason    PageWindowsByProvider and PageAccountsByProvider count groups and then read what belongs to them, and only a live PostgreSQL can show that a page boundary cannot drop a group the count promised, that an account with no counted row still reaches the page, and that the row ceiling reports itself instead of leaving a gap that looks like an idle account.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-10-04
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/migrations"
)

// newPagingPool applies the migrations over an emptied schema and hands back the pool
// the paging reads run against. upstream_endpoints is truncated with CASCADE so the
// window rows that reference it go too, and quota_windows is emptied on its own
// because no foreign key ties an account-less window row to an endpoint.
func newPagingPool(t *testing.T) *pgxpool.Pool {
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

	if _, err := pool.Exec(ctx, `TRUNCATE upstream_endpoints CASCADE`); err != nil {
		t.Fatalf("truncating upstream_endpoints: %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE quota_windows`); err != nil {
		t.Fatalf("truncating quota_windows: %v", err)
	}
	return pool
}

// TestQuotaRepository_PageWindowsByProvider_GroupsAndCounts drives the paged
// collection read against a real server (docs/PORT/006-PORT-QUOTA-PAGING.md
// D1-D3): the page unit is the provider group, groups are ordered by their
// smallest endpoint id, the no-provider lane counts as one group, and the
// total is honest past the last page.
func TestQuotaRepository_PageWindowsByProvider_GroupsAndCounts(t *testing.T) {
	pool := newPagingPool(t)
	ctx := context.Background()
	now := time.Now()

	endpoints := NewEndpointRepository(pool)
	quota := NewQuotaRepository(pool)

	// Three endpoint rows give two real provider groups; ep_y and ep_z get no
	// endpoint row at all, which is the shape the LEFT JOIN renders as the
	// no-provider lane (the credential-free virtual endpoint).
	for _, seed := range []struct{ id, provider, label string }{
		{"ep_a", "alpha", "Alpha one"},
		{"ep_b", "alpha", "Alpha two"},
		{"ep_c", "bravo", "Bravo one"},
	} {
		endpoint, err := domain.NewUpstreamEndpoint(seed.id, seed.provider, seed.label, domain.UpstreamAuthAPIKey, 1, now)
		if err != nil {
			t.Fatalf("NewUpstreamEndpoint(%s) error = %v", seed.id, err)
		}
		if err := endpoints.Create(ctx, endpoint); err != nil {
			t.Fatalf("Create(%s) error = %v", seed.id, err)
		}
	}

	batch := []domain.QuotaWindow{
		mustWindow(t, "ep_a", domain.QuotaWindowMonthly, 1, nil),
		mustWindow(t, "ep_a", domain.QuotaWindowDaily, 2, nil),
		mustWindow(t, "ep_b", domain.QuotaWindowMonthly, 3, nil),
		mustWindow(t, "ep_c", domain.QuotaWindowMonthly, 4, nil),
		mustWindow(t, "ep_c", domain.QuotaWindowDaily, 5, nil),
		mustWindow(t, "ep_y", domain.QuotaWindowMonthly, 6, nil),
		mustWindow(t, "ep_z", domain.QuotaWindowDaily, 7, nil),
	}
	if err := quota.UpsertWindows(ctx, batch); err != nil {
		t.Fatalf("UpsertWindows() error = %v", err)
	}

	// Page 1 (two groups): alpha's three windows and bravo's two, ordered by
	// endpoint then window, with the total naming all three groups.
	windows, total, truncated, err := quota.PageWindowsByProvider(ctx, 1, 2)
	if err != nil {
		t.Fatalf("PageWindowsByProvider(1,2) error = %v", err)
	}
	if truncated {
		t.Fatalf("a five-row page reports the ceiling cut it, so the probe row is being miscounted")
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3 groups (alpha, bravo, no-provider)", total)
	}
	if len(windows) != 5 {
		t.Fatalf("page 1 returned %d windows, want 5 (alpha and bravo whole)", len(windows))
	}
	if windows[0].EndpointID() != "ep_a" || windows[0].Window() != domain.QuotaWindowDaily {
		t.Fatalf("page 1 row 0 = %s/%s, want ep_a/daily", windows[0].EndpointID(), windows[0].Window())
	}
	if windows[4].EndpointID() != "ep_c" {
		t.Fatalf("page 1 row 4 = %s, want the last bravo row", windows[4].EndpointID())
	}

	// Page 2: the no-provider group, joined through the missing endpoint rows.
	windows, total, _, err = quota.PageWindowsByProvider(ctx, 2, 2)
	if err != nil {
		t.Fatalf("PageWindowsByProvider(2,2) error = %v", err)
	}
	if total != 3 || len(windows) != 2 {
		t.Fatalf("page 2 = %d windows (total %d), want the two no-provider rows (total 3)", len(windows), total)
	}

	// Past the end: an empty page that still tells the truth about the size.
	windows, total, truncated, err = quota.PageWindowsByProvider(ctx, 9, 2)
	if err != nil {
		t.Fatalf("PageWindowsByProvider(9,2) error = %v", err)
	}
	if total != 3 || len(windows) != 0 {
		t.Fatalf("page 9 = %d windows (total %d), want an empty page with total 3", len(windows), total)
	}
	if truncated {
		t.Fatal("an empty page was not cut by anything")
	}
}
