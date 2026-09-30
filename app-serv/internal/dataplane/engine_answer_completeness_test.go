// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_answer_completeness_test.go
// @for       The rule that separates an upstream answer cut short by its ceiling
//
//	from one that simply had little to say.
//
// @uses      testing, internal/schema (through the answer under test).
// @reason    A reasoning model that spends the whole output ceiling thinking
//
//	answers 200 with an empty body (measured live on
//	muse-spark-1.3-contributor-free, 2026-09-28), and the combo walk only
//	leaves a member on an error. Pinning which empty answers count — and
//	which are still an answer — is what keeps the walk from failing over a
//	reply the client asked for.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-29
package dataplane

import "testing"

func TestEmptyTruncatedAnswer(t *testing.T) {
	cases := []struct {
		name   string
		target string
		body   string
		want   bool
	}{
		{
			name:   "an openai answer with nothing in it and a ceiling stop",
			target: TargetOpenAI,
			body:   `{"id":"c1","choices":[{"index":0,"message":{"role":"assistant","content":""},"finish_reason":"length"}]}`,
			want:   true,
		},
		{
			name:   "a null content and a ceiling stop",
			target: TargetOpenAI,
			body:   `{"id":"c1","choices":[{"index":0,"message":{"role":"assistant","content":null},"finish_reason":"length"}]}`,
			want:   true,
		},
		{
			name:   "whitespace is not an answer",
			target: TargetOpenAI,
			body:   `{"id":"c1","choices":[{"index":0,"message":{"role":"assistant","content":"   "},"finish_reason":"length"}]}`,
			want:   true,
		},
		{
			name:   "a parts array with no text",
			target: TargetOpenAI,
			body:   `{"id":"c1","choices":[{"index":0,"message":{"role":"assistant","content":[{"type":"text","text":""}]},"finish_reason":"length"}]}`,
			want:   true,
		},
		{
			name:   "the same empty content stopped for another reason",
			target: TargetOpenAI,
			body:   `{"id":"c1","choices":[{"index":0,"message":{"role":"assistant","content":""},"finish_reason":"stop"}]}`,
			want:   false,
		},
		{
			name:   "content cut short mid-sentence is kept",
			target: TargetOpenAI,
			body:   `{"id":"c1","choices":[{"index":0,"message":{"role":"assistant","content":"PO"},"finish_reason":"length"}]}`,
			want:   false,
		},
		{
			name:   "a text part is content",
			target: TargetOpenAI,
			body:   `{"id":"c1","choices":[{"index":0,"message":{"role":"assistant","content":[{"type":"text","text":"PONG"}]},"finish_reason":"length"}]}`,
			want:   false,
		},
		{
			name:   "a tool call is an answer even at the ceiling",
			target: TargetOpenAI,
			body:   `{"id":"c1","choices":[{"index":0,"message":{"role":"assistant","content":"","tool_calls":[{"id":"c","type":"function","function":{"name":"lookup","arguments":"{}"}}]},"finish_reason":"length"}]}`,
			want:   false,
		},
		{
			name:   "reasoning with no content is still what the model said",
			target: TargetOpenAI,
			body:   `{"id":"c1","choices":[{"index":0,"message":{"role":"assistant","content":"","reasoning_content":"thinking hard"},"finish_reason":"length"}]}`,
			want:   false,
		},
		{
			name:   "a completion with no choices",
			target: TargetOpenAI,
			body:   `{"id":"c1","choices":[]}`,
			want:   false,
		},
		{
			name:   "an unreadable body",
			target: TargetOpenAI,
			body:   `not json`,
			want:   false,
		},
		{
			name:   "a claude answer cut short by max_tokens",
			target: TargetClaude,
			body:   `{"id":"msg_1","type":"message","role":"assistant","content":[],"stop_reason":"max_tokens","usage":{"input_tokens":9,"output_tokens":9}}`,
			want:   true,
		},
		{
			name:   "a claude text block is content",
			target: TargetClaude,
			body:   `{"id":"msg_2","type":"message","role":"assistant","content":[{"type":"text","text":"PONG"}],"stop_reason":"max_tokens"}`,
			want:   false,
		},
		{
			name:   "a claude thinking block is an answer",
			target: TargetClaude,
			body:   `{"id":"msg_3","type":"message","role":"assistant","content":[{"type":"thinking","thinking":"hmm"}],"stop_reason":"max_tokens"}`,
			want:   false,
		},
		{
			name:   "a claude tool_use block is an answer",
			target: TargetClaude,
			body:   `{"id":"msg_4","type":"message","role":"assistant","content":[{"type":"tool_use","id":"t1","name":"lookup","input":{}}],"stop_reason":"tool_use"}`,
			want:   false,
		},
		{
			name:   "a claude end_turn with no content is kept",
			target: TargetClaude,
			body:   `{"id":"msg_5","type":"message","role":"assistant","content":[],"stop_reason":"end_turn"}`,
			want:   false,
		},
		{
			name:   "a responses answer incomplete at the ceiling",
			target: TargetResponses,
			body:   `{"id":"resp_1","object":"response","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[]}`,
			want:   true,
		},
		{
			name:   "a responses message item with empty text",
			target: TargetResponses,
			body:   `{"id":"resp_2","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":""}]}]}`,
			want:   true,
		},
		{
			name:   "a responses reasoning item is an answer",
			target: TargetResponses,
			body:   `{"id":"resp_3","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[{"type":"reasoning","summary":[{"text":"because"}]}]}`,
			want:   false,
		},
		{
			name:   "a responses function call is an answer",
			target: TargetResponses,
			body:   `{"id":"resp_4","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[{"type":"function_call","call_id":"c1","name":"lookup","arguments":"{}"}]}`,
			want:   false,
		},
		{
			name:   "a responses answer completed with no output",
			target: TargetResponses,
			body:   `{"id":"resp_5","status":"completed","output":[]}`,
			want:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolution := Resolution{Target: tc.target, ModelID: "muse-spark-1.3-contributor-free"}
			if got := emptyTruncatedAnswer([]byte(tc.body), resolution); got != tc.want {
				t.Fatalf("emptyTruncatedAnswer() = %v, want %v for %s body %s", got, tc.want, tc.target, tc.body)
			}
		})
	}
}
