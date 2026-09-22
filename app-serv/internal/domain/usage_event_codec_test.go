// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/usage_event_codec_test.go
// @for       Table-driven tests for the usage event's invariants and the wire
//
//	codec a publisher and a subscriber meet on.
//
// @uses      testing, time.
// @reason    The event crosses a broker, so every payload the subscriber reads
//
//	is untrusted input: a truncated, mislabelled, or hostile message must
//	be refused rather than decoded into a zero-valued event (OWASP A08).
//	The invariants are the aggregate's own, so they are pinned here once
//	for both directions of the codec, and every refusal case carries a
//	benign control so an over-blocking validator fails the table too.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-22
package domain

import (
	"strings"
	"testing"
	"time"
)

// usageEventFixture is the benign event every table below varies one field of.
func usageEventFixture() UsageEvent {
	return UsageEvent{
		RequestID:   "req_1",
		ProviderID:  "openai",
		EndpointID:  "ep_1",
		Model:       "gpt-4o",
		TotalTokens: 7,
		CostUSD:     "0.01000000",
		Status:      UsageStatusSuccess,
		OccurredAt:  time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC),
	}
}

// TestUsageEvent_CodecRoundTrip asserts the two directions of the codec agree:
// an event this build encodes is an event this build decodes, field for field.
func TestUsageEvent_CodecRoundTrip(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*UsageEvent)
	}{
		{name: "a complete event survives the round trip"},
		{
			name: "an endpoint-less error event survives the round trip",
			mutate: func(e *UsageEvent) {
				e.EndpointID = ""
				e.Status = UsageStatusError
				e.TotalTokens = 0
				e.CostUSD = "0.00000000"
			},
		},
		{
			name: "a unicode model name survives the round trip",
			mutate: func(e *UsageEvent) {
				e.Model = "モデル/φ-4o"
			},
		},
		{
			name: "a large token count survives the round trip",
			mutate: func(e *UsageEvent) {
				e.TotalTokens = 9_007_199_254_740_993
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := usageEventFixture()
			if tc.mutate != nil {
				tc.mutate(&want)
			}
			payload, err := EncodeUsageEvent(want)
			if err != nil {
				t.Fatalf("EncodeUsageEvent() = %v, want nil", err)
			}
			got, err := DecodeUsageEvent(payload)
			if err != nil {
				t.Fatalf("DecodeUsageEvent() = %v, want nil", err)
			}
			if got != want {
				t.Fatalf("round trip = %+v, want %+v", got, want)
			}
		})
	}
}

// TestEncodeUsageEvent_RefusesAnInvalidEvent asserts the publisher half cannot
// put a payload on the channel that the subscriber half would have to refuse.
func TestEncodeUsageEvent_RefusesAnInvalidEvent(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*UsageEvent)
		wantErr bool
	}{
		{name: "a valid event is encoded"},
		{name: "a missing request id is refused", mutate: func(e *UsageEvent) { e.RequestID = "" }, wantErr: true},
		{name: "a negative token count is refused", mutate: func(e *UsageEvent) { e.TotalTokens = -5 }, wantErr: true},
		{name: "an unknown status is refused", mutate: func(e *UsageEvent) { e.Status = "SUCCESS" }, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			event := usageEventFixture()
			if tc.mutate != nil {
				tc.mutate(&event)
			}
			payload, err := EncodeUsageEvent(event)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("EncodeUsageEvent() = %q, want a refusal", payload)
				}
				return
			}
			if err != nil {
				t.Fatalf("EncodeUsageEvent() = %v, want nil", err)
			}
		})
	}
}

// TestDecodeUsageEvent_RefusesAMalformedPayload is the OWASP A08 table: every
// payload the channel can carry but the aggregate would never have produced is
// refused, and the refusals are benign controls plus hostile shapes so the
// decoder cannot pass by accepting everything or by rejecting everything.
func TestDecodeUsageEvent_RefusesAMalformedPayload(t *testing.T) {
	valid := usageEventFixture()
	encoded, err := EncodeUsageEvent(valid)
	if err != nil {
		t.Fatalf("building the control payload: %v", err)
	}
	with := func(old, replacement string) string {
		return strings.Replace(string(encoded), old, replacement, 1)
	}

	cases := []struct {
		name    string
		payload string
		wantErr bool
	}{
		{name: "the control payload decodes", payload: string(encoded)},
		{name: "empty input is refused", payload: "", wantErr: true},
		{name: "a non-JSON payload is refused", payload: "usage.recorded req_1", wantErr: true},
		{name: "a JSON array is refused", payload: `[]`, wantErr: true},
		{name: "a JSON scalar is refused", payload: `"usage.recorded"`, wantErr: true},
		{name: "a JSON null is refused", payload: `null`, wantErr: true},
		{
			name:    "a misdelivered event name is refused",
			payload: with(`"usage.recorded"`, `"usage.deleted"`),
			wantErr: true,
		},
		{
			name:    "a payload missing the request id is refused",
			payload: with(`"request_id":"req_1"`, `"request_id":""`),
			wantErr: true,
		},
		{
			name:    "a negative token count is refused",
			payload: with(`"total_tokens":7`, `"total_tokens":-7`),
			wantErr: true,
		},
		{
			name:    "an unknown status is refused",
			payload: with(`"status":"success"`, `"status":"SUCCESS"`),
			wantErr: true,
		},
		{
			name:    "an unparseable cost is refused",
			payload: with(`"cost_usd":"0.01000000"`, `"cost_usd":"NaN"`),
			wantErr: true,
		},
		{
			name:    "an unparseable timestamp is refused",
			payload: with(`"2026-09-22T10:00:00Z"`, `"yesterday"`),
			wantErr: true,
		},
		{
			name:    "an unknown field is refused",
			payload: with(`"event":"usage.recorded"`, `"event":"usage.recorded","role":"admin"`),
			wantErr: true,
		},
		{name: "a truncated payload is refused", payload: string(encoded[:len(encoded)/2]), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			event, err := DecodeUsageEvent([]byte(tc.payload))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("DecodeUsageEvent() = %+v, want a refusal", event)
				}
				// A refusal must never hand back a partly-filled event: a
				// consumer reading one would act on a value the aggregate
				// would have rejected.
				if event != (UsageEvent{}) {
					t.Fatalf("DecodeUsageEvent() returned %+v alongside %v, want the zero event", event, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("DecodeUsageEvent() = %v, want nil", err)
			}
			if event != valid {
				t.Fatalf("DecodeUsageEvent() = %+v, want %+v", event, valid)
			}
		})
	}
}
