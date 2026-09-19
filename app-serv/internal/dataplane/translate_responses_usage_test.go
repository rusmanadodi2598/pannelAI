// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_responses_usage_test.go
// @for       Table-driven tests for the Responses reasoning fold and accounting.
// @uses      testing.
// @reason    SPEC-API-001 §10 makes the Responses API a P3 deliverable, and its
//
//	usage block counts differently from OpenAI's: input_tokens already
//	includes the cached tokens, so folding it by addition is what keeps
//	a client from being billed twice. The reasoning fold sits beside it
//	because both are read from the same decoded answer.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import "testing"

// TestResponsesToOpenAIResponse_Reasoning pins the reasoning summary fold,
// including the fallback: some providers report the text under content rather
// than summary, and dropping it would lose what the model was thinking.
func TestResponsesToOpenAIResponse_Reasoning(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "a summary becomes the reasoning content",
			body: `{"status":"completed","output":[{"type":"reasoning","summary":[{"type":"summary_text","text":"why"}]}]}`,
			want: "why",
		},
		{
			name: "summary parts are joined in order",
			body: `{"status":"completed","output":[{"type":"reasoning","summary":[{"type":"summary_text","text":"a"},{"type":"summary_text","text":"b"}]}]}`,
			want: "a\nb",
		},
		{
			name: "an empty summary falls back to the content parts",
			body: `{"status":"completed","output":[{"type":"reasoning","summary":[],"content":[{"type":"reasoning_text","text":"fallback"}]}]}`,
			want: "fallback",
		},
		{
			name: "several reasoning items are folded in order",
			body: `{"status":"completed","output":[{"type":"reasoning","summary":[{"type":"summary_text","text":"one"}]},{"type":"reasoning","summary":[{"type":"summary_text","text":"two"}]}]}`,
			want: "one\ntwo",
		},
		{
			name: "no reasoning item leaves the field empty",
			body: `{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"x"}]}]}`,
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResponsesToOpenAIResponse([]byte(tc.body), "m", 0)
			if err != nil {
				t.Fatalf("translating the answer: %v", err)
			}
			if reasoning := got.Choices[0].Message.Reasoning; reasoning != tc.want {
				t.Fatalf("reasoning = %q, want %q", reasoning, tc.want)
			}
		})
	}
}

// TestResponsesUsageFromObject pins the accounting rule: input_tokens already
// includes the cached tokens, so the cached split is reported as a detail rather
// than added, and a detail of zero is omitted rather than reported as a split.
func TestResponsesUsageFromObject(t *testing.T) {
	cases := []struct {
		name         string
		body         string
		wantPrompt   int
		wantComplete int
		wantTotal    int
		wantCached   int
		wantReason   int
		wantNoUsage  bool
	}{
		{
			name:         "a plain usage block is folded by addition",
			body:         `{"usage":{"input_tokens":10,"output_tokens":4}}`,
			wantPrompt:   10,
			wantComplete: 4,
			wantTotal:    14,
		},
		{
			name:         "cached tokens are a detail and are not added to the prompt",
			body:         `{"usage":{"input_tokens":10,"output_tokens":4,"input_tokens_details":{"cached_tokens":6}}}`,
			wantPrompt:   10,
			wantComplete: 4,
			wantTotal:    14,
			wantCached:   6,
		},
		{
			name:         "reasoning tokens are a completion detail",
			body:         `{"usage":{"input_tokens":10,"output_tokens":4,"output_tokens_details":{"reasoning_tokens":3}}}`,
			wantPrompt:   10,
			wantComplete: 4,
			wantTotal:    14,
			wantReason:   3,
		},
		{
			name:         "a zero cached split is omitted rather than reported",
			body:         `{"usage":{"input_tokens":10,"output_tokens":4,"input_tokens_details":{"cached_tokens":0}}}`,
			wantPrompt:   10,
			wantComplete: 4,
			wantTotal:    14,
		},
		{
			name:         "an all-zero usage block is still folded",
			body:         `{"usage":{"input_tokens":0,"output_tokens":0}}`,
			wantPrompt:   0,
			wantComplete: 0,
			wantTotal:    0,
		},
		{
			name:         "a large usage block is folded without truncation",
			body:         `{"usage":{"input_tokens":1000000,"output_tokens":250000}}`,
			wantPrompt:   1000000,
			wantComplete: 250000,
			wantTotal:    1250000,
		},
		{
			name:        "a body with no usage reports none rather than a fabricated zero",
			body:        `{"status":"completed","output":[]}`,
			wantNoUsage: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResponsesToOpenAIResponse([]byte(tc.body), "m", 0)
			if err != nil {
				t.Fatalf("translating the answer: %v", err)
			}
			if tc.wantNoUsage {
				if got.Usage != nil {
					t.Fatalf("usage = %+v, want nil so nothing is recorded", got.Usage)
				}
				return
			}
			if got.Usage == nil {
				t.Fatal("usage = nil, want the folded block")
			}
			if got.Usage.PromptTokens != tc.wantPrompt || got.Usage.CompletionTokens != tc.wantComplete {
				t.Fatalf("usage = %d/%d, want %d/%d", got.Usage.PromptTokens, got.Usage.CompletionTokens, tc.wantPrompt, tc.wantComplete)
			}
			if got.Usage.TotalTokens != tc.wantTotal {
				t.Fatalf("total = %d, want %d", got.Usage.TotalTokens, tc.wantTotal)
			}
			cached := 0
			if got.Usage.PromptTokensDetails != nil {
				cached = got.Usage.PromptTokensDetails.CachedTokens
			}
			if cached != tc.wantCached {
				t.Fatalf("cached = %d, want %d", cached, tc.wantCached)
			}
			reasoning := 0
			if got.Usage.CompletionTokensDetail != nil {
				reasoning = got.Usage.CompletionTokensDetail.ReasoningTokens
			}
			if reasoning != tc.wantReason {
				t.Fatalf("reasoning = %d, want %d", reasoning, tc.wantReason)
			}
		})
	}
}
