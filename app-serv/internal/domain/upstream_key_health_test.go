// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_key_health_test.go
// @for       Tests for key health transitions: class-aware parking, the
//
//	rate-limit ladder, and the reset on success.
//
// @uses      testing, time, internal/domain
// @reason    The router skips keys a failure has parked, so the per-class
//
//	windows, the exclusive end of each window, and the reset on success
//	must be pinned where a regression cannot silently route requests
//	into a dead credential (SPEC-API-001 §7.5).
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

// TestUpstreamKey_RecordFailureParksByClass pins the health model from
// SPEC-API-001 §7.5: the first failure parks the key for its class's window, a
// request-shaped failure parks nothing, and a success clears everything.
func TestUpstreamKey_RecordFailureParksByClass(t *testing.T) {
	cases := []struct {
		name          string
		class         KeyFailureClass
		wantStatus    UpstreamKeyStatus
		wantAvailable bool
		wantBackoff   time.Duration
	}{
		{name: "an auth failure parks the key for two minutes", class: KeyFailureAuth,
			wantStatus: UpstreamKeyError, wantAvailable: false, wantBackoff: 2 * time.Minute},
		{name: "a rate limit parks the key for the first ladder step", class: KeyFailureRateLimit,
			wantStatus: UpstreamKeyError, wantAvailable: false, wantBackoff: 2 * time.Second},
		{name: "a transient failure parks the key for thirty seconds", class: KeyFailureTransient,
			wantStatus: UpstreamKeyError, wantAvailable: false, wantBackoff: 30 * time.Second},
		{name: "a request-shaped failure parks nothing", class: KeyFailureRequest,
			wantStatus: UpstreamKeyActive, wantAvailable: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := newTestKey(t, 1)
			key.RecordFailure("upstream said no", tc.class, keyNow)
			if got := key.Status(); got != tc.wantStatus {
				t.Fatalf("status = %q, want %q", got, tc.wantStatus)
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
				t.Fatal("rate limited until must be set when the key is parked")
			}
			if got := until.Sub(keyNow); got != tc.wantBackoff {
				t.Fatalf("park window = %v, want %v", got, tc.wantBackoff)
			}
		})
	}
}

// TestUpstreamKey_RateLimitLadder pins the exponential ladder: each consecutive
// rate limit doubles the window from 2s, capped at 5 minutes.
func TestUpstreamKey_RateLimitLadder(t *testing.T) {
	cases := []struct {
		failures  int
		wantDelay time.Duration
	}{
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{8, 4*time.Minute + 16*time.Second},
	}
	for _, tc := range cases {
		key := newTestKey(t, 1)
		for i := 0; i < tc.failures; i++ {
			key.RecordFailure("rate limited", KeyFailureRateLimit, keyNow)
		}
		if got := key.RateLimitedUntil().Sub(keyNow); got != tc.wantDelay {
			t.Fatalf("after %d rate limits: window = %v, want %v", tc.failures, got, tc.wantDelay)
		}
	}
	// The ladder caps: a long run of rate limits never parks past five minutes,
	// and the shift cannot overflow into a negative window.
	for _, failures := range []int{20, 100} {
		key := newTestKey(t, 1)
		for i := 0; i < failures; i++ {
			key.RecordFailure("rate limited", KeyFailureRateLimit, keyNow)
		}
		if got := key.RateLimitedUntil().Sub(keyNow); got != 5*time.Minute {
			t.Fatalf("after %d rate limits: window = %v, want the 5m cap", failures, got)
		}
	}
}

func TestUpstreamKey_RecordSuccessResetsHealth(t *testing.T) {
	key := newTestKey(t, 1)
	key.RecordFailure("upstream said no", KeyFailureAuth, keyNow)
	if key.Available(keyNow) {
		t.Fatal("a parked key must not be available")
	}

	later := keyNow.Add(2*time.Minute + time.Minute)
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
		{name: "immediately after parking", advance: 0, available: false},
		{name: "one second before expiry", advance: 2*time.Minute - time.Second, available: false},
		{name: "exactly at expiry", advance: 2 * time.Minute, available: true},
		{name: "well after expiry", advance: 2*time.Minute + time.Hour, available: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := newTestKey(t, 1)
			key.RecordFailure("upstream said no", KeyFailureAuth, keyNow)
			if got := key.Available(keyNow.Add(tc.advance)); got != tc.available {
				t.Fatalf("Available(+%v) = %v, want %v", tc.advance, got, tc.available)
			}
		})
	}
}
