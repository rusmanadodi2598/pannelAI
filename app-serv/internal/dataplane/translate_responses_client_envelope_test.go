// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_responses_client_envelope_test.go
// @for       Table-driven tests for the envelope and accounting of the Responses
//
//	answer a client receives.
//
// @uses      testing, internal/schema.
// @reason    SPEC-API-001 §7.15 serves POST /api/v1/responses, and the id, status,
//
//	and token counts are what a client logs and bills against. Keeping
//	them apart from the output shape holds both files inside the
//	AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestResponsesAnswerFromCompletion_Identity pins the envelope a client reads:
// the id under the Responses prefix, the resolved model, and the creation
// instant the engine measured.
func TestResponsesAnswerFromCompletion_Identity(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		wantID      string
		wantMessage string
	}{
		{
			name:   "an upstream id gains the Responses prefix",
			body:   `{"id":"chatcmpl-abc","choices":[]}`,
			wantID: "resp_chatcmpl-abc",
		},
		{
			name:   "an id already under the prefix is kept whole",
			body:   `{"id":"resp_abc","choices":[]}`,
			wantID: "resp_abc",
		},
		{
			name:   "a completion with no id falls back to the gateway prefix",
			body:   `{"choices":[]}`,
			wantID: "resp_pannelai",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			answer := ResponsesAnswerFromCompletion(completionFrom(t, tc.body), "resolved-model", 1700000000)
			if answer.ID != tc.wantID {
				t.Fatalf("id = %q, want %q", answer.ID, tc.wantID)
			}
			if answer.Object != schema.ResponsesObjectResponse {
				t.Fatalf("object = %q, want response", answer.Object)
			}
			if answer.Status != schema.ResponsesStatusCompleted {
				t.Fatalf("status = %q, want completed", answer.Status)
			}
			if answer.Model != "resolved-model" {
				t.Fatalf("model = %q, want the resolved id", answer.Model)
			}
			if answer.CreatedAt != 1700000000 {
				t.Fatalf("created_at = %d, want the value the caller passed", answer.CreatedAt)
			}
		})
	}
}

// TestResponsesAnswerFromCompletion_Usage pins the accounting rule: the prompt
// count already includes the cached tokens, so the cached split is a detail
// rather than an addition, and a completion that reported nothing yields no
// usage block rather than a fabricated zero.
func TestResponsesAnswerFromCompletion_Usage(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		wantInput   int
		wantOutput  int
		wantTotal   int
		wantCached  int
		wantReason  int
		wantNoUsage bool
	}{
		{
			name:       "a plain usage block is carried through",
			body:       `{"id":"c1","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":4,"total_tokens":14}}`,
			wantInput:  10,
			wantOutput: 4,
			wantTotal:  14,
		},
		{
			name:       "cached tokens are a detail and are not added",
			body:       `{"id":"c1","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":4,"total_tokens":14,"prompt_tokens_details":{"cached_tokens":6}}}`,
			wantInput:  10,
			wantOutput: 4,
			wantTotal:  14,
			wantCached: 6,
		},
		{
			name:       "reasoning tokens are an output detail",
			body:       `{"id":"c1","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":4,"total_tokens":14,"completion_tokens_details":{"reasoning_tokens":3}}}`,
			wantInput:  10,
			wantOutput: 4,
			wantTotal:  14,
			wantReason: 3,
		},
		{
			name:        "a completion with no usage yields no block",
			body:        `{"id":"c1","choices":[]}`,
			wantNoUsage: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			answer := ResponsesAnswerFromCompletion(completionFrom(t, tc.body), "m", 0)
			if tc.wantNoUsage {
				if answer.Usage != nil {
					t.Fatalf("usage = %+v, want nil so nothing is fabricated", answer.Usage)
				}
				return
			}
			if answer.Usage == nil {
				t.Fatal("usage = nil, want the folded block")
			}
			if answer.Usage.InputTokens != tc.wantInput || answer.Usage.OutputTokens != tc.wantOutput {
				t.Fatalf("usage = %d/%d, want %d/%d", answer.Usage.InputTokens, answer.Usage.OutputTokens, tc.wantInput, tc.wantOutput)
			}
			if answer.Usage.TotalTokens != tc.wantTotal {
				t.Fatalf("total = %d, want %d", answer.Usage.TotalTokens, tc.wantTotal)
			}
			cached := 0
			if answer.Usage.InputDetails != nil {
				cached = answer.Usage.InputDetails.CachedTokens
			}
			if cached != tc.wantCached {
				t.Fatalf("cached = %d, want %d", cached, tc.wantCached)
			}
			reasoning := 0
			if answer.Usage.OutDetails != nil {
				reasoning = answer.Usage.OutDetails.ReasoningTokens
			}
			if reasoning != tc.wantReason {
				t.Fatalf("reasoning = %d, want %d", reasoning, tc.wantReason)
			}
		})
	}
}

// TestOpenAIToResponsesAnswer_UnreadableBody pins that a malformed upstream body
// is an upstream failure rather than a half-built answer.
func TestOpenAIToResponsesAnswer_UnreadableBody(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"an empty body", ``},
		{"a bare array", `[1,2,3]`},
		{"malformed json", `{"choices":`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := OpenAIToResponsesAnswer([]byte(tc.body), "m", 0); err == nil {
				t.Fatal("want an error for an unreadable body, got nil")
			}
		})
	}
}
