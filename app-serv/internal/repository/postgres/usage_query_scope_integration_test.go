//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/usage_query_scope_integration_test.go
// @for       The free-text q scope against a real server (draft 010 F8).
// @uses      internal/domain, internal/repository, context, testing, time.
// @reason    Draft 010 F8: q compiled to `model ILIKE` only, while the panel's
//
//	placeholder promised a request id and an error code, so an operator
//	searching the id they could see on screen read an empty table. The
//	expanded predicate is `request_id OR id OR error_code OR model`, and
//	only a real server can prove the OR, the case-insensitivity, and the
//	NULL handling: a stub would accept whatever the Go code did, and an
//	unhandled NULL in `error_code ILIKE ...` would silently drop rows
//	that matched on a different column.
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

// queryScopeRange is the window every case below reads.
func queryScopeRange() domain.UsageFilter {
	return domain.NewUsageFilter(domain.UsageFilterInput{
		From: ptrTime(time.Now().UTC().Add(-24 * time.Hour)),
		To:   ptrTime(time.Now().UTC().Add(time.Hour)),
	}, time.Now())
}

// TestUsageRepository_QueryScopeMatchesIdentityAndError seeds rows whose
// request id, record id, error code, and model differ, then proves each of the
// four fields the contract now names is searchable, that the match is a
// case-insensitive substring, that an empty filter is still "everything", and
// that a value matching nothing answers zero rows rather than an error.
func TestUsageRepository_QueryScopeMatchesIdentityAndError(t *testing.T) {
	repo := newUsageRepo(t)
	ctx := context.Background()

	seedQueryRow(t, repo, "usg_alpha", "req_alpha", "openai", "gpt-4o", "UPSTREAM_TIMEOUT", domain.UsageStatusError)
	seedQueryRow(t, repo, "usg_beta", "req_beta", "anthropic", "claude-3", "", domain.UsageStatusSuccess)
	seedQueryRow(t, repo, "usg_gamma", "req_gamma", "openai", "o3-mini", "MODEL_NOT_FOUND", domain.UsageStatusError)

	cases := []struct {
		name      string
		query     string
		wantRows  int
		wantFirst string
	}{
		{name: "an empty filter reads every row", query: "", wantRows: 3},
		{name: "the request id is searchable", query: "req_beta", wantRows: 1, wantFirst: "req_beta"},
		{name: "the request id matches as a substring", query: "req_", wantRows: 3},
		{name: "the record id is searchable", query: "usg_alpha", wantRows: 1, wantFirst: "req_alpha"},
		{name: "the record id prefix matches every row", query: "usg_", wantRows: 3},
		{name: "the error code is searchable", query: "MODEL_NOT_FOUND", wantRows: 1, wantFirst: "req_gamma"},
		{name: "the error code matches case-insensitively", query: "model_not_found", wantRows: 1, wantFirst: "req_gamma"},
		{name: "the model is searchable", query: "claude", wantRows: 1, wantFirst: "req_beta"},
		{name: "the model matches case-insensitively", query: "GPT", wantRows: 1, wantFirst: "req_alpha"},
		{name: "a shared substring matches across fields", query: "mini", wantRows: 1, wantFirst: "req_gamma"},
		{name: "a value matching nothing answers zero rows", query: "no-such-request", wantRows: 0},
		{name: "a row with a null error code still matches on another field", query: "req_beta", wantRows: 1, wantFirst: "req_beta"},
		{name: "an SQL fragment stays a literal and matches nothing", query: "' OR 1=1 --", wantRows: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			filter := queryScopeRange()
			filter.Query = tc.query
			records, total, err := repo.List(ctx, filter, pageQuery(1, 100))
			if err != nil {
				t.Fatalf("List error = %v", err)
			}
			if len(records) != tc.wantRows || total != int64(tc.wantRows) {
				t.Fatalf("rows = %d, total = %d, want %d and %d", len(records), total, tc.wantRows, tc.wantRows)
			}
			if tc.wantFirst != "" && records[0].RequestID() != tc.wantFirst {
				t.Fatalf("first row request id = %q, want %q", records[0].RequestID(), tc.wantFirst)
			}
		})
	}
}

// TestUsageRepository_QueryScopeNarrowsTheAggregates proves the same predicate
// reaches the summary and the timeseries, not only the paged list: a filter
// that narrowed one read and not the others would let a chart disagree with
// the table under it.
func TestUsageRepository_QueryScopeNarrowsTheAggregates(t *testing.T) {
	repo := newUsageRepo(t)
	ctx := context.Background()

	seedQueryRow(t, repo, "usg_alpha", "req_alpha", "openai", "gpt-4o", "UPSTREAM_TIMEOUT", domain.UsageStatusError)
	seedQueryRow(t, repo, "usg_beta", "req_beta", "anthropic", "claude-3", "", domain.UsageStatusSuccess)

	byErrorCode := queryScopeRange()
	byErrorCode.Query = "UPSTREAM_TIMEOUT"

	totals, _, err := repo.Summary(ctx, byErrorCode, "")
	if err != nil {
		t.Fatalf("Summary error = %v", err)
	}
	if totals.Requests != 1 || totals.ErrorCount != 1 {
		t.Fatalf("summary totals = %+v, want the single failed row", totals)
	}

	buckets, err := repo.Timeseries(ctx, byErrorCode, domain.UsageHourly)
	if err != nil {
		t.Fatalf("Timeseries error = %v", err)
	}
	var bucketed int64
	for _, bucket := range buckets {
		bucketed += bucket.Totals.Requests
	}
	if bucketed != 1 {
		t.Fatalf("timeseries requests = %d, want 1", bucketed)
	}
}

// seedQueryRow inserts one row with the identity, error code, and status the
// scope cases distinguish, at a fixed latency so the arithmetic cases can
// depend on it.
func seedQueryRow(t *testing.T, repo *UsageRepository, id, requestID, providerID, model, errorCode string, status domain.UsageStatus) {
	t.Helper()
	record, err := domain.NewUsageRecord(domain.UsageRecordInput{
		RequestID:  requestID,
		TS:         time.Now().UTC().Add(-time.Minute),
		ProviderID: providerID,
		Model:      model,
		Status:     status,
		ErrorCode:  errorCode,
		TokensIn:   1,
		CostUSD:    "0.01",
		LatencyMS:  100,
	}, id, time.Now())
	if err != nil {
		t.Fatalf("building record %s: %v", requestID, err)
	}
	if err := repo.Record(context.Background(), record); err != nil {
		t.Fatalf("recording %s: %v", requestID, err)
	}
}
