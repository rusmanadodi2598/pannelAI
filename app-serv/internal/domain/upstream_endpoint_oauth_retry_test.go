// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_endpoint_oauth_retry_test.go
// @for       The OAuth dead-letter retry ladder: the window it opens, the rule that it only widens, and the instant the account is due to be visited again.
// @uses      testing, time.
// @reason    Draft 050 T2 decision ii replaced a grave with no exit by a widening visit, and the ladder is the whole policy: an account the vendor refused five times must be asked again soon enough to recover from an outage that has passed, and rarely enough that a revoked grant costs a day rather than a tick. Both bounds are rules, so they are pinned where the rule lives.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-13
package domain

import (
	"testing"
	"time"
)

// grave buries a fresh OAuth endpoint once, the way the worker's fifth failure does.
func grave(t *testing.T, at time.Time) *UpstreamEndpoint {
	t.Helper()
	endpoint, err := NewUpstreamEndpoint("ep_grave", "gemini", "grave", UpstreamAuthOAuth, 1, at)
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint() error = %v", err)
	}
	endpoint.ScheduleRefreshRetry(at)
	return &endpoint
}

// TestUpstreamEndpoint_ScheduleRefreshRetryWidensTheWindow pins the ladder and its
// ceiling: each failed visit doubles the wait, and an account refused for a long
// time is asked once a day instead of never or constantly.
func TestUpstreamEndpoint_ScheduleRefreshRetryWidensTheWindow(t *testing.T) {
	steps := []time.Duration{
		time.Hour, 2 * time.Hour, 4 * time.Hour, 8 * time.Hour, 16 * time.Hour,
		RefreshRetryCeiling, RefreshRetryCeiling, RefreshRetryCeiling,
	}
	endpoint, err := NewUpstreamEndpoint("ep_grave", "gemini", "grave", UpstreamAuthOAuth, 1, keyNow)
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint() error = %v", err)
	}
	at := keyNow
	for step, want := range steps {
		deadline := endpoint.ScheduleRefreshRetry(at)
		if got := deadline.Sub(at); got != want {
			t.Fatalf("visit %d: window = %v, want %v", step+1, got, want)
		}
		if got, want := endpoint.RefreshRetryRounds(), min(step+1, refreshRetryMaxStep); got != want {
			t.Fatalf("visit %d: rounds = %d, want %d", step+1, got, want)
		}
		at = deadline
	}
}

// TestUpstreamEndpoint_RefreshRetryDueOnlyAfterTheWindow pins the sweep's gate: the
// token of a buried account is expired, so "due" is true on every tick and only the
// window stands between the worker and a second burst.
func TestUpstreamEndpoint_RefreshRetryDueOnlyAfterTheWindow(t *testing.T) {
	endpoint := grave(t, keyNow)
	if endpoint.RefreshRetryDue(keyNow) || endpoint.RefreshRetryDue(keyNow.Add(time.Hour-time.Second)) {
		t.Fatal("a buried account inside its window must not be attempted")
	}
	if !endpoint.RefreshRetryDue(keyNow.Add(time.Hour)) {
		t.Fatal("the window's own instant is due: a later tick would only delay the visit")
	}
}

// TestUpstreamEndpoint_AnUnscheduledAccountIsAlwaysDue states the default the ladder
// depends on: the window withholds a grave and nothing else, so a live account or a
// freshly created one is never gated by it.
func TestUpstreamEndpoint_AnUnscheduledAccountIsAlwaysDue(t *testing.T) {
	endpoint, err := NewUpstreamEndpoint("ep_live", "gemini", "live", UpstreamAuthOAuth, 1, keyNow)
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint() error = %v", err)
	}
	if !endpoint.RefreshRetryDue(keyNow) {
		t.Fatal("an account with no dead-letter schedule must read as due")
	}
	if endpoint.RefreshRetryRounds() != 0 || endpoint.RefreshRetryAt() != nil {
		t.Fatal("a live account must carry no ladder state")
	}
}

// TestUpstreamEndpoint_ClearRefreshRetryEndsTheLadder is the recovery half: a grant
// the vendor accepted puts the account back on the token's own clock, so the next
// refresh waits for the new expiry rather than for the ladder's next visit.
func TestUpstreamEndpoint_ClearRefreshRetryEndsTheLadder(t *testing.T) {
	endpoint := grave(t, keyNow)
	endpoint.ScheduleRefreshRetry(keyNow.Add(time.Hour))
	if endpoint.RefreshRetryRounds() != 2 {
		t.Fatalf("rounds = %d, want two visits recorded", endpoint.RefreshRetryRounds())
	}

	endpoint.ClearRefreshRetry(keyNow.Add(2 * time.Hour))
	if endpoint.RefreshRetryAt() != nil || endpoint.RefreshRetryRounds() != 0 {
		t.Fatalf("ladder = (%v, %d), want both cleared by a grant that worked",
			endpoint.RefreshRetryAt(), endpoint.RefreshRetryRounds())
	}
	if !endpoint.RefreshRetryDue(keyNow.Add(2 * time.Hour)) {
		t.Fatal("a recovered account must be due on its token's own clock")
	}
}

// TestUpstreamEndpoint_ClearRefreshRetryOnALiveAccountIsQuiet pins that the refresh
// path may call this on every success: an account that was never buried keeps its
// updated_at rather than being restamped by a reset of state it does not have.
func TestUpstreamEndpoint_ClearRefreshRetryOnALiveAccountIsQuiet(t *testing.T) {
	endpoint, err := NewUpstreamEndpoint("ep_quiet", "gemini", "quiet", UpstreamAuthOAuth, 1, keyNow)
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint() error = %v", err)
	}
	before := endpoint.UpdatedAt()
	endpoint.ClearRefreshRetry(keyNow.Add(time.Minute))
	if endpoint.UpdatedAt() != before {
		t.Fatalf("updated_at = %v, want a no-op clear to leave it at %v", endpoint.UpdatedAt(), before)
	}
}
