// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_key_ladder_test.go
// @for       Tests for the two doubling parking ladders: the credential (401/402/403) window and the rate-limit (429) window, with their ceilings and the reset that returns them to the first step.
// @uses      testing, time, internal/domain
// @reason    Draft 050 T5 turned the flat 2-minute credential park into a ladder, and a ladder pins three things a constant does not: the first step, the doubling, and the ceiling that stops a long streak from overflowing the duration. They live apart from the parking-class tests because each drives RecordFailure in a loop.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-13
package domain

import (
	"testing"
	"time"
)

// TestUpstreamKey_AuthLadderEscalates pins the credential ladder: each
// consecutive 401/402/403 doubles the window from 2m up to an hour, so a dead
// credential is retried on a widening interval instead of every two minutes.
func TestUpstreamKey_AuthLadderEscalates(t *testing.T) {
	cases := []struct {
		failures  int
		wantDelay time.Duration
	}{
		{1, 2 * time.Minute},
		{2, 4 * time.Minute},
		{3, 8 * time.Minute},
		{5, 32 * time.Minute},
		{6, time.Hour},
		{20, time.Hour},
	}
	for _, tc := range cases {
		key := newTestKey(t, 1)
		for range tc.failures {
			key.RecordFailure("the credential was refused", KeyFailureAuth, DefaultKeyHealthPolicy(), keyNow)
		}
		if got := key.RateLimitedUntil().Sub(keyNow); got != tc.wantDelay {
			t.Fatalf("after %d credential failures: window = %v, want %v", tc.failures, got, tc.wantDelay)
		}
	}
}

// TestUpstreamKey_AuthLadderResetsOnSuccess pins that the ladder is a streak,
// not a ratchet: a served call returns the next credential failure to 2m.
func TestUpstreamKey_AuthLadderResetsOnSuccess(t *testing.T) {
	key := newTestKey(t, 1)
	for range 5 {
		key.RecordFailure("the credential was refused", KeyFailureAuth, DefaultKeyHealthPolicy(), keyNow)
	}
	key.RecordSuccess(keyNow)
	key.RecordFailure("the credential was refused", KeyFailureAuth, DefaultKeyHealthPolicy(), keyNow)
	if got := key.RateLimitedUntil().Sub(keyNow); got != 2*time.Minute {
		t.Fatalf("window after a success = %v, want the first step of 2m", got)
	}
}

// TestUpstreamKey_TransientStaysFlat keeps 5xx out of the credential ladder: a
// briefly unhealthy target is not a dead credential, so its park stays flat 30s.
func TestUpstreamKey_TransientStaysFlat(t *testing.T) {
	key := newTestKey(t, 1)
	for range 4 {
		key.RecordFailure("the upstream is overloaded", KeyFailureTransient, DefaultKeyHealthPolicy(), keyNow)
	}
	if got := key.RateLimitedUntil().Sub(keyNow); got != 30*time.Second {
		t.Fatalf("transient window = %v, want the flat 30s", got)
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
			key.RecordFailure("rate limited", KeyFailureRateLimit, DefaultKeyHealthPolicy(), keyNow)
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
			key.RecordFailure("rate limited", KeyFailureRateLimit, DefaultKeyHealthPolicy(), keyNow)
		}
		if got := key.RateLimitedUntil().Sub(keyNow); got != 5*time.Minute {
			t.Fatalf("after %d rate limits: window = %v, want the 5m cap", failures, got)
		}
	}
}
