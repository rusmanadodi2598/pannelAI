// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/usage_event_test.go
// @for       Table-driven tests for the usage domain event a recorded request
//
//	publishes.
//
// @uses      testing, time.
// @reason    AGENTS.md §2.3 makes the event the seam a quota or log consumer
//
//	reacts to, so the payload it carries is a contract: an event missing
//	the routing identity or the counter would leave a consumer unable
//	to do its job, and nothing else would fail.
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

// TestUsageRecord_NewUsageEvent covers the event the quota and log consumers
// react to, which must carry the routing identity and the counters.
func TestUsageRecord_NewUsageEvent(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name           string
		in             UsageRecordInput
		wantTotal      int64
		wantEndpointID string
	}{
		{
			name:           "event carries the endpoint and the token total",
			in:             UsageRecordInput{RequestID: "req_1", ProviderID: "openai", EndpointID: "ep_1", Model: "gpt", Status: UsageStatusSuccess, TokensIn: 3, TokensOut: 4, CostUSD: "0.01"},
			wantTotal:      7,
			wantEndpointID: "ep_1",
		},
		{
			name:           "an endpoint-less event is still an event",
			in:             UsageRecordInput{RequestID: "req_2", ProviderID: "openai", Model: "gpt", Status: UsageStatusError},
			wantTotal:      0,
			wantEndpointID: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record, err := NewUsageRecord(tc.in, "", now)
			if err != nil {
				t.Fatalf("NewUsageRecord error = %v", err)
			}
			event := record.NewUsageEvent()
			if event.RequestID != tc.in.RequestID {
				t.Fatalf("RequestID = %q, want %q", event.RequestID, tc.in.RequestID)
			}
			if event.EndpointID != tc.wantEndpointID {
				t.Fatalf("EndpointID = %q, want %q", event.EndpointID, tc.wantEndpointID)
			}
			if event.TotalTokens != tc.wantTotal {
				t.Fatalf("TotalTokens = %d, want %d", event.TotalTokens, tc.wantTotal)
			}
			if !event.OccurredAt.Equal(record.TS()) {
				t.Fatalf("OccurredAt = %s, want %s", event.OccurredAt, record.TS())
			}
		})
	}
}

// TestRehydrateUsageRecord_KeepsStoredCost asserts the load path returns the
// stored cost verbatim, so a value the database rendered is never re-rounded on
// the way out.
func TestRehydrateUsageRecord_KeepsStoredCost(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	record := RehydrateUsageRecord("usg_1", "req_1", now, "ep_1", "openai", "gky_1",
		"gpt", "cmb_1", 1, 2, 3, 4, "0.12345678", 55, "success", "")
	if record.CostUSD() != "0.12345678" {
		t.Fatalf("CostUSD = %q, want the stored value", record.CostUSD())
	}
	if record.TotalTokens() != 10 {
		t.Fatalf("TotalTokens = %d, want 10", record.TotalTokens())
	}
	if record.GatewayKeyID() != "gky_1" || record.Combo() != "cmb_1" {
		t.Fatalf("rehydrated identity = (%q, %q), want (gky_1, cmb_1)", record.GatewayKeyID(), record.Combo())
	}
}
