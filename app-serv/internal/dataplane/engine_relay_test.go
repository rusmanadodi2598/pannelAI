// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_relay_test.go
// @for       The Engine.Relay end-to-end tests: a combo request that fails over
//
//	from a down member to a healthy one over a real HTTP upstream.
//
// @uses      testing, context, strings, internal/domain, internal/schema.
// @reason    SPEC-API-001 §10 makes "a CLI tool completes a request through
//
//	combo fallback" P1's exit criterion. That sentence is only proven
//	when the pipeline runs as one piece, resolve the combo, select the
//	account, call the first upstream, fail over on its failure, and
//	translate the second member's answer back, so the engine is driven
//	end to end instead of asserting each branch separately, and the
//	circuit writes each member's attempt left behind are checked too.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-18
package dataplane

import (
	"context"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestRelay_ComboFallsBackFromAFailingMember pins P1's exit criterion: a request
// addressed to a combo is served by the next member after the first one fails,
// the answer reaches the client in the client's format, and each member's
// attempt left the correct circuit state behind.
func TestRelay_ComboFallsBackFromAFailingMember(t *testing.T) {
	var calls int
	server := newRelayUpstream(t, &calls)
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	repo.byProvider["beta"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-beta", "beta")}
	engine := newRelayEngine(t, server.URL, repo, map[string]domain.Combo{
		"daily": comboRow("daily", "alpha/broken", "beta/works"),
	})

	outcome, err := engine.Relay(context.Background(), relayRequest("daily"), nil)
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}

	if outcome.Combo != "daily" {
		t.Fatalf("Outcome.Combo = %q, want daily", outcome.Combo)
	}
	if outcome.ProviderID != "beta" {
		t.Fatalf("Outcome.ProviderID = %q, want beta (the healthy member)", outcome.ProviderID)
	}
	if outcome.EndpointID != "ep-beta" {
		t.Fatalf("Outcome.EndpointID = %q, want ep-beta", outcome.EndpointID)
	}
	if outcome.Model != "works" {
		t.Fatalf("Outcome.Model = %q, want works", outcome.Model)
	}
	if !strings.Contains(string(outcome.Body), "pong") {
		t.Fatalf("Outcome.Body = %s, want the served member's answer", outcome.Body)
	}
	if outcome.Usage == nil || outcome.Usage.PromptTokens != 5 || outcome.Usage.CompletionTokens != 2 {
		t.Fatalf("Outcome.Usage = %+v, want the upstream's 5/2 accounting", outcome.Usage)
	}
	if calls != 2 {
		t.Fatalf("upstream calls = %d, want one per member with no same-target retry", calls)
	}

	// The circuit writes are part of the criterion: the failed member's key
	// carries one consecutive error, and the served member's key was reset.
	if failed := repo.health["uky-ep-alpha"]; failed.ConsecutiveErrors() != 1 {
		t.Fatalf("failing key consecutive errors = %d, want 1", failed.ConsecutiveErrors())
	}
	if served := repo.health["uky-ep-beta"]; served.ConsecutiveErrors() != 0 {
		t.Fatalf("served key consecutive errors = %d, want 0", served.ConsecutiveErrors())
	}
}

// TestRelay_ComboExhaustionReportsTheLastUpstreamFailure pins the other side of
// fallback: when every member fails the client gets an upstream error, not a
// validation one, and the last member's failure is the answer.
func TestRelay_ComboExhaustionReportsTheLastUpstreamFailure(t *testing.T) {
	var calls int
	server := newRelayUpstream(t, &calls)
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	repo.byProvider["beta"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-beta", "beta")}
	engine := newRelayEngine(t, server.URL, repo, map[string]domain.Combo{
		"doomed": comboRow("doomed", "alpha/broken", "beta/broken"),
	})

	_, err := engine.Relay(context.Background(), relayRequest("doomed"), nil)
	if err == nil {
		t.Fatal("Relay() = nil error, want the last member's upstream failure")
	}
	if code := AsError(err).Code; code != CodeUpstreamError {
		t.Fatalf("Relay() code = %q, want %q", code, CodeUpstreamError)
	}
	if calls != 2 {
		t.Fatalf("upstream calls = %d, want one per member", calls)
	}
}

// TestRelay_RequestFailureStopsTheCombo pins why fallback is not blind: a
// failure that would repeat identically on every member is reported without
// spending the remaining accounts, so no upstream is called at all.
func TestRelay_RequestFailureStopsTheCombo(t *testing.T) {
	var calls int
	server := newRelayUpstream(t, &calls)
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	engine := newRelayEngine(t, server.URL, repo, map[string]domain.Combo{
		"daily": comboRow("daily", "alpha/broken", "beta/works"),
	})

	// A messages-shaped request carrying no messages body cannot be translated
	// into any upstream format: the same failure would follow the request to
	// every member of the combo.
	request := relayRequest("daily")
	request.Route = RouteMessages
	request.ClientFormat = schema.FormatAnthropic
	request.Messages = nil

	_, err := engine.Relay(context.Background(), request, nil)
	if err == nil {
		t.Fatal("Relay() = nil error, want a validation failure")
	}
	if code := AsError(err).Code; code != CodeValidation {
		t.Fatalf("Relay() code = %q, want %q", code, CodeValidation)
	}
	if calls != 0 {
		t.Fatalf("upstream calls = %d, want 0: the combo must not be spent on a request failure", calls)
	}
}
