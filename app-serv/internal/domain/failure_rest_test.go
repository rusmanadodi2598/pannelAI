// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/failure_rest_test.go
// @for       The rest one classified failure earns: which statement sets it, the ceiling that bounds it, and the ladder it escalates on when nothing is stated.
// @uses      testing, time.
// @reason    The park is the router's only lever over a failing credential and the rest is the number it turns, so the resolution that produces it is pinned apart from the taxonomy that names the class: which statement wins, and where a vendor's own number stops being believed.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-15
package domain

import (
	"testing"
	"time"
)

// TestRestUntilPrefersTheStatedReset is C2: a date the vendor printed beats
// everything the gateway could compute, and it is not clipped to a ceiling that
// was invented for a throttle.
func TestRestUntilPrefersTheStatedReset(t *testing.T) {
	reset := time.Date(2026, 10, 19, 10, 48, 0, 0, time.UTC)
	policy := DefaultKeyHealthPolicy().WithRest(RestDirective{
		Class:       FailureQuotaExhausted,
		ResetAt:     reset,
		Source:      ResetSourceBody,
		SignatureID: "tokenharbor.free-allowance",
		RetryAfter:  5 * time.Minute,
	})
	got := policy.RestUntil(KeyFailureQuota, 1, restNow)
	if !got.Equal(reset) {
		t.Fatalf("RestUntil(quota, 1) = %v, want the stated reset %v", got, reset)
	}
}

// TestRestUntilFallsBackToTheHeaderThenTheLadder is the COALESCE in the formula:
// a header the vendor sent, then the class's own window.
func TestRestUntilFallsBackToTheHeaderThenTheLadder(t *testing.T) {
	base := DefaultKeyHealthPolicy()

	headered := base.WithRest(RestDirective{Class: FailureRateLimitTemporary, RetryAfter: 2 * time.Minute})
	if got := headered.RestUntil(KeyFailureRateLimit, 1, restNow); got != restNow.Add(2*time.Minute) {
		t.Fatalf("RestUntil(rate limit, hint 2m) = %v, want the honored 2m", got.Sub(restNow))
	}
	// The ladder's first step is the floor, so a header naming one second is a
	// queue guess and not a promise.
	brief := base.WithRest(RestDirective{Class: FailureRateLimitTemporary, RetryAfter: time.Second})
	if got := brief.RestUntil(KeyFailureRateLimit, 1, restNow); got != restNow.Add(2*time.Second) {
		t.Fatalf("RestUntil(rate limit, hint 1s) = %v, want the ladder's 2s floor", got.Sub(restNow))
	}
	// A header past the ceiling is held to it: the rate-limit ceiling is the five
	// minutes it always was.
	long := base.WithRest(RestDirective{Class: FailureRateLimitTemporary, RetryAfter: time.Hour})
	if got := long.RestUntil(KeyFailureRateLimit, 1, restNow); got != restNow.Add(5*time.Minute) {
		t.Fatalf("RestUntil(rate limit, hint 1h) = %v, want the 5m ceiling", got.Sub(restNow))
	}
	// With no statement at all, the ladder answers, unchanged.
	plain := base.WithRest(RestDirective{Class: FailureRateLimitTemporary})
	if got := plain.RestUntil(KeyFailureRateLimit, 3, restNow); got != restNow.Add(8*time.Second) {
		t.Fatalf("RestUntil(rate limit, 3) = %v, want the ladder's 8s", got.Sub(restNow))
	}
	// A transient outage keeps its flat 30s.
	transient := base.WithRest(RestDirective{Class: FailureServerTransient})
	if got := transient.RestUntil(KeyFailureTransient, 4, restNow); got != restNow.Add(30*time.Second) {
		t.Fatalf("RestUntil(transient, 4) = %v, want the flat 30s", got.Sub(restNow))
	}
}

// TestRestUntilHoldsEachClassToItsCeiling pins the bounds: a computed wait never
// outruns the class it belongs to.
func TestRestUntilHoldsEachClassToItsCeiling(t *testing.T) {
	cases := []struct {
		name    string
		class   FailureClass
		parking KeyFailureClass
		hint    time.Duration
		want    time.Duration
	}{
		{name: "rate limit stops at 5m", class: FailureRateLimitTemporary, parking: KeyFailureRateLimit,
			hint: 3 * time.Hour, want: 5 * time.Minute},
		{name: "an outage stops at 30s", class: FailureServerTransient, parking: KeyFailureTransient,
			hint: 10 * time.Minute, want: 30 * time.Second},
		{name: "a refused credential stops at 1h", class: FailureAuthInvalid, parking: KeyFailureAuth,
			hint: 5 * time.Hour, want: time.Hour},
		{name: "a spent account stops at 6h", class: FailureBillingExhausted, parking: KeyFailureBilling,
			hint: 48 * time.Hour, want: 6 * time.Hour},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			policy := DefaultKeyHealthPolicy().WithRest(RestDirective{Class: tc.class, RetryAfter: tc.hint})
			if got := policy.RestUntil(tc.parking, 1, restNow); got != restNow.Add(tc.want) {
				t.Fatalf("RestUntil(%s) = %v, want %v", tc.class, got.Sub(restNow), tc.want)
			}
		})
	}
}

// TestRestUntilQuotaEscalates is the ladder for a spent allowance with no date to
// read: four steps to a day, not eight doublings, because every step is a walk
// leg the gateway already knows cannot be served.
func TestRestUntilQuotaEscalates(t *testing.T) {
	want := []time.Duration{15 * time.Minute, time.Hour, 6 * time.Hour, 24 * time.Hour, 24 * time.Hour}
	for strike, expected := range want {
		policy := DefaultKeyHealthPolicy().WithRest(RestDirective{Class: FailureQuotaExhausted})
		got := policy.RestUntil(KeyFailureQuota, strike+1, restNow)
		if got != restNow.Add(expected) {
			t.Fatalf("RestUntil(quota, %d) = %v, want %v", strike+1, got.Sub(restNow), expected)
		}
	}
}

// TestRestUntilAStatedResetIsStillBounded is the guard on the other side: a date
// that cannot be real does not bury the key for a decade.
func TestRestUntilAStatedResetIsStillBounded(t *testing.T) {
	policy := DefaultKeyHealthPolicy().WithRest(RestDirective{
		Class:   FailureQuotaExhausted,
		ResetAt: restNow.Add(365 * 24 * time.Hour),
	})
	if got := policy.RestUntil(KeyFailureQuota, 1, restNow); got != restNow.Add(restCeilingQuota) {
		t.Fatalf("RestUntil(quota, absurd reset) = %v, want the %v ceiling", got.Sub(restNow), restCeilingQuota)
	}
}

// TestRestUntilAClassToldToRotateParksNothing is the action knob: a provider whose
// throttles the operator would rather walk past than wait out parks nothing.
func TestRestUntilAClassToldToRotateParksNothing(t *testing.T) {
	policy := DefaultKeyHealthPolicy()
	policy.FailureClasses = FailureClassPolicies{
		FailureRateLimitTemporary: {Action: FailureActionRotate},
	}
	got := policy.WithRest(RestDirective{Class: FailureRateLimitTemporary}).RestUntil(KeyFailureRateLimit, 1, restNow)
	if !got.IsZero() {
		t.Fatalf("RestUntil(rotate) = %v, want no park at all", got)
	}
}

// TestRestUntilUnclassifiedKeepsTodaysBehaviour is what keeps the rest of the
// gateway honest: a caller with no classification to hand parks exactly as it did
// before the classifier existed.
func TestRestUntilUnclassifiedKeepsTodaysBehaviour(t *testing.T) {
	cases := []struct {
		class KeyFailureClass
		want  time.Duration
	}{
		{KeyFailureAuth, 2 * time.Minute},
		{KeyFailureRateLimit, 2 * time.Second},
		{KeyFailureTransient, 30 * time.Second},
		{KeyFailureBilling, 15 * time.Minute},
	}
	for _, tc := range cases {
		if got := DefaultKeyHealthPolicy().RestUntil(tc.class, 1, restNow); got != restNow.Add(tc.want) {
			t.Fatalf("RestUntil(%s, 1) = %v, want %v", tc.class, got.Sub(restNow), tc.want)
		}
	}
}
