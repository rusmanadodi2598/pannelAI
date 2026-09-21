//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/usage_status_filter_integration_test.go
// @for       The status filter's closed set against a real server (draft 010 F2).
// @uses      internal/domain, context, testing, time.
// @reason    Draft 010 F2: the status predicate is `($n = ” OR status = $n)`,
//
//	so the proof that closing the set at the boundary did not break the
//	two members that must keep filtering has to run against PostgreSQL:
//	an empty filter counts every seeded row, `success` counts only the
//	successful ones, and `error` counts only the failed ones. The
//	rejected values never reach this layer, which is the point: the
//	table here pins that the ones that do reach it still work.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-22
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestUsageRepository_StatusFilterMatchesRows seeds three successful and two
// failed rows and proves each closed-set member still filters the read: the
// unfiltered count is five, `success` reads two, `error` reads three, and a
// page of records filtered by status carries only rows of that status.
func TestUsageRepository_StatusFilterMatchesRows(t *testing.T) {
	repo := newUsageRepo(t)
	ctx := context.Background()

	seedUsage(t, repo, "req_ok_1", "openai", "gpt-4o", "0.01000000", 5, domain.UsageStatusSuccess)
	seedUsage(t, repo, "req_ok_2", "openai", "gpt-4o", "0.02000000", 6, domain.UsageStatusSuccess)
	seedUsage(t, repo, "req_fail_1", "openai", "gpt-4o", "0.00000000", 0, domain.UsageStatusError)
	seedUsage(t, repo, "req_fail_2", "openai", "gpt-4o", "0.00000000", 0, domain.UsageStatusError)
	seedUsage(t, repo, "req_fail_3", "openai", "gpt-4o", "0.00000000", 0, domain.UsageStatusError)

	statusRange := func(status domain.UsageStatus) domain.UsageFilter {
		return domain.NewUsageFilter(domain.UsageFilterInput{
			From:   ptrTime(time.Now().UTC().Add(-24 * time.Hour)),
			To:     ptrTime(time.Now().UTC().Add(time.Hour)),
			Status: status,
		}, time.Now())
	}

	cases := []struct {
		name         string
		status       domain.UsageStatus
		wantRows     int
		wantTotal    int64
		wantErrCount int64
	}{
		{name: "empty filter reads every row", status: "", wantRows: 5, wantTotal: 5, wantErrCount: 3},
		{name: "success reads only the successful rows", status: domain.UsageStatusSuccess, wantRows: 2, wantTotal: 2, wantErrCount: 0},
		{name: "error reads only the failed rows", status: domain.UsageStatusError, wantRows: 3, wantTotal: 3, wantErrCount: 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			records, total, err := repo.List(ctx, statusRange(tc.status), pageQuery(1, 100))
			if err != nil {
				t.Fatalf("List error = %v", err)
			}
			if int64(len(records)) != tc.wantTotal || total != tc.wantTotal {
				t.Fatalf("rows = %d, total = %d, want %d and %d", len(records), total, tc.wantRows, tc.wantTotal)
			}
			// An empty filter must return rows of both statuses; a member filter
			// must return only rows of that member.
			for _, record := range records {
				if tc.status != "" && record.UsageStatus() != tc.status {
					t.Fatalf("row %s status = %q, want %q", record.RequestID(), record.UsageStatus(), tc.status)
				}
			}

			totals, _, err := repo.Summary(ctx, statusRange(tc.status), "")
			if err != nil {
				t.Fatalf("Summary error = %v", err)
			}
			if totals.Requests != tc.wantTotal {
				t.Fatalf("totals.Requests = %d, want %d", totals.Requests, tc.wantTotal)
			}
			if totals.ErrorCount != tc.wantErrCount {
				t.Fatalf("totals.ErrorCount = %d, want %d", totals.ErrorCount, tc.wantErrCount)
			}
		})
	}
}
