//go:build integration

// Integration test for the quota collection's account page.
//
// @file      internal/repository/postgres/quota_paging_integration_test.go
// @for       Proves a quota page is grouped by the accounts that exist, not by the counters that happen to exist.
// @uses      context, internal/repository/postgres, testing.
// @reason    Paging cards over counted windows hid every account that had not routed a
//
//	request yet: the provider was configured, the worker had polled it, and the
//	screen still rendered no card for it. Which two reads agree about the groups
//	on a page is a question a fake database answers wrongly, so this asks a real
//	one — and pins that the statement cost of the page stays fixed.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-10-02
package postgres

import (
	"context"
	"testing"
)

// seedQuotaPagingRows creates three accounts: two under `alpha` that this gateway has
// counted, and one under `beta` that has never served a request, so no window row names
// it. Windows are emptied first because no foreign key ties them to an endpoint, and a
// leftover row from another run would add a group this test then miscounts.
func seedQuotaPagingRows(t *testing.T, published *PublishedQuotaRepository) {
	t.Helper()

	ctx := context.Background()
	if _, err := published.pool.Exec(ctx, `TRUNCATE quota_windows`); err != nil {
		t.Fatalf("truncating quota_windows: %v", err)
	}

	seedPublishedEndpoint(t, published, "ep_alpha_1", "alpha")
	seedPublishedEndpoint(t, published, "ep_alpha_2", "alpha")
	seedPublishedEndpoint(t, published, "ep_beta_1", "beta")

	// `beta` is deliberately left out: that absence is the whole case.
	const q = `INSERT INTO quota_windows (endpoint_id, "window", used_units, source)
VALUES ('ep_alpha_1', '5h', 3, 'computed'), ('ep_alpha_2', '5h', 4, 'computed')`
	if _, err := published.pool.Exec(ctx, q); err != nil {
		t.Fatalf("seeding windows: %v", err)
	}
}

func TestQuotaRepository_PageAccountsIncludesAnAccountWithNoWindows(t *testing.T) {
	published, _ := newPublishedRepo(t)
	repo := NewQuotaRepository(published.pool)
	seedQuotaPagingRows(t, published)
	ctx := context.Background()

	windows, windowTotal, err := repo.PageWindowsByProvider(ctx, 1, 10)
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

	accounts, accountTotal, err := repo.PageAccountsByProvider(ctx, 1, 10)
	if err != nil {
		t.Fatalf("PageAccountsByProvider() error = %v", err)
	}
	if accountTotal != windowTotal {
		t.Fatalf("the account page counts %d groups and the window page %d; one page number must mean one set of groups", accountTotal, windowTotal)
	}

	byID := map[string]string{}
	for _, account := range accounts {
		byID[account.EndpointID] = account.ProviderID
	}
	if len(byID) != len(accounts) {
		t.Fatalf("the account page repeated an endpoint: %v", accounts)
	}
	// The point of the change: beta is named here although no counted row names it.
	if provider, ok := byID["ep_beta_1"]; !ok || provider != "beta" {
		t.Fatalf("account page = %v, want ep_beta_1 under beta — an account with no traffic must still get a card", accounts)
	}
	for _, id := range []string{"ep_alpha_1", "ep_alpha_2"} {
		if byID[id] != "alpha" {
			t.Fatalf("account page = %v, want %s under alpha", accounts, id)
		}
	}
}

func TestQuotaRepository_PageAccountsIsCountPlusPageOnly(t *testing.T) {
	published, counter := newPublishedRepo(t)
	repo := NewQuotaRepository(published.pool)
	seedQuotaPagingRows(t, published)

	counter.reset()
	if _, _, err := repo.PageAccountsByProvider(context.Background(), 1, 10); err != nil {
		t.Fatalf("PageAccountsByProvider() error = %v", err)
	}
	if got := counter.load(); got != 2 {
		t.Fatalf("the account page sent %d statements (%s), want 2: one group count and one page, whatever the page holds", got, counter.trace())
	}
}
