// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_responses_client_reasoning_test.go
// @for       Table-driven tests for where a reasoning item's text attaches.
// @uses      testing.
// @reason    SPEC-API-001 §7.15 serves POST /api/v1/responses, whose reasoning
//
//	travels as an item of its own while the chat wire carries it as a
//	member of the assistant turn. Attaching it to the wrong turn either
//	loses what the model was thinking or tells the upstream something
//	the client did not.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import "testing"

// TestResponsesToOpenAI_ReasoningAttachment pins where a reasoning item's text
// lands: on the assistant turn that follows it, which is where the chat wire
// carries it. Reasoning that nothing follows is dropped, because there is no
// turn to attach it to.
func TestResponsesToOpenAI_ReasoningAttachment(t *testing.T) {
	cases := []struct {
		name          string
		body          string
		wantSeq       string
		wantReasoning string
		wantIndex     int
	}{
		{
			name:          "reasoning before an assistant message attaches to it",
			body:          `{"model":"m","input":[{"type":"reasoning","summary":[{"type":"summary_text","text":"why"}]},{"type":"message","role":"assistant","content":"a"}]}`,
			wantSeq:       "assistant",
			wantReasoning: "why",
			wantIndex:     0,
		},
		{
			name:          "reasoning before a call attaches to the assistant turn",
			body:          `{"model":"m","input":[{"type":"reasoning","summary":[{"type":"summary_text","text":"why"}]},{"type":"function_call","call_id":"c1","name":"lookup","arguments":"{}"}]}`,
			wantSeq:       "assistant",
			wantReasoning: "why",
			wantIndex:     0,
		},
		{
			name:          "an empty summary falls back to the content parts",
			body:          `{"model":"m","input":[{"type":"reasoning","summary":[],"content":[{"type":"reasoning_text","text":"fallback"}]},{"type":"message","role":"assistant","content":"a"}]}`,
			wantReasoning: "fallback",
			wantSeq:       "assistant",
			wantIndex:     0,
		},
		{
			name:          "several reasoning items are joined in order",
			body:          `{"model":"m","input":[{"type":"reasoning","summary":[{"type":"summary_text","text":"one"}]},{"type":"reasoning","summary":[{"type":"summary_text","text":"two"}]},{"type":"message","role":"assistant","content":"a"}]}`,
			wantReasoning: "one\ntwo",
			wantSeq:       "assistant",
			wantIndex:     0,
		},
		{
			name:          "reasoning does not attach to a user turn",
			body:          `{"model":"m","input":[{"type":"reasoning","summary":[{"type":"summary_text","text":"why"}]},{"type":"message","role":"user","content":"q"}]}`,
			wantReasoning: "",
			wantSeq:       "user",
			wantIndex:     0,
		},
		{
			name:          "reasoning with nothing after it is dropped",
			body:          `{"model":"m","input":[{"type":"reasoning","summary":[{"type":"summary_text","text":"why"}]}]}`,
			wantReasoning: "",
			wantSeq:       "",
			wantIndex:     0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResponsesToOpenAI(responsesRequest(t, tc.body), "m", false)
			if seq := roleSequence(got.Messages); seq != tc.wantSeq {
				t.Fatalf("role sequence = %q, want %q (%+v)", seq, tc.wantSeq, got.Messages)
			}
			if len(got.Messages) == 0 {
				return
			}
			if got.Messages[tc.wantIndex].Reasoning != tc.wantReasoning {
				t.Fatalf("reasoning = %q, want %q", got.Messages[tc.wantIndex].Reasoning, tc.wantReasoning)
			}
		})
	}
}
