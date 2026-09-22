//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/usage_latency_semantics_integration_test.go
// @for       The aggregate latency_ms semantic against a real server (draft 010 F3).
// @uses      internal/domain, context, testing, time.
// @reason    Draft 010 F3: the aggregate latency is a SUM of per-request
//
//	durations and nothing on the wire said so. The arithmetic test that
//	already exists seeds three rows all at 100 ms, so a mean and a sum
//	both read 300 and it cannot tell the two apart. These rows are
//	deliberately unequal so the assertion fails if the projection ever
//	becomes an average, which is the change owner decision D1 = document
//	rules out: the contract now promises the sum, so the sum is pinned.
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

// TestUsageRepository_AggregateLatencyIsASumNotAMean seeds three rows whose
// latencies differ, so the sum (600) and the mean (200) are different numbers,
// and proves every aggregate read reports the sum while the percentiles keep
// describing one request. The per-record read reports that request's own
// duration, which is the only place the field means one measurement.
func TestUsageRepository_AggregateLatencyIsASumNotAMean(t *testing.T) {
	repo := newUsageRepo(t)
	ctx := context.Background()

	seedLatencyRow(t, repo, "usg_fast", "req_fast", "gpt-4o-mini", 100)
	seedLatencyRow(t, repo, "usg_mid", "req_mid", "gpt-4o", 200)
	seedLatencyRow(t, repo, "usg_slow", "req_slow", "o3", 300)

	filter := usageRange()

	t.Run("the summary reports the sum", func(t *testing.T) {
		totals, _, err := repo.Summary(ctx, filter, "")
		if err != nil {
			t.Fatalf("Summary error = %v", err)
		}
		if totals.Requests != 3 {
			t.Fatalf("Requests = %d, want 3", totals.Requests)
		}
		if totals.LatencyMS != 600 {
			t.Fatalf("LatencyMS = %d, want 600 (the sum); a mean would read 200", totals.LatencyMS)
		}
		if totals.LatencyP50MS != 200 || totals.LatencyP95MS != 300 {
			t.Fatalf("percentiles = (%d, %d), want (200, 300): one request each, not a total",
				totals.LatencyP50MS, totals.LatencyP95MS)
		}
	})

	t.Run("each group row reports the sum of its own rows", func(t *testing.T) {
		_, groups, err := repo.Summary(ctx, filter, domain.UsageGroupModel)
		if err != nil {
			t.Fatalf("Summary error = %v", err)
		}
		want := map[string]int64{"gpt-4o-mini": 100, "gpt-4o": 200, "o3": 300}
		if len(groups) != len(want) {
			t.Fatalf("groups = %d, want %d", len(groups), len(want))
		}
		for _, group := range groups {
			if group.Totals.LatencyMS != want[group.Key] {
				t.Fatalf("group %q latency = %d, want %d: one row per model, so the group is that row's sum",
					group.Key, group.Totals.LatencyMS, want[group.Key])
			}
		}
	})

	t.Run("the timeseries bucket reports the sum of the rows it covers", func(t *testing.T) {
		buckets, err := repo.Timeseries(ctx, filter, domain.UsageHourly)
		if err != nil {
			t.Fatalf("Timeseries error = %v", err)
		}
		var sum int64
		for _, bucket := range buckets {
			sum += bucket.Totals.LatencyMS
		}
		if sum != 600 {
			t.Fatalf("bucketed latency = %d, want 600", sum)
		}
	})

	t.Run("the record read reports that request's own duration", func(t *testing.T) {
		record, err := repo.GetByRequestID(ctx, "req_slow")
		if err != nil {
			t.Fatalf("GetByRequestID error = %v", err)
		}
		if record.LatencyMS() != 300 {
			t.Fatalf("record latency = %d, want 300: this request's own measurement", record.LatencyMS())
		}
	})
}

// seedLatencyRow inserts one successful row at the given latency.
func seedLatencyRow(t *testing.T, repo *UsageRepository, id, requestID, model string, latencyMS int64) {
	t.Helper()
	record, err := domain.NewUsageRecord(domain.UsageRecordInput{
		RequestID:  requestID,
		TS:         time.Now().UTC().Add(-time.Minute),
		ProviderID: "openai",
		Model:      model,
		Status:     domain.UsageStatusSuccess,
		TokensIn:   1,
		CostUSD:    "0.01",
		LatencyMS:  latencyMS,
	}, id, time.Now())
	if err != nil {
		t.Fatalf("building record %s: %v", requestID, err)
	}
	if err := repo.Record(context.Background(), record); err != nil {
		t.Fatalf("recording %s: %v", requestID, err)
	}
}
