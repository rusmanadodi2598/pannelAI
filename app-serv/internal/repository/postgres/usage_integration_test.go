//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/usage_integration_test.go
// @for       Integration tests for usage recording and the summary aggregation.
// @uses      internal/domain, context, testing, time.
// @reason    The aggregation is one set-based statement whose correctness
//
//	depends on SQL semantics a stub cannot reproduce: numeric
//	summation that must stay exact, and a group-by whose column comes
//	from a closed set. The zero-record range is the case a stub makes
//	look right and PostgreSQL makes look empty.
//
//	The file carries an `integration` build tag so the default
//	`go test ./...` stays hermetic; with the tag active the DSN is
//	required, not optional (AGENTS.md §2.1 forbids a silent skip).
//
//	  PANNELAI_TEST_POSTGRES_DSN='postgres://...' \
//	    go test -race -tags=integration ./internal/repository/postgres/
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-18
package postgres

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestUsageRepository_SummaryArithmetic covers the aggregation totals at the
// boundaries: an empty range, a single row, several rows whose exact costs a
// float would drift on, and an all-failed range.
func TestUsageRepository_SummaryArithmetic(t *testing.T) {
	repo := newUsageRepo(t)
	filter := usageRange()
	ctx := context.Background()

	// The empty range is the case a stub makes look right and PostgreSQL makes
	// look like a row of nulls; the coalesces must turn it into zeros.
	t.Run("zero records yield zeros rather than nulls", func(t *testing.T) {
		totals, groups, err := repo.Summary(ctx, filter, "")
		if err != nil {
			t.Fatalf("Summary error = %v", err)
		}
		if totals.Requests != 0 || totals.TokensIn != 0 || totals.TokensOut != 0 ||
			totals.TokensCacheRead != 0 || totals.TokensCacheWrite != 0 ||
			totals.CostUSD != "0.00000000" || totals.LatencyMS != 0 || totals.ErrorCount != 0 {
			t.Fatalf("empty range totals = %+v, want zeros", totals)
		}
		if len(groups) != 0 {
			t.Fatalf("empty range groups = %v, want none", groups)
		}
	})

	seedUsage(t, repo, "req_1", "openai", "gpt-4o", "0.10000000", 10, domain.UsageStatusSuccess)
	seedUsage(t, repo, "req_2", "openai", "gpt-4o", "0.20000000", 20, domain.UsageStatusSuccess)
	seedUsage(t, repo, "req_3", "anthropic", "claude", "0.30000000", 30, domain.UsageStatusError)

	t.Run("totals sum exactly", func(t *testing.T) {
		totals, _, err := repo.Summary(ctx, filter, "")
		if err != nil {
			t.Fatalf("Summary error = %v", err)
		}
		if totals.Requests != 3 {
			t.Fatalf("Requests = %d, want 3", totals.Requests)
		}
		if totals.TokensIn != 60 {
			t.Fatalf("TokensIn = %d, want 60", totals.TokensIn)
		}
		// 0.1 + 0.2 + 0.3 in float64 is 0.6000000000000001; this must be exact.
		if totals.CostUSD != "0.60000000" {
			t.Fatalf("CostUSD = %q, want the exact 0.60000000", totals.CostUSD)
		}
		if totals.ErrorCount != 1 {
			t.Fatalf("ErrorCount = %d, want 1", totals.ErrorCount)
		}
		if totals.LatencyMS != 300 {
			t.Fatalf("LatencyMS = %d, want 300", totals.LatencyMS)
		}
		if totals.LatencyP50MS == 0 || totals.LatencyP95MS == 0 {
			t.Fatalf("percentiles = (%d, %d), want real figures", totals.LatencyP50MS, totals.LatencyP95MS)
		}
	})

	t.Run("a provider filter narrows the totals", func(t *testing.T) {
		narrowed := filter
		narrowed.ProviderID = "openai"
		totals, _, err := repo.Summary(ctx, narrowed, "")
		if err != nil {
			t.Fatalf("Summary error = %v", err)
		}
		if totals.Requests != 2 || totals.TokensIn != 30 || totals.CostUSD != "0.30000000" {
			t.Fatalf("filtered totals = %+v, want 2 requests and 0.30000000", totals)
		}
	})

	t.Run("a status filter narrows to the failures", func(t *testing.T) {
		failed := filter
		failed.Status = "error"
		totals, _, err := repo.Summary(ctx, failed, "")
		if err != nil {
			t.Fatalf("Summary error = %v", err)
		}
		if totals.Requests != 1 || totals.ErrorCount != 1 {
			t.Fatalf("failed totals = %+v, want the single error row", totals)
		}
	})
}

// TestUsageRepository_GroupByDimensions covers every dimension the contract
// offers, plus the rejection of one it does not. The dimension set is closed,
// and it is the only string this package interpolates into a statement, so the
// rejection path is the security-relevant one.
func TestUsageRepository_GroupByDimensions(t *testing.T) {
	repo := newUsageRepo(t)
	ctx := context.Background()
	seedUsage(t, repo, "req_a", "openai", "gpt-4o", "0.10000000", 10, domain.UsageStatusSuccess)
	seedUsage(t, repo, "req_b", "openai", "gpt-4o", "0.20000000", 20, domain.UsageStatusSuccess)
	seedUsage(t, repo, "req_c", "anthropic", "claude", "0.40000000", 40, domain.UsageStatusSuccess)

	cases := []struct {
		name        string
		groupBy     domain.UsageGroupBy
		wantKeys    map[string]int64
		wantGrouped int
	}{
		{"provider", domain.UsageGroupProvider, map[string]int64{"openai": 2, "anthropic": 1}, 2},
		{"model", domain.UsageGroupModel, map[string]int64{"gpt-4o": 2, "claude": 1}, 2},
		{"endpoint groups the empty endpoint id together", domain.UsageGroupEndpoint, map[string]int64{"": 3}, 1},
		{"gateway key groups the empty key id together", domain.UsageGroupGatewayKey, map[string]int64{"": 3}, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			totals, groups, err := repo.Summary(ctx, usageRange(), tc.groupBy)
			if err != nil {
				t.Fatalf("Summary error = %v", err)
			}
			if len(groups) != tc.wantGrouped {
				t.Fatalf("groups = %d, want %d (%+v)", len(groups), tc.wantGrouped, groups)
			}
			for key, requests := range tc.wantKeys {
				found := false
				for _, group := range groups {
					if group.Key == key {
						found = true
						if group.Totals.Requests != requests {
							t.Fatalf("group %q requests = %d, want %d", key, group.Totals.Requests, requests)
						}
					}
				}
				if !found {
					t.Fatalf("group %q is missing from %+v", key, groups)
				}
			}
			if totals.Requests != 3 {
				t.Fatalf("ungrouped totals = %d, want 3", totals.Requests)
			}
		})
	}

	t.Run("an unknown dimension is refused before any statement runs", func(t *testing.T) {
		if _, _, err := repo.Summary(ctx, usageRange(), domain.UsageGroupBy("status")); err == nil {
			t.Fatal("Summary with an unknown group_by = nil error, want a rejection")
		}
	})
}
