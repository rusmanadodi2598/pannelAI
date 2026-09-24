// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_failover_rules_test.go
// @for       The failover rules the draft 028 fixes added, beyond the five
//
//	acceptance probes: credential-first order, the class-aware stop, the
//	combined client error, and first-failure parking.
//
// @uses      context, net/http, strings, testing, internal/domain, internal/registry.
// @reason    SPEC-API-001 §7.7 now fixes the order as credentials first and names
//
//	the exhausted chain's error as the first failure's status with the last
//	failure's message; §7.5 fixes the parking window per class. Each rule is
//	a separate decision, so each is pinned on its own rather than inferred
//	from the probes (AGENTS.md §2.1).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package dataplane

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestRelay_CredentialsAreTriedBeforeTheNextModel pins §7.7's order: the same
// provider's second account is tried before the combo's next model, so a client
// is served by the model it asked for even when the first account is refused.
func TestRelay_CredentialsAreTriedBeforeTheNextModel(t *testing.T) {
	var calls int
	server := credentialRefusingUpstream(t, &calls, "plain-sealed-ep-1", http.StatusInternalServerError)
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{
		relayEndpoint(t, "ep-1", "alpha"),
		relayEndpoint(t, "ep-2", "alpha"),
	}
	repo.byProvider["beta"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-beta", "beta")}
	engine := newRelayEngine(t, server.URL, repo, map[string]domain.Combo{
		"daily": comboRow("daily", "alpha/broken", "beta/works"),
	})

	outcome, err := engine.Relay(context.Background(), relayRequest("daily"), nil)
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if outcome.ProviderID != "alpha" || outcome.EndpointID != "ep-2" {
		t.Fatalf("Outcome identity = %s/%s, want alpha/ep-2: the provider's own second account comes before the next model",
			outcome.ProviderID, outcome.EndpointID)
	}
	if calls != 2 {
		t.Fatalf("upstream calls = %d, want 2: the refused account and the healthy one", calls)
	}
}

// TestRelay_ExhaustedChainReportsFirstStatusAndLastMessage pins §7.7's error
// shape: the first failure's status (what the client's retry logic reacts to)
// with the last failure's message (the most specific cause).
func TestRelay_ExhaustedChainReportsFirstStatusAndLastMessage(t *testing.T) {
	var calls int
	server := probeUpstream(t, &calls, map[string]int{
		"limited": http.StatusTooManyRequests,
		"broken":  http.StatusInternalServerError,
	})
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	repo.byProvider["beta"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-beta", "beta")}
	engine := newRelayEngine(t, server.URL, repo, map[string]domain.Combo{
		"doomed": comboRow("doomed", "alpha/limited", "beta/broken"),
	})

	_, err := engine.Relay(context.Background(), relayRequest("doomed"), nil)
	if err == nil {
		t.Fatal("Relay() = nil error, want the exhausted chain's failure")
	}
	failure := AsError(err)
	if failure.Code != CodeRateLimited || failure.Status != http.StatusTooManyRequests {
		t.Fatalf("client error = %s/%d, want the first failure's %s/%d",
			failure.Code, failure.Status, CodeRateLimited, http.StatusTooManyRequests)
	}
	if !strings.Contains(failure.Message, "broken") {
		t.Fatalf("client message = %q, want the last failure's message", failure.Message)
	}
}

// TestRelay_FailureClassDecidesWhetherTheChainContinues pins F2's rule per
// status: a request-shaped 4xx stops the chain and reaches the client as
// UPSTREAM_REJECTED without touching any key, while the credential-shaped ones
// (401/402/403/404/429) fail over to the next account, whose first failure
// parks the refused key.
func TestRelay_FailureClassDecidesWhetherTheChainContinues(t *testing.T) {
	cases := []struct {
		name      string
		status    int
		wantStop  bool
		wantCalls int
	}{
		{name: "400 stops the chain", status: http.StatusBadRequest, wantStop: true, wantCalls: 1},
		{name: "406 stops the chain", status: http.StatusNotAcceptable, wantStop: true, wantCalls: 1},
		{name: "409 stops the chain", status: http.StatusConflict, wantStop: true, wantCalls: 1},
		{name: "422 stops the chain", status: http.StatusUnprocessableEntity, wantStop: true, wantCalls: 1},
		{name: "401 fails over", status: http.StatusUnauthorized, wantCalls: 2},
		{name: "402 fails over", status: http.StatusPaymentRequired, wantCalls: 2},
		{name: "403 fails over", status: http.StatusForbidden, wantCalls: 2},
		{name: "404 fails over", status: http.StatusNotFound, wantCalls: 2},
		// One extra call: the transport retries a 429 once on the same target
		// before the credential is excluded (retry.go MaxPOSTRetries).
		{name: "429 fails over", status: http.StatusTooManyRequests, wantCalls: 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls int
			server := credentialRefusingUpstream(t, &calls, "plain-sealed-ep-alpha", tc.status)
			repo := newMemEndpointRepo()
			repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
			repo.byProvider["beta"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-beta", "beta")}
			engine := newRelayEngine(t, server.URL, repo, map[string]domain.Combo{
				"daily": comboRow("daily", "alpha/m", "beta/m"),
			})

			outcome, err := engine.Relay(context.Background(), relayRequest("daily"), nil)
			if calls != tc.wantCalls {
				t.Fatalf("upstream calls = %d, want %d", calls, tc.wantCalls)
			}
			if tc.wantStop {
				if err == nil {
					t.Fatal("Relay() = nil error, want the upstream's refusal")
				}
				if code := AsError(err).Code; code != CodeUpstreamRejected {
					t.Fatalf("code = %s, want %s", code, CodeUpstreamRejected)
				}
				if len(repo.health) != 0 {
					t.Fatalf("health writes = %+v, want none for a request-shaped refusal", repo.health)
				}
				return
			}
			if err != nil {
				t.Fatalf("Relay() error = %v, want the second account to serve", err)
			}
			if outcome.ProviderID != "beta" {
				t.Fatalf("Outcome.ProviderID = %q, want beta (the account that served)", outcome.ProviderID)
			}
			parked := repo.health["uky-ep-alpha"]
			if parked.Status() != domain.UpstreamKeyError {
				t.Fatalf("refused key status = %q, want %q after its first failure",
					parked.Status(), domain.UpstreamKeyError)
			}
			if parked.RateLimitedUntil() == nil || parked.ConsecutiveErrors() != 1 {
				t.Fatalf("refused key = window %v, errors %d; want a parked window after one failure",
					parked.RateLimitedUntil(), parked.ConsecutiveErrors())
			}
		})
	}
}
