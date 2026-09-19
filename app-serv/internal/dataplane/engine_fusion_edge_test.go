// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_fusion_edge_test.go
// @for       The fusion edges: a client that asked for SSE, a panel thinned to
//
//	one answer, and a reference that no longer resolves.
//
// @uses      testing, context, strings.
// @reason    Each edge decides the shape of the served answer rather than the
//
//	panel's composition, so they are pinned together: a streamed client
//	must receive a stream, and a stale reference must cost its own slot.
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
)

// TestRelay_FusionKeepsTheClientsStreamForTheJudge pins the split the reference
// chose: the panel is forced non-streaming so there is prose to fuse, while the
// judge's answer streams to the client exactly as the client asked.
func TestRelay_FusionKeepsTheClientsStreamForTheJudge(t *testing.T) {
	upstream := &fusionUpstream{}
	combo := fusionRow("panel", "gamma/judge", "alpha/one", "beta/two")
	engine, _ := fusionEngine(t, upstream, combo)

	request := fusionRequest("panel", false, true)
	sink := &recordSink{}

	outcome, err := engine.Relay(context.Background(), request, sink)
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if !outcome.Streamed {
		t.Fatal("Outcome.Streamed = false, want the judge's streamed answer")
	}
	if outcome.Body != nil {
		t.Fatalf("Outcome.Body = %s, want nil for a streamed answer", outcome.Body)
	}
	for _, member := range []string{"one", "two"} {
		if calls := upstream.callsFor(member); len(calls) != 1 || calls[0].Stream {
			t.Fatalf("panel member %s calls = %+v, want one non-streamed call", member, calls)
		}
	}
	judgeCalls := upstream.callsFor("judge")
	if len(judgeCalls) != 1 || !judgeCalls[0].Stream {
		t.Fatalf("judge calls = %+v, want one streamed call", judgeCalls)
	}
	body := sink.body()
	if !strings.Contains(body, "judged answer") {
		t.Fatalf("sink received %q, want the judge's streamed answer", body)
	}
	if !strings.Contains(body, "[DONE]") {
		t.Fatalf("sink received %q, want the terminal marker", body)
	}
	if sink.flush == 0 {
		t.Fatal("sink was never flushed, want each frame flushed as it arrived")
	}
}

// TestRelay_FusionStreamsAThinPanelFromTheSurvivingMember pins the wire contract
// for the degraded streaming case: a client that asked for SSE must receive SSE,
// so the one surviving member is re-issued with the client's own request rather
// than served the panel's prose-only answer.
func TestRelay_FusionStreamsAThinPanelFromTheSurvivingMember(t *testing.T) {
	upstream := &fusionUpstream{}
	combo := fusionRow("panel", "gamma/judge", "alpha/broken", "beta/two")
	engine, _ := fusionEngine(t, upstream, combo)
	sink := &recordSink{}

	outcome, err := engine.Relay(context.Background(), fusionRequest("panel", false, true), sink)
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if !outcome.Streamed {
		t.Fatal("Outcome.Streamed = false, want the surviving member's stream")
	}
	if len(upstream.callsFor("judge")) != 0 {
		t.Fatal("the judge was called, want a thin panel served without synthesis")
	}
	calls := upstream.callsFor("two")
	if len(calls) != 2 {
		t.Fatalf("surviving member calls = %d, want the panel call plus the streamed re-issue", len(calls))
	}
	if calls[0].Stream || !calls[1].Stream {
		t.Fatalf("surviving member calls = %+v, want a non-streamed panel call then a streamed one", calls)
	}
	if body := sink.body(); !strings.Contains(body, "[DONE]") {
		t.Fatalf("sink received %q, want a terminated stream", body)
	}
}

// TestRelay_FusionSkipsAReferenceThatNoLongerResolves pins the omission rule: a
// combo entry pointing at a provider that left the registry costs its own panel
// slot, not the request.
func TestRelay_FusionSkipsAReferenceThatNoLongerResolves(t *testing.T) {
	upstream := &fusionUpstream{}
	combo := fusionRow("panel", "gamma/judge", "gone/model", "alpha/one", "beta/two")
	engine, _ := fusionEngine(t, upstream, combo)

	outcome, err := engine.Relay(context.Background(), fusionRequest("panel", false, false), nil)
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if outcome.Model != "judge" {
		t.Fatalf("Outcome.Model = %q, want the judge to have fused the two live members", outcome.Model)
	}
	if len(upstream.callsFor("judge")) != 1 {
		t.Fatal("the judge was not called, want the two resolvable members fused")
	}
}
