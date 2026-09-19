// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_responses_client_request_test.go
// @for       Table-driven tests for Responses-to-chat request translation.
// @uses      testing, internal/schema.
// @reason    SPEC-API-001 §7.15 serves POST /api/v1/responses, so a client on
//
//	that wire has to reach every resolved provider. The instruction
//	field and the input union are what the client's request means, and
//	both are pinned here against a decoded body (AGENTS.md §2.1).
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

// responsesRequest decodes a Responses body the way the handler does, so a test
// drives the same decode boundary a request does.
func responsesRequest(t *testing.T, body string) schema.ResponsesRequest {
	t.Helper()
	req, err := schema.DecodeResponsesRequest([]byte(body))
	if err != nil {
		t.Fatalf("decoding responses request: %v", err)
	}
	return req
}

// roleSequence renders a message list's roles compactly for a table assertion.
func roleSequence(messages []schema.ChatMessage) string {
	if len(messages) == 0 {
		return ""
	}
	out := messages[0].Role
	for _, message := range messages[1:] {
		out += "|" + message.Role
	}
	return out
}

// TestResponsesToOpenAI_Instructions pins where the instruction field lands.
//
// The Responses API has one `instructions` field while the chat wire has a
// system role, so the instruction becomes the leading system message. Dropping
// it would lose what the client told the model, which is the whole point of the
// field.
func TestResponsesToOpenAI_Instructions(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantSeq  string
		wantText string
	}{
		{
			name:     "an instruction becomes the leading system message",
			body:     `{"model":"m","instructions":"be terse","input":"hi"}`,
			wantSeq:  "system|user",
			wantText: "be terse",
		},
		{
			name:    "a request with no instruction starts with the user turn",
			body:    `{"model":"m","input":"hi"}`,
			wantSeq: "user",
		},
		{
			name:    "an empty instruction adds no message",
			body:    `{"model":"m","instructions":"","input":"hi"}`,
			wantSeq: "user",
		},
		{
			name:    "an instruction with no input is still carried",
			body:    `{"model":"m","instructions":"be terse","input":[]}`,
			wantSeq: "system",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResponsesToOpenAI(responsesRequest(t, tc.body), "upstream-model", false)
			if seq := roleSequence(got.Messages); seq != tc.wantSeq {
				t.Fatalf("role sequence = %q, want %q (%+v)", seq, tc.wantSeq, got.Messages)
			}
			if tc.wantText == "" {
				return
			}
			if got.Messages[0].Content.Text != tc.wantText {
				t.Fatalf("system text = %q, want %q", got.Messages[0].Content.Text, tc.wantText)
			}
		})
	}
}

// TestResponsesToOpenAI_InputShapes pins the `input` union.
//
// A bare string is the shorthand for one user turn, and an item array is the
// full conversation. An item with no `type` but a `role` is a message: the Droid
// CLI sends them that way, so skipping them would drop the conversation.
func TestResponsesToOpenAI_InputShapes(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantSeq string
		wantMsg int
	}{
		{
			name:    "a bare string becomes one user turn",
			body:    `{"model":"m","input":"hello"}`,
			wantSeq: "user",
			wantMsg: 1,
		},
		{
			name:    "an empty string produces no message",
			body:    `{"model":"m","input":""}`,
			wantSeq: "",
		},
		{
			name:    "an absent input produces no message",
			body:    `{"model":"m"}`,
			wantSeq: "",
		},
		{
			name:    "an item array carries its turns in order",
			body:    `{"model":"m","input":[{"type":"message","role":"user","content":"q"},{"type":"message","role":"assistant","content":"a"}]}`,
			wantSeq: "user|assistant",
			wantMsg: 2,
		},
		{
			name:    "a role-only item is read as a message",
			body:    `{"model":"m","input":[{"role":"user","content":"q"}]}`,
			wantSeq: "user",
			wantMsg: 1,
		},
		{
			name:    "an item with neither type nor role is skipped",
			body:    `{"model":"m","input":[{"content":"orphan"},{"type":"message","role":"user","content":"q"}]}`,
			wantSeq: "user",
			wantMsg: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResponsesToOpenAI(responsesRequest(t, tc.body), "m", false)
			if seq := roleSequence(got.Messages); seq != tc.wantSeq {
				t.Fatalf("role sequence = %q, want %q (%+v)", seq, tc.wantSeq, got.Messages)
			}
			if len(got.Messages) != tc.wantMsg {
				t.Fatalf("messages = %d, want %d", len(got.Messages), tc.wantMsg)
			}
		})
	}
}

// TestResponsesToOpenAI_MessageContent pins the content union of a message item.
func TestResponsesToOpenAI_MessageContent(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantText string
	}{
		{
			name:     "a bare-string content is carried as text",
			body:     `{"model":"m","input":[{"type":"message","role":"user","content":"hello"}]}`,
			wantText: "hello",
		},
		{
			name:     "input text parts are joined",
			body:     `{"model":"m","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"a"},{"type":"input_text","text":"b"}]}]}`,
			wantText: "a\nb",
		},
		{
			name:     "output text parts are read the same way",
			body:     `{"model":"m","input":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"said"}]}]}`,
			wantText: "said",
		},
		{
			name:     "a message with no content carries no text",
			body:     `{"model":"m","input":[{"type":"message","role":"user"}]}`,
			wantText: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResponsesToOpenAI(responsesRequest(t, tc.body), "m", false)
			if len(got.Messages) != 1 {
				t.Fatalf("messages = %d, want one", len(got.Messages))
			}
			if text := got.Messages[0].Content.TextContent(); text != tc.wantText {
				t.Fatalf("content = %q, want %q", text, tc.wantText)
			}
		})
	}
}
