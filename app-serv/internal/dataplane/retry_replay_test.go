// Package dataplane implements the request-path data plane of app-serv.
//
// @file      internal/dataplane/retry_replay_test.go
// @for       The one case where a non-idempotent POST may use the entry's full retry
//
//	budget: a refusal the client never saw an answer for.
//
// @uses      internal/registry, net/http, testing.
// @reason    Measured 2026-09-28 against Qoder's free model: the identical request was
//
//	refused twice with a nested capacity complaint and served on the third
//	try. The POST cap stopped the gateway at two attempts, so a provider
//	that refuses before any byte of the answer reaches the caller looked
//	like a spent model. The rule that fixes it is narrow, a refusal read
//	out of the first frame leaves no answer to duplicate, and a cap lifted
//	too far would let the gateway re-bill a call the client already saw.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-28
package dataplane

import (
	"net/http"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

func TestDecideRetryReplayableRefusal(t *testing.T) {
	transient := retryPlugin{retryable: map[int]bool{
		http.StatusTooManyRequests: true, http.StatusBadGateway: true,
	}}
	wide := registry.Provider{Transport: registry.Transport{Retry: registry.Retry{DefaultAttempts: 6}}}
	short := registry.Provider{Transport: registry.Transport{Retry: registry.Retry{DefaultAttempts: 3}}}
	single := registry.Provider{Transport: registry.Transport{Retry: registry.Retry{DefaultAttempts: 1}}}

	cases := []struct {
		name      string
		entry     registry.Provider
		attempt   Attempt
		wantRetry bool
	}{
		{
			name: "a replayable refusal passes the point where the POST cap stopped it",
			// The capped case fails at this exact attempt, which is the whole bug.
			entry: wide, attempt: Attempt{Retries: 1, Status: http.StatusTooManyRequests, Replayable: true},
			wantRetry: true,
		},
		{
			name:      "the same POST without that fact stays capped at one retry",
			entry:     wide,
			attempt:   Attempt{Retries: 1, Status: http.StatusTooManyRequests},
			wantRetry: false,
		},
		{
			// Replayability lifts the POST cap, not the entry's own budget: an
			// unlimited replay would re-hit a provider that is genuinely down.
			name:      "the entry's budget still bounds a replayable refusal",
			entry:     short,
			attempt:   Attempt{Retries: 2, Status: http.StatusTooManyRequests, Replayable: true},
			wantRetry: false,
		},
		{
			name:      "an entry declaring a single attempt replays nothing either",
			entry:     single,
			attempt:   Attempt{Retries: 0, Status: http.StatusBadGateway, Replayable: true},
			wantRetry: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision := DecideRetry(tc.entry, transient, tc.attempt)
			if decision.Retry != tc.wantRetry {
				t.Fatalf("retry = %v, want %v (%+v)", decision.Retry, tc.wantRetry, tc.attempt)
			}
		})
	}
}
