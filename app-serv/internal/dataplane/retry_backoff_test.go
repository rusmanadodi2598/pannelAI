// Package dataplane implements the request-path data plane of app-serv.
//
// @file      internal/dataplane/retry_backoff_test.go
// @for       The entry-declared seconds-scale backoff ladder that lets a replayable transient refusal be retried far enough apart to escape an upstream that recovers over seconds (Qoder's free model, draft 036 §9.2).
// @uses      fmt, testing, time, internal/registry.
// @reason    The default full-jitter backoff can draw a wait near zero, which re-fires straight into the same fail-streak. An entry that declares a backoff base is the only thing that changes here, and every other provider must keep its sub-second ladder, so both the floor and the untouched default are pinned against bounds that hold regardless of the jitter draw.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-28
package dataplane

import (
	"fmt"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// qoderLike is the shape the registry carries for Qoder's transient free-pool refusal:
// five attempts on a two-second base.
func qoderLike() registry.Provider {
	return registry.Provider{Transport: registry.Transport{Retry: registry.Retry{
		ByStatus:      map[int]int{429: 5},
		BackoffBaseMS: map[int]int{429: 2000},
	}}}
}

// TestRetryWaitFloorSpaced proves the floored ladder: each attempt waits at least
// half its window, the window doubles, and a long chain is capped, so a retry
// cannot collapse to ~0 and re-fire inside the same fail-streak.
func TestRetryWaitFloorSpaced(t *testing.T) {
	cases := []struct {
		retries int
		minWait time.Duration
		maxWait time.Duration
	}{
		{retries: 0, minWait: time.Second, maxWait: 2 * time.Second},
		{retries: 1, minWait: 2 * time.Second, maxWait: 4 * time.Second},
		{retries: 2, minWait: 4 * time.Second, maxWait: 8 * time.Second},
		{retries: 9, minWait: retryWaitCeiling / 2, maxWait: retryWaitCeiling},
	}
	entry := qoderLike()
	for _, tc := range cases {
		t.Run(fmt.Sprintf("retries=%d", tc.retries), func(t *testing.T) {
			for draw := 0; draw < 40; draw++ {
				got := retryWait(entry, 429, tc.retries)
				if got < tc.minWait || got > tc.maxWait {
					t.Fatalf("retryWait(429,%d) = %v, want within [%v,%v]", tc.retries, got, tc.minWait, tc.maxWait)
				}
			}
		})
	}
}

// TestRetryWaitDefaultUnchanged proves an entry that declares no backoff base keeps
// the shared sub-second ladder: a provider with no Qoder-style override cannot have
// its retry timing silently changed by this rule.
func TestRetryWaitDefaultUnchanged(t *testing.T) {
	entry := registry.Provider{Transport: registry.Transport{Retry: registry.Retry{DefaultAttempts: 3}}}
	for retries := 0; retries <= 4; retries++ {
		for draw := 0; draw < 40; draw++ {
			got := retryWait(entry, 429, retries)
			if got > backoffCeiling {
				t.Fatalf("retryWait default(%d) = %v, want at most the default ceiling %v", retries, got, backoffCeiling)
			}
		}
	}
}

// TestDecideRetrySpacedTransient proves the whole decision for a replayable Qoder
// refusal: it retries, and the wait is the floored seconds-scale window rather than
// the sub-second default. This is the difference between the free model looking
// broken and the gateway riding out the vendor's oscillation.
func TestDecideRetrySpacedTransient(t *testing.T) {
	plugin := retryPlugin{retryable: map[int]bool{429: true}}
	attempt := Attempt{Retries: 0, Status: 429, Replayable: true}

	decision := DecideRetry(qoderLike(), plugin, attempt)
	if !decision.Retry {
		t.Fatal("a replayable 429 with attempts left must retry")
	}
	if decision.After < time.Second || decision.After > 2*time.Second {
		t.Fatalf("After = %v, want the two-second base floored to [1s,2s]", decision.After)
	}
}
