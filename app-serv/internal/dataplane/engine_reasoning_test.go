// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_reasoning_test.go
// @for       The relay's §7.15 reasoning seam: which body it sees, which
//
//	identity it is told, and where the model string's "(level)" suffix
//	travels.
//
// @uses      testing, context, net/http, net/http/httptest, strings,
//
//	internal/domain, internal/provider, internal/reasoning, internal/registry.
//
// @reason    The seam's contract is placement and identity, and both are only
//
//	observable end to end: a stub that records its inputs proves the
//	injection runs on the body the upstream will receive, after
//	resolution, and that the suffix is split off before resolution while
//	still reaching the seam. Kept beside the relay tests, whose doubles
//	it reuses, and out of engine_relay_test.go for the §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-26
package dataplane

import (
	"context"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/reasoning"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// thinkingStub is a ThinkingApplier double: it records every call's inputs and,
// when a rewrite is configured, replaces the body so the upstream proves it
// received the seam's answer rather than the translation's.
type thinkingStub struct {
	calls   int
	seen    []reasoning.Call
	bodies  []string
	rewrite []byte
}

func (s *thinkingStub) Apply(_ context.Context, body []byte, call reasoning.Call) []byte {
	s.calls++
	s.seen = append(s.seen, call)
	s.bodies = append(s.bodies, string(body))
	if s.rewrite == nil {
		return body
	}
	return s.rewrite
}

// TestRelay_ThinkingSeam pins the §7.15 placement contract across the three
// configurations a deployment can have: a seam that rewrites, a model string
// carrying a suffix, and no seam at all.
func TestRelay_ThinkingSeam(t *testing.T) {
	cases := []struct {
		name     string
		model    string
		rewrite  []byte
		wantSeam bool
		wantAt   string // content the upstream must have received
		wantNot  string // content that must not be there
		wantMode string // "" means the call carried no override
	}{
		{
			name:  "the seam rewrites the upstream body and the upstream receives it",
			model: "alpha/works",
			rewrite: []byte(`{"model":"works","messages":[{"role":"user","content":"thinking-rewritten"}],` +
				`"reasoning_effort":"high"}`),
			wantSeam: true,
			wantAt:   "thinking-rewritten", wantNot: "ping",
		},
		{
			name:     "a model-string suffix resolves the bare id and reaches the seam",
			model:    "alpha/works(high)",
			wantSeam: true,
			wantAt:   `"model":"works"`, wantNot: "works(high)",
			wantMode: "level",
		},
		{
			name:     "no seam leaves the pipeline pass-through",
			model:    "alpha/works(high)",
			wantSeam: false,
			wantAt:   `"model":"works"`, wantNot: "works(high)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var sent string
			server := newSaverUpstream(t, &sent)
			repo := newMemEndpointRepo()
			repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
			stub := &thinkingStub{rewrite: tc.rewrite}
			var seam ThinkingApplier
			if tc.wantSeam {
				seam = stub
			}
			engine := newSeamEngine(t,
				[]registry.Provider{relayProvider("alpha", server.URL)}, repo, nil, nil, seam)

			outcome, err := engine.Relay(context.Background(), relayRequest(tc.model), nil)
			if err != nil {
				t.Fatalf("Relay() error = %v", err)
			}
			if !strings.Contains(string(outcome.Body), "pong") {
				t.Fatalf("Outcome.Body = %s, want the upstream's answer", outcome.Body)
			}
			if !tc.wantSeam {
				if stub.calls != 0 {
					t.Fatalf("seam asks = %d, want none with no seam wired", stub.calls)
				}
			} else {
				if stub.calls != 1 {
					t.Fatalf("seam asks = %d, want one per upstream attempt", stub.calls)
				}
				call := stub.seen[0]
				if call.Wire != TargetOpenAI || call.ProviderID != "alpha" || call.ModelID != "works" {
					t.Fatalf("seam saw wire=%q provider=%q model=%q, want openai/alpha/works",
						call.Wire, call.ProviderID, call.ModelID)
				}
				if !strings.Contains(string(call.ClientRaw), `"model":"`+tc.model+`"`) {
					t.Fatalf("seam saw ClientRaw %s, want the body as the client sent it", call.ClientRaw)
				}
				if got := overrideMode(call.Override); got != tc.wantMode {
					t.Fatalf("seam saw override mode %q, want %q", got, tc.wantMode)
				}
				if tc.wantMode == "level" && call.Override.Level != "high" {
					t.Fatalf("seam saw override level %q, want high", call.Override.Level)
				}
			}
			if !strings.Contains(sent, tc.wantAt) {
				t.Fatalf("upstream received %s, want it to carry %q", sent, tc.wantAt)
			}
			if strings.Contains(sent, tc.wantNot) {
				t.Fatalf("upstream received %s, want no %q", sent, tc.wantNot)
			}
		})
	}
}

// TestRelay_ThinkingSeamSeesEachComboMember pins that the seam runs per upstream
// attempt under that member's identity, and that one suffix on the combo name
// travels to every leg of the call.
func TestRelay_ThinkingSeamSeesEachComboMember(t *testing.T) {
	var sent string
	failing := newRelayUpstream(t, new(int))
	works := newSaverUpstream(t, &sent)
	stub := &thinkingStub{}
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	repo.byProvider["beta"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-beta", "beta")}
	engine := newSeamEngine(t,
		[]registry.Provider{relayProvider("alpha", failing.URL), relayProvider("beta", works.URL)},
		repo, map[string]domain.Combo{"daily": comboRow("daily", "alpha/broken", "beta/works")},
		nil, stub)

	outcome, err := engine.Relay(context.Background(), relayRequest("daily(medium)"), nil)
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if !strings.Contains(string(outcome.Body), "pong") {
		t.Fatalf("Outcome.Body = %s, want the served member's answer", outcome.Body)
	}
	if stub.calls != 2 {
		t.Fatalf("seam asks = %d, want one per upstream attempt", stub.calls)
	}
	if stub.seen[0].ModelID != "broken" || stub.seen[1].ModelID != "works" {
		t.Fatalf("seam saw models %q/%q, want each attempt under its own identity",
			stub.seen[0].ModelID, stub.seen[1].ModelID)
	}
	for index, call := range stub.seen {
		if overrideMode(call.Override) != "level" || call.Override.Level != "medium" {
			t.Fatalf("attempt %d saw override %+v, want the combo's own suffix on every leg", index, call.Override)
		}
	}
}

// TestRelay_ThinkingSeamRunsBeforeTheSaver pins the reference's order: the
// reasoning injection writes the fields, and the token saver then rewrites the
// body the upstream actually receives. The stub's rewrite is the marker the
// saver's own input is read for, so a saver that ran first would see the
// translation's body instead.
func TestRelay_ThinkingSeamRunsBeforeTheSaver(t *testing.T) {
	var sent string
	server := newSaverUpstream(t, &sent)
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	saver := &saverStub{}
	thinking := &thinkingStub{rewrite: []byte(`{"model":"works","messages":[{"role":"user","content":"ping"}],` +
		`"reasoning_effort":"high"}`)}
	engine := newSeamEngine(t,
		[]registry.Provider{relayProvider("alpha", server.URL)}, repo, nil, saver, thinking)

	if _, err := engine.Relay(context.Background(), relayRequest("alpha/works(high)"), nil); err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if len(saver.bodies) != 1 || !strings.Contains(saver.bodies[0], `"reasoning_effort":"high"`) {
		t.Fatalf("saver saw %v, want the reasoning-normalized body", saver.bodies)
	}
	if !strings.Contains(sent, "saver-rewritten") {
		t.Fatalf("upstream received %s, want the saver's rewrite last", sent)
	}
}

// overrideMode reports the mode an override carries, or "" for none, so the
// assertions above read as one comparison rather than a nil check each.
func overrideMode(suffix *reasoning.Suffix) string {
	if suffix == nil {
		return ""
	}
	return suffix.Mode
}
