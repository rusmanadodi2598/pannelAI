// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_truncated_failover_test.go
// @for       The combo walk over a member that answered 200 with nothing in it.
// @uses      encoding/json, net/http, net/http/httptest, strings, testing,
//
//	internal/domain, internal/schema.
//
// @reason    A reasoning model that spends its whole output ceiling on thinking
//
//	answers 200 with an empty body (measured live on
//	muse-spark-1.3-contributor-free, 2026-09-28), and the walk only left a
//	member on an error, so a combo rotated onto that member served the
//	client nothing. These tests pin the other three answers the walk owes:
//	try the next member, do not park the key that answered, prefer an empty
//	body over an error, and leave a streamed answer alone because it has
//	already reached the client.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-29
package dataplane

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// sseDone terminates an SSE body the way an OpenAI stream does.
const sseDone = "\n\ndata: [DONE]\n\n"

// newTruncatingUpstream answers every model with a complete completion, except
// the models emptyByModel marks, which answer 200 with an empty message cut short
// by its ceiling, and "broken", which refuses. It answers a stream to a client
// that asked for one, so the streamed path can be told apart from the folded one.
func newTruncatingUpstream(t *testing.T, calls *int, emptyByModel map[string]bool) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		var body struct {
			Model  string `json:"model"`
			Stream bool   `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("upstream received an undecodable body: %v", err)
		}
		if body.Model == "broken" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"this member is down"}}`))
			return
		}
		empty := emptyByModel[body.Model]
		if body.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			if empty {
				_, _ = w.Write([]byte(`data: {"id":"chunk-1","object":"chat.completion.chunk","choices":` +
					`[{"index":0,"delta":{},"finish_reason":"length"}]` + sseDone))
				return
			}
			_, _ = w.Write([]byte(`data: {"id":"chunk-1","object":"chat.completion.chunk","choices":` +
				`[{"index":0,"delta":{"content":"pong"},"finish_reason":"stop"}]` + sseDone))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if empty {
			_, _ = w.Write([]byte(`{"id":"chatcmpl-empty","object":"chat.completion","created":1,` +
				`"model":"` + body.Model + `","choices":[{"index":0,"message":{"role":"assistant",` +
				`"content":""},"finish_reason":"length"}],` +
				`"usage":{"prompt_tokens":5,"completion_tokens":1,"total_tokens":6}}`))
			return
		}
		writeProbeCompletion(w, body.Model)
	}))
	t.Cleanup(server.Close)
	return server
}

// TestRelay_ComboSkipsAMemberThatAnsweredEmptyAtItsCeiling pins the reported case:
// a round_robin combo whose leading member spends the ceiling on thinking must
// serve the next member's answer, and must not treat the empty one as its key's
// fault — the upstream answered, so the credential stays usable.
func TestRelay_ComboSkipsAMemberThatAnsweredEmptyAtItsCeiling(t *testing.T) {
	var calls int
	server := newTruncatingUpstream(t, &calls, map[string]bool{"empty": true})
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	repo.byProvider["beta"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-beta", "beta")}
	engine := newRelayEngine(t, server.URL, repo, map[string]domain.Combo{
		"rotating": roundRobinRow("rotating", 1, "alpha/empty", "beta/works"),
	})

	outcome, err := engine.Relay(context.Background(), relayRequest("rotating"), nil)
	if err != nil {
		t.Fatalf("Relay() error = %v, want the healthy member's answer", err)
	}
	if outcome.ProviderID != "beta" || outcome.Model != "works" {
		t.Fatalf("served by %s/%s, want beta/works", outcome.ProviderID, outcome.Model)
	}
	if !strings.Contains(string(outcome.Body), "pong") {
		t.Fatalf("Outcome.Body = %s, want the second member's answer", outcome.Body)
	}
	if outcome.Truncated {
		t.Fatal("Outcome.Truncated = true, want false for the member that answered")
	}
	if calls != 2 {
		t.Fatalf("upstream calls = %d, want one per member", calls)
	}
	key, recorded := repo.health["uky-ep-alpha"]
	if !recorded {
		t.Fatal("the empty member's key was not recorded at all")
	}
	if key.ConsecutiveErrors() != 0 {
		t.Fatalf("empty member key consecutive errors = %d, want 0: it answered, so it is not parked",
			key.ConsecutiveErrors())
	}
}

// TestRelay_ComboServesTheEmptyAnswerWhenNoMemberAnswers pins the floor of the
// change: when every member came back empty, the client gets the body it would
// have got before, with the ceiling stop intact, rather than a 502 that hides why.
func TestRelay_ComboServesTheEmptyAnswerWhenNoMemberAnswers(t *testing.T) {
	var calls int
	server := newTruncatingUpstream(t, &calls, map[string]bool{"empty": true, "also-empty": true})
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	repo.byProvider["beta"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-beta", "beta")}
	engine := newRelayEngine(t, server.URL, repo, map[string]domain.Combo{
		"thinking": comboRow("thinking", "alpha/empty", "beta/also-empty"),
	})

	outcome, err := engine.Relay(context.Background(), relayRequest("thinking"), nil)
	if err != nil {
		t.Fatalf("Relay() error = %v, want the empty body served rather than an error", err)
	}
	if !outcome.Truncated {
		t.Fatal("Outcome.Truncated = false, want the served answer marked as the empty one")
	}
	if !strings.Contains(string(outcome.Body), `"finish_reason":"length"`) {
		t.Fatalf("Outcome.Body = %s, want the ceiling stop the upstream reported", outcome.Body)
	}
	if outcome.ProviderID != "beta" {
		t.Fatalf("served by %q, want the last member that answered", outcome.ProviderID)
	}
	if calls != 2 {
		t.Fatalf("upstream calls = %d, want one per member", calls)
	}
}

// TestRelay_SingleMemberEmptyAnswerIsServedUnchanged pins that the walk only
// applies where there is another member to try: a plain model request is not made
// to fail because the gateway has nothing to rotate to.
func TestRelay_SingleMemberEmptyAnswerIsServedUnchanged(t *testing.T) {
	var calls int
	server := newTruncatingUpstream(t, &calls, map[string]bool{"empty": true})
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	engine := newRelayEngine(t, server.URL, repo, map[string]domain.Combo{
		"lonely": roundRobinRow("lonely", 1, "alpha/empty"),
	})

	outcome, err := engine.Relay(context.Background(), relayRequest("lonely"), nil)
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("upstream calls = %d, want one: nothing else is in this combo", calls)
	}
	if outcome.ProviderID != "alpha" {
		t.Fatalf("served by %q, want alpha", outcome.ProviderID)
	}
}

// TestRelay_AnEmptyAnswerBeatsALaterMembersError pins the precedence the walk
// needs once both kinds of failure are on the table. A member that answered
// (empty) and a member that refused are not the same report to a client, and the
// answer is the one the client can act on.
func TestRelay_AnEmptyAnswerBeatsALaterMembersError(t *testing.T) {
	var calls int
	server := newTruncatingUpstream(t, &calls, map[string]bool{"empty": true})
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	repo.byProvider["beta"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-beta", "beta")}
	engine := newRelayEngine(t, server.URL, repo, map[string]domain.Combo{
		"mixed": comboRow("mixed", "alpha/empty", "beta/broken"),
	})

	outcome, err := engine.Relay(context.Background(), relayRequest("mixed"), nil)
	if err != nil {
		t.Fatalf("Relay() error = %v, want the empty answer served rather than the later refusal", err)
	}
	if outcome.ProviderID != "alpha" || !outcome.Truncated {
		t.Fatalf("served by %q (truncated %v), want alpha's empty answer", outcome.ProviderID, outcome.Truncated)
	}
	if calls != 2 {
		t.Fatalf("upstream calls = %d, want both members tried", calls)
	}
}

// TestRelay_AStreamedEmptyAnswerDoesNotRestartTheCombo pins the boundary of the
// change: a client on a stream has already received the frames, so the walk cannot
// hand it a second member's answer. It is served the one it was already sent.
func TestRelay_AStreamedEmptyAnswerDoesNotRestartTheCombo(t *testing.T) {
	var calls int
	server := newTruncatingUpstream(t, &calls, map[string]bool{"empty": true})
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	repo.byProvider["beta"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-beta", "beta")}
	engine := newRelayEngine(t, server.URL, repo, map[string]domain.Combo{
		"streaming": comboRow("streaming", "alpha/empty", "beta/works"),
	})

	request := relayRequest("streaming")
	request.Stream = true
	request.Chat = &schema.ChatRequest{Model: "streaming", Stream: true}
	request.Raw = []byte(`{"model":"streaming","stream":true,"messages":[{"role":"user","content":"ping"}]}`)
	sink := &recordingSink{}

	outcome, err := engine.Relay(context.Background(), request, sink)
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("upstream calls = %d, want the stream's own member only: frames are already delivered", calls)
	}
	if outcome.ProviderID != "alpha" {
		t.Fatalf("served by %q, want the member the stream started on", outcome.ProviderID)
	}
	if outcome.Truncated {
		t.Fatal("Outcome.Truncated = true, want false: a streamed answer is not read back")
	}
	if len(sink.frames) == 0 {
		t.Fatal("the sink received no frames")
	}
}
