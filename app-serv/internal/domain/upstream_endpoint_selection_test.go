// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_endpoint_selection_test.go
// @for       Table-driven tests for how an endpoint picks the key it routes with.
// @uses      testing, time.
// @reason    The router reads these orderings directly: priority decides which
//
//	credential is spent first, health and backoff decide when the next one is
//	skipped, so they are pinned apart from the endpoint CRUD tests.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import (
	"testing"
	"time"
)

func TestUpstreamEndpoint_ValidKeysAreOrderedByPriority(t *testing.T) {
	third := newTestKey(t, 3)
	first := newTestKey(t, 1)
	second := newTestKey(t, 2)
	endpoint := newTestEndpoint(t, "api_key", third, first, second)

	got := endpoint.ValidKeys(keyNow)
	if len(got) != 3 {
		t.Fatalf("ValidKeys() returned %d keys, want 3", len(got))
	}
	for i, want := range []int{1, 2, 3} {
		if got[i].Priority() != want {
			t.Fatalf("ValidKeys()[%d].Priority() = %d, want %d", i, got[i].Priority(), want)
		}
	}

	// A tripped key leaves the list. The failure runs through the aggregate root,
	// because Key returns a copy: mutating a detached value cannot affect it.
	if _, err := endpoint.RecordKeyFailure(first.ID(), "upstream said no", keyNow); err != nil {
		t.Fatalf("RecordKeyFailure() error = %v", err)
	}
	for range 2 {
		if _, err := endpoint.RecordKeyFailure(first.ID(), "upstream said no", keyNow); err != nil {
			t.Fatalf("RecordKeyFailure() error = %v", err)
		}
	}
	if got := endpoint.ValidKeys(keyNow); len(got) != 2 {
		t.Fatalf("ValidKeys() = %d entries with a tripped key, want 2", len(got))
	}
	if got := endpoint.ValidKeys(keyNow.Add(10 * time.Minute)); len(got) != 3 {
		t.Fatalf("ValidKeys() = %d entries after the backoff expired, want 3", len(got))
	}
}

func TestUpstreamEndpoint_NextKeyFollowsPriorityThenHealth(t *testing.T) {
	cases := []struct {
		name        string
		disable     []int
		trip        []int
		wantFirst   int
		wantPresent bool
	}{
		{name: "the highest priority key wins", wantFirst: 1, wantPresent: true},
		{name: "a disabled top key passes to the next", disable: []int{1}, wantFirst: 2, wantPresent: true},
		{name: "a tripped top key passes to the next", trip: []int{1}, wantFirst: 2, wantPresent: true},
		{name: "two unavailable keys leave the third", disable: []int{1}, trip: []int{2}, wantFirst: 3, wantPresent: true},
		{name: "every key unavailable yields nothing", disable: []int{1, 2}, trip: []int{3}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := newTestEndpoint(t, "api_key", newTestKey(t, 1), newTestKey(t, 2), newTestKey(t, 3))
			for _, priority := range tc.disable {
				key := endpoint.KeyByPriority(priority)
				if _, err := endpoint.SetKeyStatus(key.ID(), "disabled", keyNow); err != nil {
					t.Fatalf("SetKeyStatus(disabled) error = %v", err)
				}
			}
			for _, priority := range tc.trip {
				key := endpoint.KeyByPriority(priority)
				for range 3 {
					if _, err := endpoint.RecordKeyFailure(key.ID(), "nope", keyNow); err != nil {
						t.Fatalf("RecordKeyFailure() error = %v", err)
					}
				}
			}

			next, ok := endpoint.NextKey(keyNow)
			if ok != tc.wantPresent {
				t.Fatalf("NextKey() ok = %v, want %v", ok, tc.wantPresent)
			}
			if ok && next.Priority() != tc.wantFirst {
				t.Fatalf("NextKey().Priority() = %d, want %d", next.Priority(), tc.wantFirst)
			}
		})
	}
}

// TestUpstreamEndpoint_AvailableToRoute covers what the router asks before it
// spends a request: an endpoint the operator disabled or the circuit tripped is
// skipped.
func TestUpstreamEndpoint_AvailableToRoute(t *testing.T) {
	cases := []struct {
		name      string
		status    string
		advance   time.Duration
		available bool
	}{
		{name: "active and healthy", status: "active", available: true},
		{name: "disabled", status: "disabled"},
		{name: "in backoff", status: "active", advance: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := newTestEndpoint(t, "api_key", newTestKey(t, 1))
			if tc.status == "disabled" {
				if err := endpoint.Update("primary", 1, "disabled", keyNow); err != nil {
					t.Fatalf("Update(disabled) error = %v", err)
				}
			}
			if tc.name == "in backoff" {
				until := keyNow.Add(5 * time.Minute)
				endpoint.MarkRateLimited(until)
			}
			if got := endpoint.Available(keyNow.Add(tc.advance)); got != tc.available {
				t.Fatalf("Available() = %v, want %v", got, tc.available)
			}
		})
	}
}
