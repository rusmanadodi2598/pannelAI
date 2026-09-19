// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_relay_identity_test.go
// @for       The routing identity a failed relay call still reports (register
//
//	G17): the member that was attempted, and which member an exhausted
//	combo names.
//
// @uses      testing, context, internal/domain.
// @reason    The chat plane writes its usage row from the outcome the engine
//
//	returns, so a failure that reports nothing is a failure the panel
//	cannot show. Pinning the identity here keeps that rule beside the
//	pipeline rather than inside the service's recorder, and keeps
//	engine_relay_test.go inside the AGENTS.md §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestRelay_FailureKeepsTheAttemptedIdentity pins register G17: a call that
// fails at the upstream still reports which provider, endpoint, and model it was
// attempted against, because the chat plane writes its usage row from that
// identity — a zero outcome carries nothing to record.
func TestRelay_FailureKeepsTheAttemptedIdentity(t *testing.T) {
	var calls int
	server := newRelayUpstream(t, &calls)
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	engine := newRelayEngine(t, server.URL, repo, map[string]domain.Combo{
		"doomed": comboRow("doomed", "alpha/broken"),
	})

	outcome, err := engine.Relay(context.Background(), relayRequest("doomed"), nil)
	if err == nil {
		t.Fatal("Relay() = nil error, want the upstream failure")
	}
	if code := AsError(err).Code; code != CodeUpstreamError {
		t.Fatalf("Relay() code = %q, want %q", code, CodeUpstreamError)
	}
	if outcome.ProviderID != "alpha" || outcome.EndpointID != "ep-alpha" || outcome.Model != "broken" {
		t.Fatalf("Outcome identity = %s/%s/%s, want alpha/ep-alpha/broken (the attempted member)",
			outcome.ProviderID, outcome.EndpointID, outcome.Model)
	}
	if outcome.Combo != "doomed" {
		t.Fatalf("Outcome.Combo = %q, want doomed", outcome.Combo)
	}
	if outcome.Streamed {
		t.Fatal("Outcome.Streamed = true, want false for a non-streamed request")
	}
}

// TestRelay_ExhaustedComboReportsTheLastAttemptedMember pins which identity an
// exhausted combo reports: the member whose failure is the client's error, so a
// recorded row names the attempt the reported error belongs to.
func TestRelay_ExhaustedComboReportsTheLastAttemptedMember(t *testing.T) {
	var calls int
	server := newRelayUpstream(t, &calls)
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	repo.byProvider["beta"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-beta", "beta")}
	engine := newRelayEngine(t, server.URL, repo, map[string]domain.Combo{
		"doomed": comboRow("doomed", "alpha/broken", "beta/broken"),
	})

	outcome, err := engine.Relay(context.Background(), relayRequest("doomed"), nil)
	if err == nil {
		t.Fatal("Relay() = nil error, want the last member's upstream failure")
	}
	if outcome.ProviderID != "beta" || outcome.EndpointID != "ep-beta" || outcome.Model != "broken" {
		t.Fatalf("Outcome identity = %s/%s/%s, want beta/ep-beta/broken (the member whose failure is reported)",
			outcome.ProviderID, outcome.EndpointID, outcome.Model)
	}
	if calls != 2 {
		t.Fatalf("upstream calls = %d, want one per member", calls)
	}
}
