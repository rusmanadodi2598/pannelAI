// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/failure_class_test.go
// @for       The failure taxonomy and what the key records from it: the status a class falls back to, the park a stated reset writes onto the key, and how a policy override merges.
// @uses      testing, time.
// @reason    The class is the word the router uses for a refusal and the key is where its consequence is written, so each rule that names a class or lands a park on the credential is pinned where it is decided: which statement wins, what happens when none is made, and which fields an operator's override inherits.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-15
package domain

import (
	"testing"
	"time"
)

var restNow = time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)

// TestFailureClassForStatus pins the conservative mapping: with no signature, a
// status answers exactly what it answered before this classification existed.
func TestFailureClassForStatus(t *testing.T) {
	cases := []struct {
		status int
		want   FailureClass
	}{
		{429, FailureRateLimitTemporary},
		{401, FailureAuthInvalid},
		{403, FailureAuthInvalid},
		{402, FailureBillingExhausted},
		{500, FailureServerTransient},
		{503, FailureServerTransient},
		{404, FailureTargetUnreachable},
		{400, FailureRequestInvalid},
		{422, FailureRequestInvalid},
		{0, FailureTargetUnreachable},
		{200, FailureUnknown},
	}
	for _, tc := range cases {
		if got := FailureClassForStatus(tc.status); got != tc.want {
			t.Fatalf("FailureClassForStatus(%d) = %s, want %s", tc.status, got, tc.want)
		}
	}
}

// TestUpstreamKey_RecordFailureParksToTheStatedReset is the whole path in one
// assertion: a key that failed with a spent allowance is unavailable until the
// vendor's own reset and available the moment it passes.
func TestUpstreamKey_RecordFailureParksToTheStatedReset(t *testing.T) {
	key := newTestKey(t, 1)
	reset := time.Date(2026, 10, 19, 10, 48, 0, 0, time.UTC)
	policy := DefaultKeyHealthPolicy().WithRest(RestDirective{
		Class:       FailureQuotaExhausted,
		ResetAt:     reset,
		Source:      ResetSourceBody,
		SignatureID: "tokenharbor.free-allowance",
	})
	key.RecordFailure("You've used this period's free allowance", KeyFailureQuota, policy, restNow)

	if got := key.RateLimitedUntil(); got == nil || !got.Equal(reset) {
		t.Fatalf("parked until = %v, want the stated reset %v", got, reset)
	}
	if key.LastErrorClass() != "QUOTA_EXHAUSTED" {
		t.Fatalf("last_error_class = %q, want the recognised class", key.LastErrorClass())
	}
	if key.ResetSource() != string(ResetSourceBody) || key.SignatureID() != "tokenharbor.free-allowance" {
		t.Fatalf("provenance = %q/%q, want body/tokenharbor.free-allowance",
			key.ResetSource(), key.SignatureID())
	}
	// A day before the reset the key is still out of rotation: this is the walk
	// leg the five-minute ceiling was spending every cycle.
	if key.Available(reset.Add(-24 * time.Hour)) {
		t.Fatal("the key is available a day before its reset, want it withheld")
	}
	if !key.Available(reset) {
		t.Fatal("the key is still parked at its reset, want it back in rotation")
	}
}

// TestUpstreamKey_QuotaSharesTheAccountRun keeps the escalation live: a spent
// allowance advances the same run a 402 does, so the second strike parks longer
// than the first.
func TestUpstreamKey_QuotaSharesTheAccountRun(t *testing.T) {
	key := newTestKey(t, 1)
	policy := DefaultKeyHealthPolicy().WithRest(RestDirective{Class: FailureQuotaExhausted})
	for range 2 {
		key.RecordFailure("allowance spent", KeyFailureQuota, policy, restNow)
	}
	if got := key.RateLimitedUntil().Sub(restNow); got != time.Hour {
		t.Fatalf("second quota park = %v, want the ladder's 1h", got)
	}
	key.RecordSuccess(restNow)
	if key.RateLimitedUntil() != nil || key.ResetSource() != "" || key.SignatureID() != "" {
		t.Fatalf("a served call left %v/%q/%q behind, want a clean key",
			key.RateLimitedUntil(), key.ResetSource(), key.SignatureID())
	}
}

// TestUpstreamKey_AKeyToldToRotateIsNotParked is the action knob on the key: no
// park is written, so the credential stays in rotation for the next request.
func TestUpstreamKey_AKeyToldToRotateIsNotParked(t *testing.T) {
	key := newTestKey(t, 1)
	policy := DefaultKeyHealthPolicy().WithRest(RestDirective{Class: FailureRateLimitTemporary})
	policy.FailureClasses = FailureClassPolicies{
		FailureRateLimitTemporary: {Action: FailureActionRotate},
	}
	key.RecordFailure("too many requests", KeyFailureRateLimit, policy, restNow)
	if key.RateLimitedUntil() != nil {
		t.Fatalf("parked until = %v, want no park for a class told to rotate", key.RateLimitedUntil())
	}
}

// TestFailureClassPolicy_MergedKeepsTheUnnamedFields is the override rule: a
// provider that retunes one field inherits the others.
func TestFailureClassPolicy_MergedKeepsTheUnnamedFields(t *testing.T) {
	base := FailureClassPolicy{Action: FailureActionPark, CeilingSeconds: 300, CooldownSeconds: 60}
	merged := base.Merged(FailureClassPolicy{Action: FailureActionStop})
	if merged.Action != FailureActionStop || merged.CeilingSeconds != 300 || merged.CooldownSeconds != 60 {
		t.Fatalf("merged = %+v, want only the action replaced", merged)
	}
	if got := merged.Ceiling(999 * time.Hour); got != 300*time.Second {
		t.Fatalf("ceiling = %v, want the configured 300s", got)
	}
	if got := (FailureClassPolicy{}).Ceiling(5 * time.Minute); got != 5*time.Minute {
		t.Fatalf("ceiling = %v, want the inherited 5m", got)
	}
	if got := (FailureClassPolicy{}).Cooldown(0); got != 0 {
		t.Fatalf("cooldown = %v, want zero for 'keep the ladder'", got)
	}
	if !FailureAction("").Valid() == false {
		t.Fatal("an empty action must not read as valid")
	}
	if !FailureClass("RATE_LIMIT_TEMPORARY").Recognised() || FailureClass("SLOW_DOWN").Recognised() {
		t.Fatal("Recognised() drew the wrong line around the vocabulary")
	}
}
