// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_relay_tokensaver_test.go
// @for       The relay's token-saver seam (SPEC-API-002 §8): which body the
//
//	pipeline sees, which model name it names, and where the bypass
//	flag travels.
//
// @uses      testing, context, io, net/http, net/http/httptest, strings,
//
//	internal/domain, internal/registry.
//
// @reason    The seam's contract is placement, and placement is only observable
//
//	end to end: a stub that records its inputs proves the saver runs on
//	the body the upstream will receive, after resolution (the member's
//	model, not the client's string) and before the call (the upstream
//	receives the rewrite). Kept beside the relay tests, whose doubles
//	it reuses, and out of engine_relay_test.go for the §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// saverStub is a TokenSaver double: it records what the seam handed it and, when
// not bypassed, rewrites the body with a marker message the upstream can see.
type saverStub struct {
	calls  int
	bodies []string
	wire   string
	models []string
	bypass bool
}

func (s *saverStub) Apply(_ context.Context, body []byte, wire, model string, bypass bool) []byte {
	s.calls++
	s.bodies = append(s.bodies, string(body))
	s.models = append(s.models, model)
	s.wire, s.bypass = wire, bypass
	if bypass {
		return body
	}
	return []byte(`{"model":"` + model + `","messages":[{"role":"user","content":"saver-rewritten"}]}`)
}

// newSaverUpstream answers any request with a complete OpenAI completion and
// records the raw bytes it was sent, which is what the placement assertions read.
func newSaverUpstream(t *testing.T, sent *string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("upstream could not read the request body: %v", err)
		}
		*sent = string(raw)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-upstream","object":"chat.completion","created":1,` +
			`"model":"works","choices":[{"index":0,"message":{"role":"assistant","content":"pong"},` +
			`"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`))
	}))
	t.Cleanup(server.Close)
	return server
}

// TestRelay_TokenSaverSeam pins SPEC-API-002 §8's placement contract across the
// three configurations a deployment can have: a saver that runs, the bypass
// header's flag, and no saver at all.
func TestRelay_TokenSaverSeam(t *testing.T) {
	cases := []struct {
		name       string
		saver      *saverStub
		bypass     bool
		wantAsks   int
		wantAt     string // content the upstream must have received
		wantNotAt  string // content that must not be there
		wantWire   string
		wantModel  string
		wantBypass bool
	}{
		{
			name:     "the saver rewrites the upstream body and the upstream receives it",
			saver:    &saverStub{},
			wantAsks: 1,
			wantAt:   "saver-rewritten", wantNotAt: "ping",
			wantWire: TargetOpenAI, wantModel: "works",
		},
		{
			name:  "the bypass flag travels and the body goes through untouched",
			saver: &saverStub{}, bypass: true,
			wantAsks: 1,
			wantAt:   "ping", wantNotAt: "saver-rewritten",
			wantWire: TargetOpenAI, wantModel: "works", wantBypass: true,
		},
		{
			name:     "no saver leaves the pipeline pass-through",
			saver:    nil,
			wantAsks: 0,
			wantAt:   "ping", wantNotAt: "saver-rewritten",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var sent string
			server := newSaverUpstream(t, &sent)
			repo := newMemEndpointRepo()
			repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
			// A nil *saverStub inside the interface is not a nil seam, so the
			// no-saver case hands the wiring a genuinely nil TokenSaver.
			var saver TokenSaver
			if tc.saver != nil {
				saver = tc.saver
			}
			engine := newSeamEngine(t,
				[]registry.Provider{relayProvider("alpha", server.URL)}, repo, nil, saver, nil)
			request := relayRequest("alpha/works")
			request.TokenSaverBypass = tc.bypass

			outcome, err := engine.Relay(context.Background(), request, nil)
			if err != nil {
				t.Fatalf("Relay() error = %v", err)
			}
			if !strings.Contains(string(outcome.Body), "pong") {
				t.Fatalf("Outcome.Body = %s, want the upstream's answer", outcome.Body)
			}
			if tc.saver != nil {
				if tc.saver.calls != tc.wantAsks {
					t.Fatalf("saver asks = %d, want %d", tc.saver.calls, tc.wantAsks)
				}
				if !strings.Contains(tc.saver.bodies[0], `"model":"works"`) {
					t.Fatalf("saver saw %s, want the upstream body naming the resolved member", tc.saver.bodies[0])
				}
				if strings.Contains(tc.saver.bodies[0], `"model":"alpha/works"`) {
					t.Fatalf("saver saw %s, want nothing of the client's raw model string", tc.saver.bodies[0])
				}
				if tc.saver.wire != tc.wantWire || tc.saver.models[0] != tc.wantModel {
					t.Fatalf("saver saw wire=%q model=%q, want %q/%q", tc.saver.wire, tc.saver.models[0], tc.wantWire, tc.wantModel)
				}
				if tc.saver.bypass != tc.wantBypass {
					t.Fatalf("saver saw bypass=%v, want %v", tc.saver.bypass, tc.wantBypass)
				}
			}
			if !strings.Contains(sent, tc.wantAt) {
				t.Fatalf("upstream received %s, want it to carry %q", sent, tc.wantAt)
			}
			if tc.wantNotAt != "" && strings.Contains(sent, tc.wantNotAt) {
				t.Fatalf("upstream received %s, want no %q", sent, tc.wantNotAt)
			}
		})
	}
}

// TestRelay_TokenSaverSeesEachComboMember pins that the seam runs per upstream
// attempt: a failing first member costs one ask under its identity, and the
// served member's rewrite is the body the answering upstream receives.
func TestRelay_TokenSaverSeesEachComboMember(t *testing.T) {
	var sent string
	failing := newRelayUpstream(t, new(int))
	works := newSaverUpstream(t, &sent)
	saver := &saverStub{}
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	repo.byProvider["beta"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-beta", "beta")}
	engine := newSeamEngine(t,
		[]registry.Provider{relayProvider("alpha", failing.URL), relayProvider("beta", works.URL)},
		repo, map[string]domain.Combo{"daily": comboRow("daily", "alpha/broken", "beta/works")},
		saver, nil)

	outcome, err := engine.Relay(context.Background(), relayRequest("daily"), nil)
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if !strings.Contains(string(outcome.Body), "pong") {
		t.Fatalf("Outcome.Body = %s, want the served member's answer", outcome.Body)
	}
	if saver.calls != 2 {
		t.Fatalf("saver asks = %d, want one per upstream attempt", saver.calls)
	}
	if saver.models[0] != "broken" || saver.models[1] != "works" {
		t.Fatalf("saver saw models %v, want [broken works], each attempt under its own identity", saver.models)
	}
	if !strings.Contains(sent, "saver-rewritten") {
		t.Fatalf("served upstream received %s, want the rewritten body", sent)
	}
}
