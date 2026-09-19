// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_responses_response_test.go
// @for       Table-driven tests for Responses-to-client answer translation.
// @uses      testing, internal/schema.
// @reason    SPEC-API-001 §10 makes the Responses API a P3 deliverable, and an
//
//	answer read from it has to become either client wire. The rules
//	that decide text, calls, finish reason, and accounting are what a
//	client sees, so each is pinned here against a decoded body.
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

// TestResponsesToOpenAIResponse_TextAndCalls pins what an OpenAI client receives.
func TestResponsesToOpenAIResponse_TextAndCalls(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantText   string
		wantCalls  int
		wantFinish string
	}{
		{
			name:       "a message item's output text becomes the content",
			body:       `{"id":"resp_1","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}]}`,
			wantText:   "hello",
			wantFinish: FinishStop,
		},
		{
			name:       "several output text parts are joined in order",
			body:       `{"id":"resp_1","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"a"},{"type":"output_text","text":"b"}]}]}`,
			wantText:   "a\nb",
			wantFinish: FinishStop,
		},
		{
			name:       "a function call becomes a tool call and finishes as one",
			body:       `{"id":"resp_1","status":"completed","output":[{"type":"function_call","call_id":"call_9","name":"lookup","arguments":"{\"q\":1}"}]}`,
			wantCalls:  1,
			wantFinish: FinishToolCalls,
		},
		{
			name:       "text and a call arrive together",
			body:       `{"id":"resp_1","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"thinking"}]},{"type":"function_call","call_id":"call_9","name":"lookup","arguments":"{}"}]}`,
			wantText:   "thinking",
			wantCalls:  1,
			wantFinish: FinishToolCalls,
		},
		{
			name:       "an answer cut by the output ceiling reports length",
			body:       `{"id":"resp_1","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[{"type":"message","content":[{"type":"output_text","text":"half"}]}]}`,
			wantText:   "half",
			wantFinish: FinishLength,
		},
		{
			name:       "an incomplete answer for another reason still reports stop",
			body:       `{"id":"resp_1","status":"incomplete","incomplete_details":{"reason":"content_filter"},"output":[]}`,
			wantFinish: FinishStop,
		},
		{
			name:       "an empty output array yields empty content and a stop",
			body:       `{"id":"resp_1","status":"completed","output":[]}`,
			wantFinish: FinishStop,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResponsesToOpenAIResponse([]byte(tc.body), "m", 1700000000)
			if err != nil {
				t.Fatalf("translating the answer: %v", err)
			}
			if len(got.Choices) != 1 {
				t.Fatalf("choices = %d, want exactly one", len(got.Choices))
			}
			choice := got.Choices[0]
			if choice.Message.Content.Text != tc.wantText {
				t.Fatalf("content = %q, want %q", choice.Message.Content.Text, tc.wantText)
			}
			if len(choice.Message.ToolCalls) != tc.wantCalls {
				t.Fatalf("tool calls = %d, want %d", len(choice.Message.ToolCalls), tc.wantCalls)
			}
			if choice.FinishReason != tc.wantFinish {
				t.Fatalf("finish reason = %q, want %q", choice.FinishReason, tc.wantFinish)
			}
			if choice.Message.Role != RoleAssistant {
				t.Fatalf("role = %q, want assistant", choice.Message.Role)
			}
			if got.Model != "m" {
				t.Fatalf("model = %q, want the resolved id", got.Model)
			}
			if got.Created != 1700000000 {
				t.Fatalf("created = %d, want the value the caller passed", got.Created)
			}
		})
	}
}

// TestResponsesToOpenAIResponse_CallShape pins the tool-call fields a client
// dispatches on: the call_id becomes the id, and an empty argument string
// becomes an empty object so a client parsing it does not fail.
func TestResponsesToOpenAIResponse_CallShape(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantID   string
		wantName string
		wantArgs string
	}{
		{
			name:     "a complete call is mapped field for field",
			body:     `{"status":"completed","output":[{"type":"function_call","call_id":"call_9","name":"lookup","arguments":"{\"q\":1}"}]}`,
			wantID:   "call_9",
			wantName: "lookup",
			wantArgs: `{"q":1}`,
		},
		{
			name:     "an empty argument string defaults to an empty object",
			body:     `{"status":"completed","output":[{"type":"function_call","call_id":"call_9","name":"lookup","arguments":""}]}`,
			wantID:   "call_9",
			wantName: "lookup",
			wantArgs: "{}",
		},
		{
			name:     "a call with no id keeps an empty one rather than inventing it",
			body:     `{"status":"completed","output":[{"type":"function_call","name":"lookup","arguments":"{}"}]}`,
			wantID:   "",
			wantName: "lookup",
			wantArgs: "{}",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResponsesToOpenAIResponse([]byte(tc.body), "m", 0)
			if err != nil {
				t.Fatalf("translating the answer: %v", err)
			}
			calls := got.Choices[0].Message.ToolCalls
			if len(calls) != 1 {
				t.Fatalf("tool calls = %d, want one (%+v)", len(calls), calls)
			}
			if calls[0].ID != tc.wantID {
				t.Fatalf("id = %q, want %q", calls[0].ID, tc.wantID)
			}
			if calls[0].Function.Name != tc.wantName {
				t.Fatalf("name = %q, want %q", calls[0].Function.Name, tc.wantName)
			}
			if calls[0].Function.Arguments != tc.wantArgs {
				t.Fatalf("arguments = %q, want %q", calls[0].Function.Arguments, tc.wantArgs)
			}
			if calls[0].Type != schema.BlockFunction {
				t.Fatalf("type = %q, want function", calls[0].Type)
			}
		})
	}
}

// TestResponsesToOpenAIResponse_DropsNamelessCall pins that a nameless call is
// dropped rather than surfaced: a client cannot dispatch one, and reporting it
// would look like a call it can make.
func TestResponsesToOpenAIResponse_DropsNamelessCall(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantCalls int
	}{
		{
			name:      "a named call is kept",
			body:      `{"status":"completed","output":[{"type":"function_call","call_id":"c1","name":"lookup","arguments":"{}"}]}`,
			wantCalls: 1,
		},
		{
			name:      "a nameless call is dropped",
			body:      `{"status":"completed","output":[{"type":"function_call","call_id":"c1","name":"","arguments":"{}"}]}`,
			wantCalls: 0,
		},
		{
			name:      "a nameless call among named ones leaves the named ones",
			body:      `{"status":"completed","output":[{"type":"function_call","call_id":"c1","name":"","arguments":"{}"},{"type":"function_call","call_id":"c2","name":"lookup","arguments":"{}"}]}`,
			wantCalls: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResponsesToOpenAIResponse([]byte(tc.body), "m", 0)
			if err != nil {
				t.Fatalf("translating the answer: %v", err)
			}
			if calls := got.Choices[0].Message.ToolCalls; len(calls) != tc.wantCalls {
				t.Fatalf("tool calls = %d, want %d (%+v)", len(calls), tc.wantCalls, calls)
			}
		})
	}
}

// TestResponsesToOpenAIResponse_UnreadableBody pins that an unreadable upstream
// body is an upstream failure, not a client error or a panic: the client's own
// request was already accepted.
func TestResponsesToOpenAIResponse_UnreadableBody(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"an empty body", ``},
		{"a bare array", `[1,2,3]`},
		{"a bare string", `"nope"`},
		{"malformed json", `{"output":`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ResponsesToOpenAIResponse([]byte(tc.body), "m", 0); err == nil {
				t.Fatal("want an error for an unreadable body, got nil")
			}
		})
	}
}
