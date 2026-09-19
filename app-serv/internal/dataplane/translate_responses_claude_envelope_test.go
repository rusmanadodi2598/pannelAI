// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_responses_claude_envelope_test.go
// @for       Table-driven tests for the Anthropic envelope a Responses answer becomes.
// @uses      testing.
// @reason    SPEC-API-001 §10 makes the Responses API a P3 deliverable, and an
//
//	Anthropic client reads an envelope whose counts differ from
//	OpenAI's: its input count excludes what OpenAI's includes, its id
//	must never be empty, and a malformed body must fail rather than
//	yield a half-built message.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import "testing"

// TestResponsesToClaudeResponse_Usage pins the count rule in the Anthropic
// direction: OpenAI's prompt count already includes the cached tokens, while
// Anthropic's input count excludes them, so the cached split is subtracted and
// reported separately. Reporting it unsubtracted would overstate what the client
// is billed for.
func TestResponsesToClaudeResponse_Usage(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantInput  int
		wantOutput int
		wantCached int
		wantNone   bool
	}{
		{
			name:       "a plain usage block is carried through",
			body:       `{"status":"completed","output":[],"usage":{"input_tokens":10,"output_tokens":4}}`,
			wantInput:  10,
			wantOutput: 4,
		},
		{
			name:       "cached tokens are subtracted from the input count",
			body:       `{"status":"completed","output":[],"usage":{"input_tokens":10,"output_tokens":4,"input_tokens_details":{"cached_tokens":6}}}`,
			wantInput:  4,
			wantOutput: 4,
			wantCached: 6,
		},
		{
			name:     "a body with no usage leaves the envelope's usage zeroed",
			body:     `{"status":"completed","output":[]}`,
			wantNone: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResponsesToClaudeResponse([]byte(tc.body), "m")
			if err != nil {
				t.Fatalf("translating the answer: %v", err)
			}
			if tc.wantNone {
				if got.Usage.InputTokens != 0 || got.Usage.OutputTokens != 0 {
					t.Fatalf("usage = %+v, want zeroed", got.Usage)
				}
				return
			}
			if got.Usage.InputTokens != tc.wantInput {
				t.Fatalf("input tokens = %d, want %d", got.Usage.InputTokens, tc.wantInput)
			}
			if got.Usage.OutputTokens != tc.wantOutput {
				t.Fatalf("output tokens = %d, want %d", got.Usage.OutputTokens, tc.wantOutput)
			}
			if got.Usage.CacheReadInputTokens != tc.wantCached {
				t.Fatalf("cache read = %d, want %d", got.Usage.CacheReadInputTokens, tc.wantCached)
			}
		})
	}
}

// TestResponsesToClaudeResponse_IDFallback pins the identifier rule: the
// upstream's id is kept when it has one, and a deterministic prefix is used when
// it does not, so a client always has something to correlate on.
func TestResponsesToClaudeResponse_IDFallback(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "the upstream id is kept",
			body: `{"id":"resp_abc","status":"completed","output":[]}`,
			want: "resp_abc",
		},
		{
			name: "a body with no id falls back to the gateway prefix",
			body: `{"status":"completed","output":[]}`,
			want: "msg_pannelai",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResponsesToClaudeResponse([]byte(tc.body), "m")
			if err != nil {
				t.Fatalf("translating the answer: %v", err)
			}
			if got.ID != tc.want {
				t.Fatalf("id = %q, want %q", got.ID, tc.want)
			}
		})
	}
}

// TestResponsesToClaudeResponse_UnreadableBody pins that a malformed upstream
// body is reported as an error rather than yielding a half-built envelope.
func TestResponsesToClaudeResponse_UnreadableBody(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"an empty body", ``},
		{"a bare array", `[1,2,3]`},
		{"a bare number", `7`},
		{"malformed json", `{"output":`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ResponsesToClaudeResponse([]byte(tc.body), "m"); err == nil {
				t.Fatal("want an error for an unreadable body, got nil")
			}
		})
	}
}
