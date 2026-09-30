// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_vision_relay_test.go
// @for       How the relay walk consults the §7.8 vision seam.
//
// @uses      context, errors, testing, internal/domain, internal/schema.
// @reason    The seam decides a served request's model order, so the engine's
//
//	side of it — the whole candidate list handed over, the image-only
//	guard, the fail-open direction, and whose name an adapter answer is
//	recorded under — is one concern and gets one file. Splitting it out
//	also keeps engine_relay_test.go inside the AGENTS.md §1.1 budget
//	once the seam started reporting candidates rather than one model.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-29
package dataplane

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

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

// TestRelay_VisionAugmentationOrdersTheAdapterAndStripsIdentity pins §7.8: an
// image-bearing request is handed to the seam whole, and an adapter member that
// serves leaves the response identity with the model the client asked for rather
// than naming the adapter.
func TestRelay_VisionAugmentationOrdersTheAdapterAndStripsIdentity(t *testing.T) {
	var calls int
	server := newRelayUpstream(t, &calls)
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	vision := &visionAdapter{
		refs:    []string{"alpha/works", "alpha/broken"},
		adapted: []string{"alpha/works"},
	}
	engine := newRelayEngine(t, server.URL, repo, nil, vision)

	outcome, err := engine.Relay(context.Background(), imageRequest("alpha/broken"), nil)
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if vision.calls != 1 {
		t.Fatalf("augmenter asks = %d, want 1", vision.calls)
	}
	// The seam is given every candidate the request could be served by, not the one
	// the rotation happened to put first: judging only the leader is what let an
	// adapter displace a member that reads images.
	if len(vision.candidates) != 1 || vision.candidates[0] != "alpha/broken" {
		t.Fatalf("augmenter given %v, want the request's own candidates", vision.candidates)
	}
	if outcome.Model != "broken" {
		t.Fatalf("Outcome.Model = %q, want the requested model, not the adapter member", outcome.Model)
	}
	// The substitution the accounting row hides — the row is written under the
	// addressed model, so without this flag nothing says which model actually got
	// the image, and the client's answer now names the combo rather than either.
	if !outcome.VisionAdapted {
		t.Fatal("Outcome.VisionAdapted = false, want true: an adapter model served this answer")
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

// TestRelay_AnAdapterThatIsAlsoTheRequestsOwnModelIsNotSubstitution pins the
// collision the operator's configuration actually produces: one model named both
// a combo member and the §7.8 adapter. Reaching it then says nothing the caller
// did not ask for, so the answer must not be reported as borrowed and its usage
// row must not be rewritten — measured live with
// opencode/muse-spark-1.3-contributor-free, which is both.
func TestRelay_AnAdapterThatIsAlsoTheRequestsOwnModelIsNotSubstitution(t *testing.T) {
	var calls int
	server := newRelayUpstream(t, &calls)
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	vision := &visionAdapter{refs: []string{"alpha/works"}, adapted: []string{"alpha/works"}}
	engine := newRelayEngine(t, server.URL, repo, nil, vision)

	outcome, err := engine.Relay(context.Background(), imageRequest("alpha/works"), nil)
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if outcome.VisionAdapted {
		t.Fatal("Outcome.VisionAdapted = true, want false: the model that served is the one the request named")
	}
	if outcome.Model != "works" {
		t.Fatalf("Outcome.Model = %q, want the serving model left alone", outcome.Model)
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
	vision := &visionAdapter{
		refs:    []string{"alpha/works"},
		adapted: []string{"alpha/works"},
	}
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
