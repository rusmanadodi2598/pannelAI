// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/usage_active_codec_test.go
// @for       The in-flight marker's wire codec and the staleness cutoff the
//
//	live read is bounded by.
//
// @uses      encoding/json, strings, testing, time.
// @reason    The marker is the one value the live drawing's "routing now" claim
//
//	rests on (R-36), and it crosses a shared Redis instance, so it is
//	both a validated aggregate input and an untrusted decode. Each rule
//	is pinned here: a marker missing its provider would light the wrong
//	node, and a decoder that accepted a foreign member would put a node
//	on screen for a value nothing wrote.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-22
package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestActiveRequestCodec pins the round trip and the refusals. The store is a
// shared Redis instance, so a member read back is untrusted input (OWASP A08)
// and the decoder re-applies every invariant the constructor enforces.
func TestActiveRequestCodec(t *testing.T) {
	t.Run("a valid marker round trips", func(t *testing.T) {
		want := activeRequestFixture()
		payload, err := EncodeActiveRequest(want)
		if err != nil {
			t.Fatalf("EncodeActiveRequest() = %v, want nil", err)
		}
		got, err := DecodeActiveRequest(payload)
		if err != nil {
			t.Fatalf("DecodeActiveRequest() = %v, want nil", err)
		}
		if got != want {
			t.Fatalf("round trip = %+v, want %+v", got, want)
		}
		if !strings.Contains(string(payload), `"provider_id":"openai"`) {
			t.Fatalf("payload = %s, want snake_case keys like every other wire", payload)
		}
	})

	refusals := []struct {
		name    string
		payload string
	}{
		{name: "an empty payload is refused", payload: ""},
		{name: "a non-object payload is refused", payload: `"openai"`},
		{name: "a truncated payload is refused", payload: `{"marker_id":"m"`},
		{name: "a missing provider is refused", payload: `{"marker_id":"m","request_id":"r","started_at":"2026-09-22T10:00:00Z"}`},
		{name: "an unparseable instant is refused", payload: `{"marker_id":"m","request_id":"r","provider_id":"openai","started_at":"yesterday"}`},
		{name: "an unknown field is refused", payload: `{"marker_id":"m","request_id":"r","provider_id":"openai","started_at":"2026-09-22T10:00:00Z","extra":1}`},
		{name: "a trailing value is refused", payload: `{"marker_id":"m","request_id":"r","provider_id":"openai","started_at":"2026-09-22T10:00:00Z"}{}`},
	}
	for _, tc := range refusals {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecodeActiveRequest([]byte(tc.payload))
			if err == nil {
				t.Fatalf("DecodeActiveRequest(%s) = %+v, want a refusal", tc.payload, got)
			}
			if got != (ActiveRequest{}) {
				t.Fatalf("a refused decode = %+v, want the zero value", got)
			}
		})
	}
}

// TestActiveRequestCutoff pins the staleness boundary the live read is bounded
// by: a marker exactly at the cutoff is already stale, which is the same
// exclusive boundary the panel's own 60 second guard applies.
func TestActiveRequestCutoff(t *testing.T) {
	cases := []struct {
		name      string
		now       time.Time
		startedAt time.Time
		wantLive  bool
	}{
		{
			name: "a request that just started is live", now: time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC),
			startedAt: time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC), wantLive: true,
		},
		{
			name: "a request one second short of the window is live", now: time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC),
			startedAt: time.Date(2026, 9, 22, 9, 59, 1, 0, time.UTC), wantLive: true,
		},
		{
			name: "a request exactly at the window is stale", now: time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC),
			startedAt: time.Date(2026, 9, 22, 9, 59, 0, 0, time.UTC), wantLive: false,
		},
		{
			name: "a request well past the window is stale", now: time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC),
			startedAt: time.Date(2026, 9, 22, 9, 0, 0, 0, time.UTC), wantLive: false,
		},
		{
			name: "a start instant ahead of now is live", now: time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC),
			startedAt: time.Date(2026, 9, 22, 10, 0, 5, 0, time.UTC), wantLive: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cutoff := ActiveRequestCutoff(tc.now)
			// The rule is "not older than the window", so a marker at exactly the
			// cutoff instant is already stale and one ahead of now is still live.
			live := tc.startedAt.After(cutoff)
			if live != tc.wantLive {
				t.Fatalf("marker at %v against cutoff %v = live %v, want %v", tc.startedAt, cutoff, live, tc.wantLive)
			}
			if got := tc.now.Sub(cutoff); got != ActiveRequestStaleAfter {
				t.Fatalf("the cutoff is %v before now, want %v", got, ActiveRequestStaleAfter)
			}
		})
	}
}

// TestActiveRequestWireShapeIsStable pins the JSON member names, because the
// store's members outlive one process: a rename would make every marker already
// in Redis undecodable, and the panel would draw an idle gateway.
func TestActiveRequestWireShapeIsStable(t *testing.T) {
	payload, err := EncodeActiveRequest(activeRequestFixture())
	if err != nil {
		t.Fatalf("EncodeActiveRequest() = %v, want nil", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decoding the payload: %v", err)
	}
	for _, field := range []string{"marker_id", "request_id", "provider_id", "endpoint_id", "model", "started_at"} {
		if _, ok := decoded[field]; !ok {
			t.Errorf("payload %s is missing %q", payload, field)
		}
	}
	if len(decoded) != 6 {
		t.Fatalf("payload carries %d fields, want exactly 6: %s", len(decoded), payload)
	}
}
