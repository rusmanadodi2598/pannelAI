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
//	when the pipeline runs as one piece — resolve the combo, select the
//	account, call the first upstream, fail over on its failure, and
//	translate the second member's answer back — so the engine is driven
//	end to end instead of asserting each branch separately, and the
//	circuit writes each member's attempt left behind are checked too.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-18
package dataplane

import (
	"context"
	"errors"
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
	engine := newRelayEngine(t, server.URL, repo, map[string][]string{
		"daily": {"alpha/broken", "beta/works"},
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
	engine := newRelayEngine(t, server.URL, repo, map[string][]string{
		"doomed": {"alpha/broken", "beta/broken"},
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
	engine := newRelayEngine(t, server.URL, repo, map[string][]string{
		"daily": {"alpha/broken", "beta/works"},
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

// imageRequest is a client request whose only message carries an image part.
func imageRequest(model string) Request {
	request := relayRequest(model)
	request.Chat = &schema.ChatRequest{
		Model: model,
		Messages: []schema.ChatMessage{{
			Role: "user",
			Content: schema.MessageContent{Parts: []schema.ContentPart{{
				Type: schema.PartImageURL, ImageURL: &schema.ImageURL{URL: "https://example.test/pixel.png"},
			}}},
		}},
	}
	request.Raw = []byte(`{"model":"` + model + `","messages":[{"role":"user","content":[` +
		`{"type":"image_url","image_url":{"url":"https://example.test/pixel.png"}}]}]}`)
	return request
}

// TestRelay_VisionAugmentationPrependsAdapterModelsAndStripsIdentity pins §7.8:
// an image-bearing request aimed at a model that cannot read images tries the
// adapter's models first, and an adapter member that serves leaves the response
// identity with the model the client asked for rather than naming the adapter.
func TestRelay_VisionAugmentationPrependsAdapterModelsAndStripsIdentity(t *testing.T) {
	var calls int
	server := newRelayUpstream(t, &calls)
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	vision := &visionAdapter{refs: []string{"alpha/works"}, applies: true}
	engine := newRelayEngine(t, server.URL, repo, nil, vision)

	outcome, err := engine.Relay(context.Background(), imageRequest("alpha/broken"), nil)
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if vision.calls != 1 {
		t.Fatalf("augmenter asks = %d, want 1", vision.calls)
	}
	if vision.providerID != "alpha" || vision.modelID != "broken" {
		t.Fatalf("augmenter asked for %s/%s, want alpha/broken", vision.providerID, vision.modelID)
	}
	if outcome.Model != "broken" {
		t.Fatalf("Outcome.Model = %q, want the requested model, not the adapter member", outcome.Model)
	}
	if outcome.ProviderID != "alpha" || outcome.EndpointID != "ep-alpha" {
		t.Fatalf("Outcome routing identity = %s/%s, want alpha/ep-alpha (the adapter member that served)",
			outcome.ProviderID, outcome.EndpointID)
	}
	if !strings.Contains(string(outcome.Body), "pong") {
		t.Fatalf("Outcome.Body = %s, want the adapter member's answer", outcome.Body)
	}
	if calls != 1 {
		t.Fatalf("upstream calls = %d, want 1: the adapter member served on the first try", calls)
	}
}

// TestRelay_TextRequestsAreNeverAugmented pins the seam's guard: the adapter is
// consulted only when the body carries image content, whatever the adapter has
// stored.
func TestRelay_TextRequestsAreNeverAugmented(t *testing.T) {
	var calls int
	server := newRelayUpstream(t, &calls)
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	vision := &visionAdapter{refs: []string{"alpha/works"}, applies: true}
	engine := newRelayEngine(t, server.URL, repo, nil, vision)

	_, err := engine.Relay(context.Background(), relayRequest("alpha/broken"), nil)
	if err == nil {
		t.Fatal("Relay() = nil error, want the upstream failure a text request meets un-augmented")
	}
	if vision.calls != 0 {
		t.Fatalf("augmenter asks = %d, want 0: a text request must not reach the adapter", vision.calls)
	}
}

// TestRelay_VisionSeamFailureFailsOpen pins the failure direction: an adapter
// that cannot answer its own question must not take the request down with it,
// so the request is served un-augmented and reports the upstream's failure.
func TestRelay_VisionSeamFailureFailsOpen(t *testing.T) {
	var calls int
	server := newRelayUpstream(t, &calls)
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	vision := &visionAdapter{err: errors.New("rotation store is down")}
	engine := newRelayEngine(t, server.URL, repo, nil, vision)

	_, err := engine.Relay(context.Background(), imageRequest("alpha/broken"), nil)
	if err == nil {
		t.Fatal("Relay() = nil error, want the un-augmented upstream failure")
	}
	if code := AsError(err).Code; code != CodeUpstreamError {
		t.Fatalf("Relay() code = %q, want the upstream error, not the augmenter's failure", code)
	}
	if vision.calls != 1 {
		t.Fatalf("augmenter asks = %d, want 1", vision.calls)
	}
	// Fail-open proceeds against the model the client named: one attempt at the
	// intended upstream, no adapter members tried, and the client's error is the
	// upstream's failure rather than the augmenter's.
	if calls != 1 {
		t.Fatalf("upstream calls = %d, want 1: the request must run against the named model only", calls)
	}
}
