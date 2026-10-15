// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_failover_actions_test.go
// @for       The credential walk obeying the action a failure class is configured to take: stop ends the chain, rotate keeps it walking and parks nothing.
// @uses      context, net/http, testing, time, internal/domain.
// @reason    The class policy is the operator's answer to "what does the router do with this failure", and until the walk reads it that answer only set a park's length. Each action is pinned against a real second account, so a policy that changes nothing cannot pass.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package dataplane

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// actionEngine wires a relay walk whose provider carries one class policy, so the
// test states the setting rather than the code path it is supposed to change.
func actionEngine(t *testing.T, status int, classes domain.FailureClassPolicies) (*Engine, *memEndpointRepo, *int) {
	t.Helper()
	var calls int
	server := credentialRefusingUpstream(t, &calls, "plain-sealed-ep-alpha", status)
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	repo.byProvider["beta"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-beta", "beta")}
	engine := newRelayEngine(t, server.URL, repo, map[string]domain.Combo{
		"daily": comboRow("daily", "alpha/m", "beta/m"),
	})
	engine.selector.health = &stubHealth{policies: map[string]domain.KeyHealthPolicy{
		"alpha": {
			AuthWindow:      time.Minute,
			RateLimitWindow: time.Minute,
			TransientWindow: time.Minute,
			FailureClasses:  classes,
		},
	}}
	return engine, repo, &calls
}

// TestRelay_AClassToldToStopEndsTheChain is the operator's answer winning over the
// code's: a throttle that would normally fail over to the next account is told to
// stop, so the second account is never called and the client gets the refusal.
func TestRelay_AClassToldToStopEndsTheChain(t *testing.T) {
	engine, _, calls := actionEngine(t, http.StatusTooManyRequests, domain.FailureClassPolicies{
		domain.FailureRateLimitTemporary: {Action: domain.FailureActionStop},
	})

	_, err := engine.Relay(context.Background(), relayRequest("daily"), nil)
	if err == nil {
		t.Fatal("Relay() = nil error, want the refusal the chain stopped on")
	}
	if code := AsError(err).Code; code != CodeRateLimited {
		t.Fatalf("code = %s, want %s", code, CodeRateLimited)
	}
	// Two calls, not three: the transport's one same-key retry, and no second
	// account. A stop that still walked would answer 3 here.
	if *calls != 2 {
		t.Fatalf("upstream calls = %d, want 2: the chain stopped before the next account", *calls)
	}
}

// TestRelay_AClassToldToRotateKeepsWalkingAndParksNothing pins the other half: a
// credential-shaped refusal the operator refuses to park still fails over, and the
// key it just spent returns to rotation untouched.
func TestRelay_AClassToldToRotateKeepsWalkingAndParksNothing(t *testing.T) {
	engine, repo, calls := actionEngine(t, http.StatusUnauthorized, domain.FailureClassPolicies{
		domain.FailureAuthInvalid: {Action: domain.FailureActionRotate},
	})

	outcome, err := engine.Relay(context.Background(), relayRequest("daily"), nil)
	if err != nil {
		t.Fatalf("Relay() error = %v, want the second account to serve", err)
	}
	if outcome.ProviderID != "beta" {
		t.Fatalf("Outcome.ProviderID = %q, want beta", outcome.ProviderID)
	}
	if *calls != 2 {
		t.Fatalf("upstream calls = %d, want 2", *calls)
	}
	parked := repo.health["uky-ep-alpha"]
	if parked.RateLimitedUntil() != nil {
		t.Fatalf("key parked until %v, want no withholding for a class told to rotate", parked.RateLimitedUntil())
	}
	if parked.ConsecutiveErrors() != 1 {
		t.Fatalf("consecutive errors = %d, want the failure still counted against the key", parked.ConsecutiveErrors())
	}
}

// TestRelay_TheDefaultActionsStillWalk proves the table changes nothing it is not
// asked to: with no entry configured, a 401 parks and fails over exactly as the
// rules file fixes it.
func TestRelay_TheDefaultActionsStillWalk(t *testing.T) {
	engine, repo, calls := actionEngine(t, http.StatusUnauthorized, nil)

	outcome, err := engine.Relay(context.Background(), relayRequest("daily"), nil)
	if err != nil {
		t.Fatalf("Relay() error = %v, want the second account to serve", err)
	}
	if outcome.ProviderID != "beta" {
		t.Fatalf("Outcome.ProviderID = %q, want beta", outcome.ProviderID)
	}
	if *calls != 2 {
		t.Fatalf("upstream calls = %d, want 2", *calls)
	}
	if parked := repo.health["uky-ep-alpha"]; parked.RateLimitedUntil() == nil {
		t.Fatal("refused key carries no park, want the auth window applied")
	}
}
