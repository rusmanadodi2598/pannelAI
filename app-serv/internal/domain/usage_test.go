// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/usage_test.go
// @for       Table-driven tests for the UsageRecord aggregate's validation and
//
//	projection into the aggregate totals.
//
// @uses      testing, time.
// @reason    AGENTS.md §2.1 and §2.4 require the recorder's invariants to be
//
//	pinned: a row with a negative counter or an unparseable cost would
//	skew every summary, group, and bucket built on it, so the rejection
//	path needs a test as much as the happy path does.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-18
package domain

import (
	"testing"
	"time"
)

// TestNewUsageRecord covers construction validation and the derived id and
// timestamp.
func TestNewUsageRecord(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	explicit := now.Add(-time.Hour)

	cases := []struct {
		name     string
		in       UsageRecordInput
		id       string
		wantErr  bool
		wantTS   time.Time
		wantCost string
	}{
		{
			name:     "successful request",
			in:       UsageRecordInput{RequestID: "req_1", ProviderID: "openai", Model: "gpt", Status: UsageStatusSuccess, CostUSD: "0.0042", TokensIn: 10},
			wantTS:   now,
			wantCost: "0.00420000",
		},
		{
			name:     "failed request keeps its error code",
			in:       UsageRecordInput{RequestID: "req_2", ProviderID: "openai", Model: "gpt", Status: UsageStatusError, ErrorCode: "UPSTREAM_TIMEOUT"},
			wantTS:   now,
			wantCost: "0.00000000",
		},
		{
			name:     "an explicit timestamp is kept and normalized to UTC",
			in:       UsageRecordInput{RequestID: "req_3", ProviderID: "openai", Model: "gpt", Status: UsageStatusSuccess, TS: explicit},
			wantTS:   explicit.UTC(),
			wantCost: "0.00000000",
		},
		{
			name:     "a supplied id is used",
			in:       UsageRecordInput{RequestID: "req_4", ProviderID: "openai", Model: "gpt", Status: UsageStatusSuccess},
			id:       "usg_fixed",
			wantTS:   now,
			wantCost: "0.00000000",
		},
		{
			name:     "zero counters and an empty cost are valid",
			in:       UsageRecordInput{RequestID: "req_5", ProviderID: "openai", Model: "gpt", Status: UsageStatusSuccess, CostUSD: ""},
			wantTS:   now,
			wantCost: "0.00000000",
		},
		{
			name:    "missing request id",
			in:      UsageRecordInput{ProviderID: "openai", Model: "gpt", Status: UsageStatusSuccess},
			wantErr: true,
		},
		{
			name:    "missing provider",
			in:      UsageRecordInput{RequestID: "req_6", Model: "gpt", Status: UsageStatusSuccess},
			wantErr: true,
		},
		{
			name:    "missing model",
			in:      UsageRecordInput{RequestID: "req_7", ProviderID: "openai", Status: UsageStatusSuccess},
			wantErr: true,
		},
		{
			name:    "unknown status",
			in:      UsageRecordInput{RequestID: "req_8", ProviderID: "openai", Model: "gpt", Status: "partial"},
			wantErr: true,
		},
		{
			name:    "negative tokens",
			in:      UsageRecordInput{RequestID: "req_9", ProviderID: "openai", Model: "gpt", Status: UsageStatusSuccess, TokensIn: -1},
			wantErr: true,
		},
		{
			name:    "negative cache write",
			in:      UsageRecordInput{RequestID: "req_10", ProviderID: "openai", Model: "gpt", Status: UsageStatusSuccess, TokensCacheWrite: -5},
			wantErr: true,
		},
		{
			name:    "negative latency",
			in:      UsageRecordInput{RequestID: "req_11", ProviderID: "openai", Model: "gpt", Status: UsageStatusSuccess, LatencyMS: -10},
			wantErr: true,
		},
		{
			name:    "malformed cost",
			in:      UsageRecordInput{RequestID: "req_12", ProviderID: "openai", Model: "gpt", Status: UsageStatusSuccess, CostUSD: "free"},
			wantErr: true,
		},
		{
			name:    "negative cost",
			in:      UsageRecordInput{RequestID: "req_13", ProviderID: "openai", Model: "gpt", Status: UsageStatusSuccess, CostUSD: "-0.01"},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record, err := NewUsageRecord(tc.in, tc.id, now)
			if tc.wantErr {
				if err == nil {
					t.Fatal("NewUsageRecord = nil error, want a rejection")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewUsageRecord error = %v", err)
			}
			if !record.TS().Equal(tc.wantTS) {
				t.Fatalf("TS = %s, want %s", record.TS(), tc.wantTS)
			}
			if record.CostUSD() != tc.wantCost {
				t.Fatalf("CostUSD = %q, want %q", record.CostUSD(), tc.wantCost)
			}
			if tc.id != "" {
				if record.ID() != tc.id {
					t.Fatalf("ID = %q, want %q", record.ID(), tc.id)
				}
			} else if record.ID() == "" {
				t.Fatal("ID is empty, want a generated ULID")
			}
		})
	}
}

// TestUsageRecord_TotalTokensAndTotals covers the projections a rollup and a
// quota counter read.
func TestUsageRecord_TotalTokensAndTotals(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name         string
		in           UsageRecordInput
		wantTotal    int64
		wantRequests int64
		wantErrors   int64
		wantCost     string
	}{
		{
			name:         "cache traffic counts toward the total",
			in:           UsageRecordInput{RequestID: "r1", ProviderID: "p", Model: "m", Status: UsageStatusSuccess, TokensIn: 10, TokensOut: 20, TokensCacheRead: 5, TokensCacheWrite: 1, CostUSD: "0.5", LatencyMS: 120},
			wantTotal:    36,
			wantRequests: 1,
			wantErrors:   0,
			wantCost:     "0.50000000",
		},
		{
			name:         "a failed request counts as an error",
			in:           UsageRecordInput{RequestID: "r2", ProviderID: "p", Model: "m", Status: UsageStatusError, LatencyMS: 40},
			wantTotal:    0,
			wantRequests: 1,
			wantErrors:   1,
			wantCost:     "0.00000000",
		},
		{
			name:         "an extreme token count stays exact",
			in:           UsageRecordInput{RequestID: "r3", ProviderID: "p", Model: "m", Status: UsageStatusSuccess, TokensIn: 1 << 40, TokensOut: 1 << 30},
			wantTotal:    (1 << 40) + (1 << 30),
			wantRequests: 1,
			wantErrors:   0,
			wantCost:     "0.00000000",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record, err := NewUsageRecord(tc.in, "", now)
			if err != nil {
				t.Fatalf("NewUsageRecord error = %v", err)
			}
			if got := record.TotalTokens(); got != tc.wantTotal {
				t.Fatalf("TotalTokens = %d, want %d", got, tc.wantTotal)
			}
			totals := record.Totals()
			if totals.Requests != tc.wantRequests {
				t.Fatalf("Requests = %d, want %d", totals.Requests, tc.wantRequests)
			}
			if totals.ErrorCount != tc.wantErrors {
				t.Fatalf("ErrorCount = %d, want %d", totals.ErrorCount, tc.wantErrors)
			}
			if totals.CostUSD != tc.wantCost {
				t.Fatalf("CostUSD = %q, want %q", totals.CostUSD, tc.wantCost)
			}
			// The percentiles of a single record are its own latency, which is
			// what makes a one-record window report a real figure.
			if totals.LatencyP50MS != record.LatencyMS() || totals.LatencyP95MS != record.LatencyMS() {
				t.Fatalf("percentiles = (%d, %d), want both %d", totals.LatencyP50MS, totals.LatencyP95MS, record.LatencyMS())
			}
		})
	}
}
