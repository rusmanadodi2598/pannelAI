//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/usage_read_integration_test.go
// @for       Integration tests for the timeseries, paged list, detail, and
//
//	monthly rollup reads.
//
// @uses      internal/domain, context, testing, time.
// @reason    The timeseries bucket boundary is the database's date_bin, and the
//
//	paged read's count comes from a window function in the same
//	statement: neither is reproducible against a stub, and the page past
//	the last row is the case a stub makes look right and PostgreSQL
//	makes return nothing.
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
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestUsageRepository_Timeseries covers the two granularities and the
// zero-record range, at the bucket boundary.
func TestUsageRepository_Timeseries(t *testing.T) {
	repo := newUsageRepo(t)
	ctx := context.Background()
	seedUsage(t, repo, "req_1", "openai", "gpt-4o", "0.10000000", 10, domain.UsageStatusSuccess)
	seedUsage(t, repo, "req_2", "openai", "gpt-4o", "0.20000000", 20, domain.UsageStatusSuccess)

	cases := []struct {
		name        string
		granularity domain.UsageGranularity
		wantBuckets int
		wantErr     bool
	}{
		{"hourly buckets the seeded records into one point", domain.UsageHourly, 1, false},
		{"daily buckets them into one point", domain.UsageDaily, 1, false},
		{"an unknown granularity is refused", domain.UsageGranularity("minute"), 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buckets, err := repo.Timeseries(ctx, usageRange(), tc.granularity)
			if tc.wantErr {
				if err == nil {
					t.Fatal("Timeseries = nil error, want a rejection")
				}
				return
			}
			if err != nil {
				t.Fatalf("Timeseries error = %v", err)
			}
			if len(buckets) != tc.wantBuckets {
				t.Fatalf("buckets = %d, want %d (%+v)", len(buckets), tc.wantBuckets, buckets)
			}
			if len(buckets) > 0 && buckets[0].Totals.Requests != 2 {
				t.Fatalf("bucket requests = %d, want 2", buckets[0].Totals.Requests)
			}
		})
	}

	t.Run("a range with no records yields no buckets rather than zeroes", func(t *testing.T) {
		from := time.Now().UTC().Add(-48 * time.Hour)
		to := time.Now().UTC().Add(-47 * time.Hour)
		empty := domain.NewUsageFilter(domain.UsageFilterInput{From: &from, To: &to}, time.Now())
		buckets, err := repo.Timeseries(ctx, empty, domain.UsageHourly)
		if err != nil {
			t.Fatalf("Timeseries error = %v", err)
		}
		if len(buckets) != 0 {
			t.Fatalf("buckets = %+v, want none for an empty range", buckets)
		}
	})
}

// TestUsageRepository_ListAndDetail covers the paged read, its count agreement
// at the page boundaries, and the single-request detail.
func TestUsageRepository_ListAndDetail(t *testing.T) {
	repo := newUsageRepo(t)
	ctx := context.Background()
	for _, id := range []string{"req_1", "req_2", "req_3", "req_4", "req_5"} {
		seedUsage(t, repo, id, "openai", "gpt-4o", "0.01000000", 5, domain.UsageStatusSuccess)
	}

	cases := []struct {
		name      string
		page      int
		perPage   int
		wantRows  int
		wantTotal int64
	}{
		{"first page", 1, 2, 2, 5},
		{"second page", 2, 2, 2, 5},
		{"last partial page", 3, 2, 1, 5},
		{"a page past the last row still reports the total", 9, 2, 0, 5},
		{"a page of one", 1, 1, 1, 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			records, total, err := repo.List(ctx, usageRange(), pageQuery(tc.page, tc.perPage))
			if err != nil {
				t.Fatalf("List error = %v", err)
			}
			if len(records) != tc.wantRows {
				t.Fatalf("rows = %d, want %d", len(records), tc.wantRows)
			}
			if total != tc.wantTotal {
				t.Fatalf("total = %d, want %d", total, tc.wantTotal)
			}
		})
	}

	t.Run("detail returns the stored record", func(t *testing.T) {
		record, err := repo.GetByRequestID(ctx, "req_3")
		if err != nil {
			t.Fatalf("GetByRequestID error = %v", err)
		}
		if record.RequestID() != "req_3" || record.Model() != "gpt-4o" || record.CostUSD() != "0.01000000" {
			t.Fatalf("record = %+v, want the seeded row", record)
		}
	})

	t.Run("a missing request yields the not-found sentinel", func(t *testing.T) {
		if _, err := repo.GetByRequestID(ctx, "req_missing"); !errorsIs(err, domain.ErrUsageRecordNotFound) {
			t.Fatalf("GetByRequestID error = %v, want ErrUsageRecordNotFound", err)
		}
	})

	t.Run("recording the same row twice does not double-count", func(t *testing.T) {
		record, err := repo.GetByRequestID(ctx, "req_1")
		if err != nil {
			t.Fatalf("GetByRequestID error = %v", err)
		}
		if err := repo.Record(ctx, record); err != nil {
			t.Fatalf("second Record error = %v", err)
		}
		_, total, err := repo.List(ctx, usageRange(), pageQuery(1, 100))
		if err != nil {
			t.Fatalf("List error = %v", err)
		}
		if total != 5 {
			t.Fatalf("total = %d, want 5: an identical record must not be stored twice", total)
		}
	})
}

// TestUsageRepository_MonthlyUsage covers the rollup a budget cap is compared
// against, at the month boundary. The cap is monthly, so a record from two
// months ago must not count: a rollup that summed every row would skip an
// endpoint whose cap it never reached.
func TestUsageRepository_MonthlyUsage(t *testing.T) {
	repo := newUsageRepo(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// One record inside this month and one before it, so the window is what the
	// test measures rather than the row count.
	inside := domain.UsageRecordInput{RequestID: "req_in", ProviderID: "openai", Model: "gpt", Status: domain.UsageStatusSuccess, TokensIn: 10, CostUSD: "0.1", TS: now}
	before := domain.UsageRecordInput{RequestID: "req_out", ProviderID: "openai", Model: "gpt", Status: domain.UsageStatusSuccess, TokensIn: 999, CostUSD: "9.9", TS: now.AddDate(0, -2, 0)}
	for _, in := range []domain.UsageRecordInput{inside, before} {
		record, err := domain.NewUsageRecord(in, "", now)
		if err != nil {
			t.Fatalf("building %s: %v", in.RequestID, err)
		}
		if err := repo.Record(ctx, record); err != nil {
			t.Fatalf("recording %s: %v", in.RequestID, err)
		}
	}

	// Every endpoint: only the in-month row counts.
	totals, err := repo.MonthlyUsage(ctx, "", now)
	if err != nil {
		t.Fatalf("MonthlyUsage error = %v", err)
	}
	if totals.Requests != 1 {
		t.Fatalf("Requests = %d, want 1: the row from two months ago is outside the monthly window", totals.Requests)
	}
	if totals.TokensIn != 10 || totals.CostUSD != "0.10000000" {
		t.Fatalf("totals = %+v, want only the in-month row", totals)
	}

	// An endpoint-scoped rollup sees only its own rows.
	scoped, err := repo.MonthlyUsage(ctx, "ep_absent", now)
	if err != nil {
		t.Fatalf("MonthlyUsage error = %v", err)
	}
	if scoped.Requests != 0 || scoped.CostUSD != "0.00000000" {
		t.Fatalf("scoped totals = %+v, want zeros", scoped)
	}
}
