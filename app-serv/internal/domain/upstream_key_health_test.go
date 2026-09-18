// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_key_health_test.go
// @for       Tests for key health transitions, circuit breaking, and backoff windows.
// @uses      testing, time, internal/domain
// @reason    The router skips keys the breaker has tripped, so the failure
//
//	threshold, the reset on success, and the exclusive end of the backoff
//	window must be pinned where a regression cannot silently route requests
//	into a rate-limited credential.
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

// TestUpstreamKey_RecordFailureTripsTheCircuit pins the health model from
// SPEC-API-001 §7.5: repeated failures mark the key unusable for a backoff
// window, a success clears that state, and the counter never trips early.
func TestUpstreamKey_RecordFailureTripsTheCircuit(t *testing.T) {
	cases := []struct {
		name          string
		failures      int
		wantStatus    UpstreamKeyStatus
		wantAvailable bool
		wantBackoff   time.Duration
	}{
		{name: "a single failure stays healthy", failures: 1,
			wantStatus: UpstreamKeyActive, wantAvailable: true},
		{name: "two failures stay healthy", failures: 2,
			wantStatus: UpstreamKeyActive, wantAvailable: true},
		{name: "the third failure trips the circuit", failures: 3,
			wantStatus: UpstreamKeyError, wantAvailable: false, wantBackoff: keyCircuitBackoff},
		{name: "a fourth failure keeps it tripped", failures: 4,
			wantStatus: UpstreamKeyError, wantAvailable: false, wantBackoff: keyCircuitBackoff},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := newTestKey(t, 1)
			for range tc.failures {
				key.RecordFailure("upstream said no", keyNow)
			}
			if got := key.Status(); got != tc.wantStatus {
				t.Fatalf("status = %q, want %q", got, tc.wantStatus)
			}
			if got := key.ConsecutiveErrors(); got != tc.failures {
				t.Fatalf("consecutive errors = %d, want %d", got, tc.failures)
			}
			if got := key.Available(keyNow); got != tc.wantAvailable {
				t.Fatalf("Available() = %v, want %v", got, tc.wantAvailable)
			}
			until := key.RateLimitedUntil()
			if tc.wantBackoff == 0 {
				if until != nil {
					t.Fatalf("rate limited until = %v, want unset", until)
				}
				return
			}
			if until == nil {
				t.Fatal("rate limited until must be set when the circuit trips")
			}
			if got := until.Sub(keyNow); got != tc.wantBackoff {
				t.Fatalf("backoff = %v, want %v", got, tc.wantBackoff)
			}
		})
	}
}

func TestUpstreamKey_RecordSuccessResetsHealth(t *testing.T) {
	key := newTestKey(t, 1)
	for range 3 {
		key.RecordFailure("upstream said no", keyNow)
	}
	if key.Available(keyNow) {
		t.Fatal("a tripped key must not be available")
	}

	later := keyNow.Add(keyCircuitBackoff + time.Minute)
	key.RecordSuccess(later)
	if got := key.ConsecutiveErrors(); got != 0 {
		t.Fatalf("consecutive errors = %d, want 0 after a success", got)
	}
	if key.RateLimitedUntil() != nil {
		t.Fatal("a success must clear the rate-limited window")
	}
	if key.LastError() != "" {
		t.Fatalf("last error = %q, want it cleared", key.LastError())
	}
	if !key.Available(later) {
		t.Fatal("a key that succeeded must be available")
	}
}

// TestUpstreamKey_AvailableDuringAndAfterBackoff covers the boundary the router
// reads on every request: the window is exclusive at its end.
func TestUpstreamKey_AvailableDuringAndAfterBackoff(t *testing.T) {
	cases := []struct {
		name      string
		advance   time.Duration
		available bool
	}{
		{name: "immediately after tripping", advance: 0, available: false},
		{name: "one second before expiry", advance: keyCircuitBackoff - time.Second, available: false},
		{name: "exactly at expiry", advance: keyCircuitBackoff, available: true},
		{name: "well after expiry", advance: keyCircuitBackoff + time.Hour, available: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := newTestKey(t, 1)
			for range 3 {
				key.RecordFailure("upstream said no", keyNow)
			}
			if got := key.Available(keyNow.Add(tc.advance)); got != tc.available {
				t.Fatalf("Available(+%v) = %v, want %v", tc.advance, got, tc.available)
			}
		})
	}
}
