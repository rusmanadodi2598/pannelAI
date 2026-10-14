// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/key_health_retry_after_test.go
// @for       Tests for the vendor's named wait inside the parking budget: the floor, the ceiling, and the classes that ignore it.
// @uses      testing, time.
// @reason    A 429 that says "come back in two minutes" answered with the ladder's two-second first step is a credential put straight back into a limiter. Honoring the hint matters, and so does not trusting it further than the ladder's own ceiling: one vendor could park a key for a day with a header.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-14
package domain

import (
	"testing"
	"time"
)

func TestKeyHealthPolicy_WindowForHonorsRetryAfterHint(t *testing.T) {
	cases := []struct {
		name    string
		class   KeyFailureClass
		hint    time.Duration
		strikes int
		want    time.Duration
	}{
		{name: "no hint keeps the ladder step", class: KeyFailureRateLimit, hint: 0, strikes: 3, want: 8 * time.Second},
		{name: "a two minute wait is parked for two minutes", class: KeyFailureRateLimit,
			hint: 2 * time.Minute, strikes: 1, want: 2 * time.Minute},
		{name: "a sub-second wait is raised to the ladder's first step", class: KeyFailureRateLimit,
			hint: 500 * time.Millisecond, strikes: 1, want: keyRateLimitBase},
		{name: "a day is held to the ceiling", class: KeyFailureRateLimit,
			hint: 24 * time.Hour, strikes: 1, want: keyRateLimitMax},
		{name: "the ceiling exactly is not stretched", class: KeyFailureRateLimit,
			hint: keyRateLimitMax, strikes: 1, want: keyRateLimitMax},
		{name: "an auth hint does not move the credential ladder", class: KeyFailureAuth,
			hint: 45 * time.Second, strikes: 1, want: 2 * time.Minute},
		{name: "a transient hint does not move the flat window", class: KeyFailureTransient,
			hint: 90 * time.Second, strikes: 4, want: 30 * time.Second},
		{name: "a billing hint does not move the billing ladder", class: KeyFailureBilling,
			hint: 3 * time.Minute, strikes: 2, want: 30 * time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			policy := DefaultKeyHealthPolicy().WithRetryAfter(tc.hint)
			if got := policy.WindowFor(tc.class, tc.strikes); got != tc.want {
				t.Fatalf("WindowFor(%v, %d) with hint %v = %v, want %v",
					tc.class, tc.strikes, tc.hint, got, tc.want)
			}
		})
	}
}

// TestKeyHealthPolicy_WithRetryAfterCopies pins that applying a hint returns a new
// budget: one provider's parked wait must not leak into the policy the settings read
// hands to the next failure.
func TestKeyHealthPolicy_WithRetryAfterCopies(t *testing.T) {
	base := DefaultKeyHealthPolicy()
	hinted := base.WithRetryAfter(90 * time.Second)

	if hinted.RetryAfterHint != 90*time.Second {
		t.Fatalf("hinted budget = %v, want the wait applied", hinted.RetryAfterHint)
	}
	if base.RetryAfterHint != 0 {
		t.Fatalf("the base budget was mutated: %v", base.RetryAfterHint)
	}
}
