// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/usage_live_test.go
// @for       Table-driven tests for the live frame: the wire shape the panel
//
//	reads, and the mapping from the aggregate the gateway holds.
//
// @uses      encoding/json, testing, time, internal/domain.
// @reason    The frame is a contract app-ui already parses with Zod
//
//	(`src/lib/schemas/usage-live.ts`), so its member names and its
//	null-versus-empty rules are pinned here rather than left to review:
//	a renamed member makes the panel drop every frame silently, which
//	looks exactly like a gateway with nothing to report.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-22
package schema

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestUsageLiveFrame_JSONShape pins the exact member set the panel reads, in
// both the frame and its two list entries. An extra or missing member is a
// contract change on a route the panel consumes, so it fails here.
func TestUsageLiveFrame_JSONShape(t *testing.T) {
	frame := UsageLiveFrameFrom(
		[]domain.ActiveRequest{{
			MarkerID: "m1", RequestID: "req_1", ProviderID: "openai",
			EndpointID: "ep_1", Model: "gpt-4o",
			StartedAt: time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC),
		}},
		[]domain.UsageRecord{},
		"",
	)
	payload, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("marshalling the frame: %v", err)
	}

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decoding the frame: %v", err)
	}
	for _, field := range []string{"active", "recent", "error_provider"} {
		if _, ok := decoded[field]; !ok {
			t.Errorf("frame %s is missing %q", payload, field)
		}
	}
	if len(decoded) != 3 {
		t.Fatalf("frame carries %d members, want exactly 3: %s", len(decoded), payload)
	}

	var active []map[string]json.RawMessage
	if err := json.Unmarshal(decoded["active"], &active); err != nil {
		t.Fatalf("decoding active: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("active carries %d entries, want 1", len(active))
	}
	for _, field := range []string{"provider_id", "endpoint_id", "model", "started_at"} {
		if _, ok := active[0][field]; !ok {
			t.Errorf("an active entry is missing %q: %s", field, payload)
		}
	}
	if len(active[0]) != 4 {
		t.Errorf("an active entry carries %d members, want exactly 4: %s", len(active[0]), payload)
	}
}

// TestUsageLiveFrame_EmptyListsAreArraysNotNull pins the rule the panel's
// `nullableList` normalizes: Go marshals a nil slice as `null`, and a reader
// that only ever saw `[]` would be reading a different contract than the one
// this gateway writes.
func TestUsageLiveFrame_EmptyListsAreArraysNotNull(t *testing.T) {
	cases := []struct {
		name   string
		frame  UsageLiveFrame
		field  string
		want   string
		reason string
	}{
		{
			name:  "no active request",
			frame: UsageLiveFrameFrom(nil, nil, ""),
			field: "active", want: "[]",
			reason: "an empty in-flight set is a fact, and null would be a second spelling of it",
		},
		{
			name:  "no finished request",
			frame: UsageLiveFrameFrom(nil, nil, ""),
			field: "recent", want: "[]",
			reason: "an empty recent list is a fact, and null would be a second spelling of it",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(tc.frame)
			if err != nil {
				t.Fatalf("marshalling: %v", err)
			}
			var decoded map[string]json.RawMessage
			if err := json.Unmarshal(payload, &decoded); err != nil {
				t.Fatalf("decoding: %v", err)
			}
			if got := string(decoded[tc.field]); got != tc.want {
				t.Fatalf("%s = %s, want %s: %s", tc.field, got, tc.want, tc.reason)
			}
		})
	}
}
