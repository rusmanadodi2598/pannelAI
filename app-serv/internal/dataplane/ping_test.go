// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/ping_test.go
// @for       The bounded probe: the request it invents and the outcome the
//
//	pipeline reports for it (SPEC-API-001 §7.7).
//
// @uses      testing, context, encoding/json, net/http, net/http/httptest, sync,
//
//	internal/domain.
//
// @reason    The combo test route is only as trustworthy as the probe behind it,
//
//	so the tests pin both halves: that the invented request is the cheap,
//	non-streaming turn §7.7 promises, and that a failure surfaces with the
//	pipeline's own code rather than a second error vocabulary.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// pingRecorder captures the body the probe's upstream received.
type pingRecorder struct {
	mu   sync.Mutex
	body map[string]any
}

func (r *pingRecorder) record(body map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.body = body
}

func (r *pingRecorder) last() map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body
}

// pingUpstream records the request and answers a minimal completion, so a test
// can assert the invented request and the reported outcome in one pass.
func pingUpstream(t *testing.T, recorder *pingRecorder) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("upstream received an undecodable body: %v", err)
		}
		recorder.record(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-ping","object":"chat.completion","created":1,` +
			`"model":"works","choices":[{"index":0,"message":{"role":"assistant","content":"p"},` +
			`"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	t.Cleanup(server.Close)
	return server
}

// TestPing_SendsABoundedProbe pins the invented request: one user turn reading
// "ping", a bounded ceiling, and no streaming — the cheapest call an upstream
// bills for, sent through the same pipeline every other request runs.
func TestPing_SendsABoundedProbe(t *testing.T) {
	recorder := &pingRecorder{}
	server := pingUpstream(t, recorder)
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	engine := newRelayEngine(t, server.URL, repo, nil)

	outcome, err := engine.Ping(context.Background(), "alpha/works")
	if err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	if outcome.ProviderID != "alpha" || outcome.EndpointID != "ep-alpha" || outcome.Model != "works" {
		t.Fatalf("Ping() routing identity = %s/%s/%s, want alpha/ep-alpha/works",
			outcome.ProviderID, outcome.EndpointID, outcome.Model)
	}
	if outcome.Streamed || outcome.Body == nil {
		t.Fatalf("Ping() streamed = %v, body nil = %v, want one complete non-streamed answer",
			outcome.Streamed, outcome.Body == nil)
	}
	if outcome.Usage == nil || outcome.Usage.PromptTokens != 1 {
		t.Fatalf("Ping() usage = %+v, want the upstream's accounting", outcome.Usage)
	}

	received := recorder.last()
	if received["model"] != "works" {
		t.Fatalf("upstream model = %v, want works (the ref after the provider split)", received["model"])
	}
	if maxTokens, ok := received["max_tokens"].(float64); !ok || int(maxTokens) != PingMaxTokens {
		t.Fatalf("upstream max_tokens = %v, want %d", received["max_tokens"], PingMaxTokens)
	}
	if stream, ok := received["stream"].(bool); !ok || stream {
		t.Fatalf("upstream stream = %v, want false", received["stream"])
	}
	messages, ok := received["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("upstream messages = %v, want exactly one turn", received["messages"])
	}
	turn, _ := messages[0].(map[string]any)
	if turn["role"] != "user" || turn["content"] != PingPrompt {
		t.Fatalf("upstream turn = %v, want one user turn reading %q", turn, PingPrompt)
	}
}

// TestPing_ReportsThePipelinesOwnFailure pins that a dead member and an
// unresolvable ref are told apart by the pipeline's codes, because the combo
// test route reports exactly this code per member.
func TestPing_ReportsThePipelinesOwnFailure(t *testing.T) {
	var calls int
	server := newRelayUpstream(t, &calls)
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	engine := newRelayEngine(t, server.URL, repo, nil)

	cases := []struct {
		name string
		ref  string
		code string
	}{
		{name: "a member whose upstream fails", ref: "alpha/broken", code: CodeUpstreamError},
		{name: "a provider that is not in the registry", ref: "ghost/model", code: CodeModelNotFound},
		{name: "a name that is not a combo or an alias", ref: "ghost", code: CodeModelNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := engine.Ping(context.Background(), tc.ref); err == nil {
				t.Fatalf("Ping(%q) = nil error, want %s", tc.ref, tc.code)
			} else if code := AsError(err).Code; code != tc.code {
				t.Fatalf("Ping(%q) code = %q, want %q", tc.ref, code, tc.code)
			}
		})
	}
}
