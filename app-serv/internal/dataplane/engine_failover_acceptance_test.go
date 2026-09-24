// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_failover_acceptance_test.go
// @for       The five acceptance probes draft 028 section 2.4 defines, kept as
//
//	permanent tests: credential-first failover, member identity, the
//	request-scoped refusal, and key rotation.
//
// @uses      context, encoding/json, net/http, net/http/httptest, strings,
//
//	testing, internal/domain, internal/registry.
//
// @reason    The draft measured these behaviours as five failures and made them
//
//	the completion criteria of the F1-F4 fixes, so keeping them as tests
//	means a later change that re-introduces model-first failover, a stale
//	member owning the error, or a pinned key breaks a test rather than
//	only disagreeing with a document (AGENTS.md section 2.1).
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
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestRelay_CredentialFailoverTriesTheNextAccount is draft 028 probe 1: one
// request against a provider whose first account is refused is served by that
// provider's second account.
func TestRelay_CredentialFailoverTriesTheNextAccount(t *testing.T) {
	var calls int
	server := credentialRefusingUpstream(t, &calls, "plain-sealed-ep-1", http.StatusInternalServerError)
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{
		relayEndpoint(t, "ep-1", "alpha"),
		relayEndpoint(t, "ep-2", "alpha"),
	}
	engine := newRelayEngine(t, server.URL, repo, nil)

	outcome, err := engine.Relay(context.Background(), relayRequest("alpha/m"), nil)
	if err != nil {
		t.Fatalf("request failed although a second healthy account exists: calls=%d err=%v", calls, err)
	}
	if calls != 2 {
		t.Fatalf("upstream calls = %d, want 2 (one per account)", calls)
	}
	if outcome.EndpointID != "ep-2" {
		t.Fatalf("Outcome.EndpointID = %q, want ep-2 (the account that served)", outcome.EndpointID)
	}
}

// TestRelay_UnroutableMemberDoesNotOwnTheError is draft 028 probe 2: the member
// that was never called (its provider has no endpoint) must not supply the
// client's error or the recorded identity.
func TestRelay_UnroutableMemberDoesNotOwnTheError(t *testing.T) {
	var calls int
	server := probeUpstream(t, &calls, map[string]int{"broken": http.StatusInternalServerError})
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	providers := []registry.Provider{relayProvider("alpha", server.URL), relayProvider("ghost", server.URL)}
	engine := newEngineWith(t, providers, repo, map[string]domain.Combo{
		"daily": comboRow("daily", "alpha/broken", "ghost/anything"),
	}, nil)

	outcome, err := engine.Relay(context.Background(), relayRequest("daily"), nil)
	if err == nil {
		t.Fatal("want the called member's failure")
	}
	if outcome.ProviderID == "ghost" || AsError(err).Code == CodeNoProvider {
		t.Fatalf("the never-called member owns the outcome: provider=%q code=%s", outcome.ProviderID, AsError(err).Code)
	}
	if outcome.ProviderID != "alpha" || outcome.EndpointID != "ep-alpha" {
		t.Fatalf("Outcome identity = %s/%s, want alpha/ep-alpha (the member that was called)",
			outcome.ProviderID, outcome.EndpointID)
	}
	if calls != 1 {
		t.Fatalf("upstream calls = %d, want 1: only the callable member was called", calls)
	}
}

// TestRelay_UnresolvableMemberDoesNotOwnTheError is draft 028 probe 3: a stale
// member whose model cannot be resolved does not become the client's error.
func TestRelay_UnresolvableMemberDoesNotOwnTheError(t *testing.T) {
	var calls int
	server := probeUpstream(t, &calls, map[string]int{"broken": http.StatusInternalServerError})
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	strict := relayProvider("strict", server.URL)
	strict.PassthroughModels = false
	engine := newEngineWith(t, []registry.Provider{relayProvider("alpha", server.URL), strict}, repo,
		map[string]domain.Combo{"daily": comboRow("daily", "alpha/broken", "strict/missing")}, nil)

	_, err := engine.Relay(context.Background(), relayRequest("daily"), nil)
	if err == nil {
		t.Fatal("want the called member's failure")
	}
	if code := AsError(err).Code; code != CodeUpstreamError {
		t.Fatalf("the stale member owns the client error: code=%s message=%s", code, AsError(err).Message)
	}
	if message := AsError(err).Message; !strings.Contains(message, "broken") {
		t.Fatalf("client message = %q, want the called member's failure", message)
	}
}

// TestRelay_RequestScopedRefusalIsReturnedToTheClient is draft 028 probe 4: a
// 400 the upstream aimed at the request stops the chain instead of spending the
// next model, and the client sees that 400.
func TestRelay_RequestScopedRefusalIsReturnedToTheClient(t *testing.T) {
	var calls int
	server := probeUpstream(t, &calls, map[string]int{"bad": http.StatusBadRequest})
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	engine := newRelayEngine(t, server.URL, repo, map[string]domain.Combo{
		"daily": comboRow("daily", "alpha/bad", "alpha/works"),
	})

	_, err := engine.Relay(context.Background(), relayRequest("daily"), nil)
	if err == nil {
		t.Fatal("want the upstream's refusal")
	}
	failure := AsError(err)
	if failure.Code != CodeUpstreamRejected {
		t.Fatalf("code = %s, want %s", failure.Code, CodeUpstreamRejected)
	}
	if failure.Status != http.StatusBadRequest {
		t.Fatalf("client status = %d, want 400", failure.Status)
	}
	if calls != 1 {
		t.Fatalf("upstream calls = %d, want 1: the chain must stop at the refusal", calls)
	}
	if len(repo.health) != 0 {
		t.Fatalf("health writes = %+v, want none for a request-shaped refusal", repo.health)
	}
}

// TestRelay_KeysRotateWithinAnEndpoint is draft 028 probe 5: two healthy keys on
// one endpoint must both be spent, because the least-recently-used order rotates
// what the next request picks.
func TestRelay_KeysRotateWithinAnEndpoint(t *testing.T) {
	var calls int
	server := probeUpstream(t, &calls, nil)
	repo := newMemEndpointRepo()
	endpoint := relayEndpoint(t, "ep-1", "alpha")
	endpoint.AttachKey(domain.RehydrateUpstreamKey(
		"uky-k2", "ep-1", "key2", "sealed-k2", "…hint2",
		1, domain.UpstreamKeyActive, nil, "", 0, nil, now, now,
	))
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{endpoint}
	engine := rotatingEngine(t, server.URL, repo, 1)

	for i := 0; i < 3; i++ {
		if _, err := engine.Relay(context.Background(), relayRequest("alpha/m"), nil); err != nil {
			t.Fatalf("request %d error = %v", i+1, err)
		}
	}
	if _, used := repo.health["uky-k2"]; !used {
		t.Fatalf("key 2 was never selected in 3 requests; keys do not rotate")
	}
}
