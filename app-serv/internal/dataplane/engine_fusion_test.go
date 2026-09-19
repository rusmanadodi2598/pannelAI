// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_fusion_test.go
// @for       Engine.Relay over a fusion combo: the fan-out, the judge synthesis,
//
//	and the degradation the reference defines (SPEC-API-001 §7.7).
//
// @uses      testing, context, strings, internal/domain, internal/schema.
// @reason    SPEC-API-001 §10 makes fusion part of P2, and the strategy is only
//
//	proven when the pipeline runs as one piece: resolve the combo, fan
//	out to every member in parallel, hand the panel's prose to the judge,
//	and serve the judge's answer in the client's format.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"context"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// fusionEngine wires a three-provider engine over the stand-in upstream: alpha
// and beta are the panel, gamma is the judge.
func fusionEngine(t *testing.T, upstream *fusionUpstream, combo domain.Combo) (*Engine, *memEndpointRepo) {
	t.Helper()
	server := newFusionServer(t, upstream)
	repo := newMemEndpointRepo()
	for _, providerID := range []string{"alpha", "beta", "gamma"} {
		repo.byProvider[providerID] = []domain.UpstreamEndpoint{
			relayEndpoint(t, "ep-"+providerID, providerID),
		}
	}
	engine := newEngineWith(t, fusionProviders(server.URL, "alpha", "beta", "gamma"), repo,
		map[string]domain.Combo{combo.Name(): combo}, nil)
	return engine, repo
}

// TestRelay_FusionFansOutAndServesTheJudgesAnswer pins the whole strategy: both
// panel members are consulted in parallel and never streamed, the judge receives
// the panel's answers and the client's tools, and the client is served the
// judge's answer under the combo's name.
func TestRelay_FusionFansOutAndServesTheJudgesAnswer(t *testing.T) {
	upstream := &fusionUpstream{}
	combo := fusionRow("panel", "gamma/judge", "alpha/one", "beta/two")
	engine, _ := fusionEngine(t, upstream, combo)

	outcome, err := engine.Relay(context.Background(), fusionRequest("panel", true, false), nil)
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}

	if outcome.Combo != "panel" {
		t.Fatalf("Outcome.Combo = %q, want panel", outcome.Combo)
	}
	if outcome.ProviderID != "gamma" || outcome.Model != "judge" {
		t.Fatalf("Outcome routing identity = %s/%s, want the judge gamma/judge",
			outcome.ProviderID, outcome.Model)
	}
	if !strings.Contains(string(outcome.Body), "answer from judge") {
		t.Fatalf("Outcome.Body = %s, want the judge's answer", outcome.Body)
	}
	if outcome.Usage == nil || outcome.Usage.PromptTokens != 3 {
		t.Fatalf("Outcome.Usage = %+v, want the judge's accounting", outcome.Usage)
	}

	for _, member := range []string{"one", "two"} {
		calls := upstream.callsFor(member)
		if len(calls) != 1 {
			t.Fatalf("panel member %s calls = %d, want exactly 1", member, len(calls))
		}
		if calls[0].Stream {
			t.Fatalf("panel member %s was streamed, want a non-streamed panel call", member)
		}
		if calls[0].Tools {
			t.Fatalf("panel member %s received tools, want the declarations withdrawn", member)
		}
	}

	judgeCalls := upstream.callsFor("judge")
	if len(judgeCalls) != 1 {
		t.Fatalf("judge calls = %d, want exactly 1", len(judgeCalls))
	}
	if !judgeCalls[0].Tools {
		t.Fatal("the judge did not receive the client's tools, want them kept for the served call")
	}
	prompt := judgeCalls[0].LastTurn
	if !strings.Contains(prompt, "You are the JUDGE") {
		t.Fatalf("the judge's last turn is not the synthesis directive: %s", prompt)
	}
	first := strings.Index(prompt, "answer from one")
	second := strings.Index(prompt, "answer from two")
	if first < 0 || second < 0 {
		t.Fatalf("the judge's directive is missing a panel answer: %s", prompt)
	}
	if first > second {
		t.Fatalf("the judge's sources are out of member order: %s", prompt)
	}
	if !strings.Contains(prompt, "[Source 1]") || !strings.Contains(prompt, "[Source 2]") {
		t.Fatalf("the judge's directive does not number its sources: %s", prompt)
	}
}

// TestRelay_FusionDegradesWhenThePanelIsThin is the degradation matrix the
// reference defines: a full panel is fused, a single surviving answer is served
// directly without a judge call, and an empty panel reports the panel's failure.
func TestRelay_FusionDegradesWhenThePanelIsThin(t *testing.T) {
	cases := []struct {
		name           string
		refs           []string
		wantModel      string
		wantErrCode    string
		wantJudgeCalls int
	}{
		{
			name:           "two answers are fused by the judge",
			refs:           []string{"alpha/one", "beta/two"},
			wantModel:      "judge",
			wantJudgeCalls: 1,
		},
		{
			name:      "one answer is served directly, so no judge is spent",
			refs:      []string{"alpha/broken", "beta/two"},
			wantModel: "two",
		},
		{
			name:        "no answer reports the panel's failure",
			refs:        []string{"alpha/broken", "beta/broken"},
			wantErrCode: CodeUpstreamError,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			upstream := &fusionUpstream{}
			combo := fusionRow("panel", "gamma/judge", testCase.refs...)
			engine, _ := fusionEngine(t, upstream, combo)

			outcome, err := engine.Relay(context.Background(), fusionRequest("panel", false, false), nil)
			if testCase.wantErrCode != "" {
				if err == nil {
					t.Fatalf("Relay() = nil error, want %s", testCase.wantErrCode)
				}
				if code := AsError(err).Code; code != testCase.wantErrCode {
					t.Fatalf("Relay() code = %q, want %q", code, testCase.wantErrCode)
				}
			} else {
				if err != nil {
					t.Fatalf("Relay() error = %v", err)
				}
				if outcome.Model != testCase.wantModel {
					t.Fatalf("Outcome.Model = %q, want %q", outcome.Model, testCase.wantModel)
				}
			}

			if judgeCalls := len(upstream.callsFor("judge")); judgeCalls != testCase.wantJudgeCalls {
				t.Fatalf("judge calls = %d, want %d", judgeCalls, testCase.wantJudgeCalls)
			}
		})
	}
}
