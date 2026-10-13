// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_key_policy_test.go
// @for       The health model running on a configured parking budget: the window each class takes from the policy, the ceiling that stays fixed, and the streak a new ladder restarts from.
// @uses      testing, time.
// @reason    Draft 050 T8 turned the §7.5 windows from constants into the routing group's policy, so the rule that matters is what the key does with a budget it is handed: the credential and rate-limit classes escalate from the configured base, the outage class stays flat, and the ceilings hold no matter what the operator set.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-13
package domain

import (
	"testing"
	"time"
)

// TestUpstreamKey_RecordFailureUsesTheConfiguredBase is the operator-facing half of
// T8: a shortened credential base parks the first failure for that length, and the
// streak still doubles from it.
func TestUpstreamKey_RecordFailureUsesTheConfiguredBase(t *testing.T) {
	policy := KeyHealthPolicy{AuthWindow: 10 * time.Second}
	key := newTestKey(t, 1)
	key.RecordFailure("the credential was refused", KeyFailureAuth, policy, keyNow)

	if got := key.RateLimitedUntil().Sub(keyNow); got != 10*time.Second {
		t.Fatalf("first park = %v, want the configured 10s", got)
	}
	key.RecordFailure("the credential was refused", KeyFailureAuth, policy, keyNow)
	if got := key.RateLimitedUntil().Sub(keyNow); got != 20*time.Second {
		t.Fatalf("second park = %v, want the streak doubled to 20s", got)
	}
}

// TestUpstreamKey_RecordFailureCeilingsIgnoreTheConfiguredBase keeps the fixed
// ceilings: a base the operator stretched past its ceiling parks at the ceiling on
// the first failure rather than running the ladder past the bound.
func TestUpstreamKey_RecordFailureCeilingsIgnoreTheConfiguredBase(t *testing.T) {
	cases := []struct {
		name   string
		class  KeyFailureClass
		policy KeyHealthPolicy
		want   time.Duration
	}{
		{
			name:   "credential ladder stops at an hour",
			class:  KeyFailureAuth,
			policy: KeyHealthPolicy{AuthWindow: 20 * time.Minute},
			want:   time.Hour,
		},
		{
			name:   "rate-limit ladder stops at five minutes",
			class:  KeyFailureRateLimit,
			policy: KeyHealthPolicy{RateLimitWindow: 4 * time.Minute},
			want:   5 * time.Minute,
		},
	}
	for _, tc := range cases {
		key := newTestKey(t, 1)
		for range 8 {
			key.RecordFailure("refused", tc.class, tc.policy, keyNow)
		}
		if got := key.RateLimitedUntil().Sub(keyNow); got != tc.want {
			t.Fatalf("%s: window after 8 failures = %v, want the %v ceiling", tc.name, got, tc.want)
		}
	}
}

// TestUpstreamKey_RecordFailureTransientStaysFlatOnAConfiguredWindow is D3a's rule
// with a retuned number: an outage-shaped failure takes the configured window and
// never doubles, however long the streak grows.
func TestUpstreamKey_RecordFailureTransientStaysFlatOnAConfiguredWindow(t *testing.T) {
	policy := KeyHealthPolicy{TransientWindow: 3 * time.Minute}
	key := newTestKey(t, 1)
	for range 4 {
		key.RecordFailure("the upstream is overloaded", KeyFailureTransient, policy, keyNow)
	}
	if got := key.RateLimitedUntil().Sub(keyNow); got != 3*time.Minute {
		t.Fatalf("transient window = %v, want the flat configured 3m", got)
	}
}

// TestUpstreamKey_RecordFailureZeroPolicyKeepsDefaults is what lets the health
// model stay callable from a path that reads no settings: the zero value parks on
// the documented windows, not at zero.
func TestUpstreamKey_RecordFailureZeroPolicyKeepsDefaults(t *testing.T) {
	key := newTestKey(t, 1)
	key.RecordFailure("the upstream is overloaded", KeyFailureTransient, KeyHealthPolicy{}, keyNow)
	if got := key.RateLimitedUntil().Sub(keyNow); got != 30*time.Second {
		t.Fatalf("transient window = %v, want the documented 30s", got)
	}
	if got := (KeyHealthPolicy{}).WindowFor(KeyFailureAuth, 1); got != 2*time.Minute {
		t.Fatalf("credential window = %v, want the documented 2m", got)
	}
	if got := (KeyHealthPolicy{}).WindowFor(KeyFailureRateLimit, 1); got != 2*time.Second {
		t.Fatalf("rate-limit window = %v, want the documented 2s", got)
	}
}
