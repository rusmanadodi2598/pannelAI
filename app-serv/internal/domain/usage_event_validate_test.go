// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/usage_event_validate_test.go
// @for       Table-driven tests for the invariants a usage event shares with the
//
//	aggregate it was derived from.
//
// @uses      testing, time.
// @reason    The event is read back from a broker, so its invariants are the
//
//	gate that keeps a hostile or truncated payload out of every consumer
//	(OWASP A08). They are the aggregate's own rules, so they are pinned
//	once here and the codec's own table relies on them rather than
//	restating them.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-22
package domain

import (
	"testing"
	"time"
)

// TestUsageEvent_Validate pins the invariants the event shares with the
// aggregate it was derived from. The benign controls matter as much as the
// refusals: a validator that refused everything would pass a refusal-only
// table, and an event no consumer may act on would be lost silently.
func TestUsageEvent_Validate(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*UsageEvent)
		wantErr bool
	}{
		{name: "a complete success event is valid"},
		{
			name: "an endpoint-less error event is valid",
			mutate: func(e *UsageEvent) {
				e.EndpointID = ""
				e.TotalTokens = 0
				e.CostUSD = "0.00000000"
				e.Status = UsageStatusError
			},
		},
		{
			name: "a large token count is valid",
			mutate: func(e *UsageEvent) {
				e.TotalTokens = 9_007_199_254_740_993
			},
		},
		{
			name:   "a negative token count is refused",
			mutate: func(e *UsageEvent) { e.TotalTokens = -1 },
			// A negative counter would skew every aggregate the event feeds,
			// which is the same reason NewUsageRecord refuses one.
			wantErr: true,
		},
		{name: "a missing request id is refused", mutate: func(e *UsageEvent) { e.RequestID = "" }, wantErr: true},
		{name: "a missing provider is refused", mutate: func(e *UsageEvent) { e.ProviderID = "" }, wantErr: true},
		{name: "a missing model is refused", mutate: func(e *UsageEvent) { e.Model = "" }, wantErr: true},
		{name: "an unknown status is refused", mutate: func(e *UsageEvent) { e.Status = "banana" }, wantErr: true},
		{name: "an empty status is refused", mutate: func(e *UsageEvent) { e.Status = "" }, wantErr: true},
		{name: "an unparseable cost is refused", mutate: func(e *UsageEvent) { e.CostUSD = "abc" }, wantErr: true},
		{name: "a negative cost is refused", mutate: func(e *UsageEvent) { e.CostUSD = "-1.00" }, wantErr: true},
		{name: "a zero timestamp is refused", mutate: func(e *UsageEvent) { e.OccurredAt = time.Time{} }, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			event := usageEventFixture()
			if tc.mutate != nil {
				tc.mutate(&event)
			}
			err := event.Validate()
			if tc.wantErr {
				if err == nil {
					t.Fatal("Validate() = nil, want a refusal")
				}
				if AsAppError(err).Code != "VALIDATION_ERROR" {
					t.Fatalf("Validate() code = %q, want VALIDATION_ERROR", AsAppError(err).Code)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}
