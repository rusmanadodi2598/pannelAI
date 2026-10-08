//go:build integration

// Integration test for the quota collection's account page.
//
// @file      internal/repository/postgres/quota_paging_accounts_integration_test.go
// @for       The account side of the quota page against a real database: who the page carries, what it costs, and when it says it was cut.
// @uses      context, strconv, strings, testing, time, pgxpool, internal/domain.
// @reason    The account read decides which cards exist, and a fake cannot show that: an endpoint with no counted row appears in no window, the page owes two statements however many accounts it holds, and the row ceiling is a fact only a real page of more accounts than the ceiling can prove.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-10-08
package postgres

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// seedPagingAccounts creates one account per id under the given provider, and counts a
// window only for the ids listed in counted. An id absent from counted is the case
// this file exists for: an account the gateway knows and no counted row names.
func seedPagingAccounts(t *testing.T, pool *pgxpool.Pool, provider string, ids []string, counted []string) {
	t.Helper()

	ctx := context.Background()
	endpoints := NewEndpointRepository(pool)
	for _, id := range ids {
		endpoint, err := domain.NewUpstreamEndpoint(id, provider, "seed "+id, domain.UpstreamAuthAPIKey, 1, time.Now())
		if err != nil {
			t.Fatalf("NewUpstreamEndpoint(%s) error = %v", id, err)
		}
		if err := endpoints.Create(ctx, endpoint); err != nil {
			t.Fatalf("Create(%s) error = %v", id, err)
		}
	}
	if len(counted) == 0 {
		return
	}
	// One statement rather than one per row, because this seeds the page the ceiling
	// test measures (AGENTS.md §1.7).
	placeholders := make([]string, 0, len(counted))
	args := make([]any, 0, len(counted))
	for index, id := range counted {
		placeholders = append(placeholders, "($"+strconv.Itoa(index+1)+", '5h', 1, 'computed')")
		args = append(args, id)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO quota_windows (endpoint_id, "window", used_units, source) VALUES `+
			strings.Join(placeholders, ","), args...); err != nil {
		t.Fatalf("seeding counted windows: %v", err)
	}
}

// TestQuotaRepository_PageAccountsIncludesAnAccountWithNoWindows is the reason the
// account read exists: a provider whose account has routed nothing yet is configured,
// polled, and still renders no card when the page is built from counted rows. The
// window page must keep ignoring that account, because it counts what it counted.
func TestQuotaRepository_PageAccountsIncludesAnAccountWithNoWindows(t *testing.T) {
	pool := newPagingPool(t)
	ctx := context.Background()
	quota := NewQuotaRepository(pool)

	seedPagingAccounts(t, pool, "alpha", []string{"ep_alpha_1", "ep_alpha_2"}, []string{"ep_alpha_1", "ep_alpha_2"})
	seedPagingAccounts(t, pool, "beta", []string{"ep_beta_1"}, nil)

	windows, windowTotal, _, err := quota.PageWindowsByProvider(ctx, 1, 10)
	if err != nil {
		t.Fatalf("PageWindowsByProvider() error = %v", err)
	}
	if len(windows) != 2 {
		t.Fatalf("window page = %d rows, want the two counted alpha rows", len(windows))
	}
	for _, window := range windows {
		if window.EndpointID() == "ep_beta_1" {
			t.Fatal("a never-counted account appeared in the counted rows")
		}
	}

	accounts, accountTotal, _, err := quota.PageAccountsByProvider(ctx, 1, 10)
	if err != nil {
		t.Fatalf("PageAccountsByProvider() error = %v", err)
	}
	if accountTotal != windowTotal {
		t.Fatalf("the account page counts %d groups and the window page %d; one page number must mean one set of groups",
			accountTotal, windowTotal)
	}

	byID := map[string]string{}
	for _, account := range accounts {
		byID[account.EndpointID] = account.ProviderID
	}
	if len(byID) != len(accounts) {
		t.Fatalf("the account page repeated an endpoint: %v", accounts)
	}
	// The point of the read: beta is named here although no counted row names it.
	if provider, ok := byID["ep_beta_1"]; !ok || provider != "beta" {
		t.Fatalf("account page = %v, want ep_beta_1 under beta: an account with no traffic must still get a card", accounts)
	}
	for _, id := range []string{"ep_alpha_1", "ep_alpha_2"} {
		if byID[id] != "alpha" {
			t.Fatalf("account page = %v, want %s under alpha", accounts, id)
		}
	}
}

// TestQuotaRepository_PageAccountsIsCountPlusPageOnly pins the statement cost of the
// account read. The page ceiling is a probe row on the same statement, not a third
// query, so a screen read stays constant-cost however many accounts a provider holds
// (AGENTS.md §1.7).
func TestQuotaRepository_PageAccountsIsCountPlusPageOnly(t *testing.T) {
	published, counter := newPublishedRepo(t)
	ctx := context.Background()
	if _, err := published.pool.Exec(ctx, `TRUNCATE quota_windows`); err != nil {
		t.Fatalf("truncating quota_windows: %v", err)
	}
	seedPagingAccounts(t, published.pool, "alpha", []string{"ep_alpha_1", "ep_alpha_2"}, []string{"ep_alpha_1", "ep_alpha_2"})
	seedPagingAccounts(t, published.pool, "beta", []string{"ep_beta_1"}, nil)

	quota := NewQuotaRepository(published.pool)
	counter.reset()
	if _, _, _, err := quota.PageAccountsByProvider(ctx, 1, 10); err != nil {
		t.Fatalf("PageAccountsByProvider() error = %v", err)
	}
	if got := counter.load(); got != 2 {
		t.Fatalf("the account page sent %d statements (%s), want 2: one group count and one page, whatever the page holds",
			got, counter.trace())
	}
}

// TestQuotaRepository_PageAccountsReportsACeilingCut is the flag's own case. The page
// unit is the group and the ceiling is the rows, so one provider holding more accounts
// than the ceiling serves a page that is not the whole group, and the caller can only
// learn that from the read itself.
func TestQuotaRepository_PageAccountsReportsACeilingCut(t *testing.T) {
	pool := newPagingPool(t)
	quota := NewQuotaRepository(pool)

	// The no-provider lane: window rows with no endpoint behind them are accounts the
	// page must carry, and one bulk statement seeds more of them than the ceiling.
	const rows = quotaMaxRowsPerPage + 7
	if _, err := pool.Exec(context.Background(), `
INSERT INTO quota_windows (endpoint_id, "window", used_units, source)
SELECT 'ep_bulk_' || g, '5h', 1, 'computed' FROM generate_series(1, $1::int) g`, rows); err != nil {
		t.Fatalf("seeding %d accounts: %v", rows, err)
	}

	accounts, total, truncated, err := quota.PageAccountsByProvider(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("PageAccountsByProvider() error = %v", err)
	}
	if !truncated {
		t.Fatalf("truncated = false with %d accounts held in one group, want the cut reported", rows)
	}
	if len(accounts) != quotaMaxRowsPerPage {
		t.Fatalf("accounts = %d, want exactly the ceiling %d, not the probe row", len(accounts), quotaMaxRowsPerPage)
	}
	// The group total still counts one group: the ceiling cut rows, not providers.
	if total != 1 {
		t.Fatalf("total = %d, want the one group the page carries", total)
	}
}
