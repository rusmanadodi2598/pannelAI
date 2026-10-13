// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_key_health_test.go
// @for       Tests for key health transitions: class-aware parking, the rate-limit ladder, and the reset on success.
// @uses      testing, time, internal/domain
// @reason    The router skips keys a failure has parked, so the per-class windows, the exclusive end of each window, and the reset on success must be pinned where a regression cannot silently route requests into a dead credential (SPEC-API-001 §7.5).
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
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
		{name: "a target-shaped failure parks nothing", class: KeyFailureTarget,
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

// TestKeyFailureClass_ImplicatesCredential pins the single answer the selector
// and the aggregate share: a class that does not implicate the credential parks
// nothing and earns no key-row write.
func TestKeyFailureClass_ImplicatesCredential(t *testing.T) {
	cases := []struct {
		name  string
		class KeyFailureClass
		want  bool
	}{
		{name: "auth", class: KeyFailureAuth, want: true},
		{name: "rate limit", class: KeyFailureRateLimit, want: true},
		{name: "transient", class: KeyFailureTransient, want: true},
		{name: "request", class: KeyFailureRequest, want: false},
		{name: "target", class: KeyFailureTarget, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.class.ImplicatesCredential(); got != tc.want {
				t.Fatalf("ImplicatesCredential() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestUpstreamKey_NonCredentialFailureLeavesTheCounterAlone pins why the strike
// ladder cannot be fed by a target-shaped failure: a credential that never
// earned the failure must not carry it into the next window's backoff.
func TestUpstreamKey_NonCredentialFailureLeavesTheCounterAlone(t *testing.T) {
	for _, class := range []KeyFailureClass{KeyFailureRequest, KeyFailureTarget} {
		key := newTestKey(t, 1)
		key.RecordFailure("the upstream could not be reached", class, keyNow)
		key.RecordFailure("the upstream could not be reached", class, keyNow)
		if got := key.ConsecutiveErrors(); got != 0 {
			t.Fatalf("consecutive errors = %d for class %d, want 0", got, class)
		}
		if !key.Available(keyNow) {
			t.Fatalf("a class %d failure must leave the key selectable", class)
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
