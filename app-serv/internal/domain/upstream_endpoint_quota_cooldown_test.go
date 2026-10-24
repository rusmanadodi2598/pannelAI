// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_endpoint_quota_cooldown_test.go
// @for       The endpoint's quota-poller cooldown: the window a vendor 429 opens, the rule that only extends it, and the fact that it never withholds an account from serving.
// @uses      testing, time.
// @reason    Draft 050 T1 found one column asked two incompatible questions: the poller's "is this account throttled" and the router's "may this account serve". This pins the separation the fix depends on, because a cooldown that leaked into Available() would take an endpoint's healthy keys out of rotation on the strength of one 429.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-13
package domain

import (
	"testing"
	"time"
)

// cooldownEndpoint builds an API-key endpoint with one usable key, so a test can ask
// both questions about the same account: is it throttled for the poller, and may it
// still serve.
func cooldownEndpoint(t *testing.T) *UpstreamEndpoint {
	t.Helper()
	endpoint := newTestEndpoint(t, "api_key", newTestKey(t, 1))
	return &endpoint
}

// TestUpstreamEndpoint_MarkQuotaThrottledOpensTheWindow pins the deadline and the
// read that depends on it: throttled inside the window, due again once it passes.
func TestUpstreamEndpoint_MarkQuotaThrottledOpensTheWindow(t *testing.T) {
	endpoint := cooldownEndpoint(t)
	endpoint.MarkQuotaThrottled(keyNow)

	if until := endpoint.QuotaCooldownUntil(); until == nil || !until.Equal(keyNow.Add(QuotaCooldownWindow)) {
		t.Fatalf("cooldown = %v, want %v after the 429", until, keyNow.Add(QuotaCooldownWindow))
	}
	if !endpoint.QuotaThrottled(keyNow.Add(QuotaCooldownWindow - time.Minute)) {
		t.Fatal("the endpoint must read as throttled inside its window")
	}
	if endpoint.QuotaThrottled(keyNow.Add(QuotaCooldownWindow)) {
		t.Fatal("the window must release the account at its deadline, without a sweeper")
	}
}

// TestUpstreamEndpoint_MarkQuotaThrottledOnlyExtends is the race rule: a second 429
// pushes the wait out, and an outcome that would shorten it leaves the deadline
// alone, so two concurrent writes cannot release the poller early.
func TestUpstreamEndpoint_MarkQuotaThrottledOnlyExtends(t *testing.T) {
	endpoint := cooldownEndpoint(t)
	endpoint.MarkQuotaThrottled(keyNow)
	stored := *endpoint.QuotaCooldownUntil()

	endpoint.MarkQuotaThrottled(keyNow.Add(-4 * time.Minute))
	if !endpoint.QuotaCooldownUntil().Equal(stored) {
		t.Fatalf("cooldown = %v, want the earlier 429 to leave %v alone", endpoint.QuotaCooldownUntil(), stored)
	}

	endpoint.MarkQuotaThrottled(keyNow.Add(10 * time.Minute))
	if got := *endpoint.QuotaCooldownUntil(); got != keyNow.Add(25*time.Minute) {
		t.Fatalf("cooldown = %v, want the newer 429 to extend it to 25m", got)
	}
}

// TestUpstreamEndpoint_QuotaCooldownNeverWithholdsServing is the finding's whole
// point, stated as an invariant: an account the vendor throttled for usage reads
// still routes traffic on its healthy keys.
func TestUpstreamEndpoint_QuotaCooldownNeverWithholdsServing(t *testing.T) {
	endpoint := cooldownEndpoint(t)
	endpoint.MarkQuotaThrottled(keyNow)

	if !endpoint.Available(keyNow) {
		t.Fatal("a quota cooldown must not remove a working account from rotation")
	}
	if !endpoint.QuotaThrottled(keyNow) {
		t.Fatal("the same endpoint must still read as throttled for the poller")
	}
}

// TestUpstreamEndpoint_QuotaCooldownSurvivesAParityReset pins that a served call
// clears the error block and leaves the cooldown to expire on its own: a successful
// model request says nothing about whether the vendor will answer a usage read.
func TestUpstreamEndpoint_QuotaCooldownSurvivesAParityReset(t *testing.T) {
	endpoint := cooldownEndpoint(t)
	endpoint.MarkQuotaThrottled(keyNow)
	stored := *endpoint.QuotaCooldownUntil()

	endpoint.RecordUpstreamSuccess(keyNow.Add(time.Minute))
	if until := endpoint.QuotaCooldownUntil(); until == nil || !until.Equal(stored) {
		t.Fatalf("cooldown = %v, want a served call to leave the %v deadline standing", until, stored)
	}
	if code, _, _ := endpoint.LastError(); code != "" {
		t.Fatalf("last error code = %q, want a served call to clear the parity block", code)
	}
}
