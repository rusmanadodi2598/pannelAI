// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_responses_request_test.go
// @for       Table-driven tests for chat-to-Responses turn translation.
// @uses      testing, internal/schema.
// @reason    SPEC-API-001 §10 makes the Responses API a P3 deliverable, and the
//
//	rules that decide what an upstream sees — one instruction field,
//	one item per turn, input text split from output text — are what a
//	client on either wire depends on. Translation is a pure function,
//	so these run with no network and no clock (AGENTS.md §2.1).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// itemTypes renders an item sequence compactly, so a test can state the shape it
// expects in one readable string.
func itemTypes(items []ResponsesItem) string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Type)
	}
	return strings.Join(out, "|")
}

// TestOpenAIToResponses_Instructions pins the one-instruction rule.
//
// The Responses API has a single `instructions` field, so the reference keeps
// the first system turn and drops the rest rather than joining them: joining
// would tell the model something the client never asked for.
func TestOpenAIToResponses_Instructions(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "a lone system message becomes the instructions",
			body: `{"model":"m","messages":[{"role":"system","content":"be terse"},{"role":"user","content":"hi"}]}`,
			want: "be terse",
		},
		{
			name: "a developer message counts as a system turn",
			body: `{"model":"m","messages":[{"role":"developer","content":"be exact"},{"role":"user","content":"hi"}]}`,
			want: "be exact",
		},
		{
			name: "only the first of several system turns is kept",
			body: `{"model":"m","messages":[{"role":"system","content":"first"},{"role":"user","content":"q"},{"role":"system","content":"second"}]}`,
			want: "first",
		},
		{
			name: "array content is flattened to its text parts",
			body: `{"model":"m","messages":[{"role":"system","content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]},{"role":"user","content":"q"}]}`,
			want: "a\nb",
		},
		{
			name: "a request with no system turn leaves the field empty",
			body: `{"model":"m","messages":[{"role":"user","content":"q"}]}`,
			want: "",
		},
		{
			name: "a request with no messages at all is accepted",
			body: `{"model":"m","messages":[]}`,
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := OpenAIToResponses(chatRequest(t, tc.body), "upstream-model", false)
			if got.Instructions != tc.want {
				t.Fatalf("Instructions = %q, want %q", got.Instructions, tc.want)
			}
			if got.Model != "upstream-model" {
				t.Fatalf("Model = %q, want the resolved upstream id", got.Model)
			}
			if got.Store {
				t.Fatal("Store = true, want false: the gateway does not ask an upstream to retain the answer")
			}
		})
	}
}

// TestOpenAIToResponses_ItemSequence pins which items a message list becomes.
//
// Every turn has to land in the input array in order, and a turn with nothing to
// say must not become an empty item: the API rejects one, so a translation that
// emits it fails the whole request.
func TestOpenAIToResponses_ItemSequence(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "user and assistant turns become message items",
			body: `{"model":"m","messages":[{"role":"user","content":"q"},{"role":"assistant","content":"a"}]}`,
			want: "message|message",
		},
		{
			name: "a tool result becomes its own output item",
			body: `{"model":"m","messages":[{"role":"user","content":"q"},{"role":"assistant","content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},{"role":"tool","tool_call_id":"c1","content":"42"}]}`,
			want: "message|function_call|function_call_output",
		},
		{
			name: "an assistant turn with only calls emits no message item",
			body: `{"model":"m","messages":[{"role":"user","content":"q"},{"role":"assistant","content":null,"tool_calls":[{"id":"c1","type":"function","function":{"name":"lookup","arguments":"{}"}}]}]}`,
			want: "message|function_call",
		},
		{
			name: "an empty user turn is skipped rather than sent as an empty item",
			body: `{"model":"m","messages":[{"role":"user","content":""},{"role":"user","content":"q"}]}`,
			want: "message",
		},
		{
			name: "an empty messages array yields no items",
			body: `{"model":"m","messages":[]}`,
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := OpenAIToResponses(chatRequest(t, tc.body), "m", false)
			if seq := itemTypes(got.Input); seq != tc.want {
				t.Fatalf("item sequence = %q, want %q (%+v)", seq, tc.want, got.Input)
			}
		})
	}
}

// TestOpenAIToResponses_AssistantTextType pins the input/output text split: the
// Responses API distinguishes who wrote a text part, so an assistant turn must
// not travel as input text.
func TestOpenAIToResponses_AssistantTextType(t *testing.T) {
	cases := []struct {
		name     string
		role     string
		wantType string
	}{
		{"a user turn is input text", "user", ItemInputText},
		{"an assistant turn is output text", "assistant", ItemOutputText},
		{"a system turn is input text", "system", ItemInputText},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"model":"m","messages":[{"role":"` + tc.role + `","content":"x"}]}`
			got := OpenAIToResponses(chatRequest(t, body), "m", false)
			if tc.role == "system" {
				if got.Instructions != "x" {
					t.Fatalf("Instructions = %q, want the system text", got.Instructions)
				}
				return
			}
			if len(got.Input) != 1 || len(got.Input[0].Content) != 1 {
				t.Fatalf("want one item with one part, got %+v", got.Input)
			}
			if partType := got.Input[0].Content[0].Type; partType != tc.wantType {
				t.Fatalf("part type = %q, want %q", partType, tc.wantType)
			}
		})
	}
}

// TestOpenAIToResponses_StreamFlag pins that the caller's stream decision reaches
// the payload unchanged, since the upstream frames its answer differently.
func TestOpenAIToResponses_StreamFlag(t *testing.T) {
	body := `{"model":"m","messages":[{"role":"user","content":"q"}]}`
	for _, stream := range []bool{true, false} {
		got := OpenAIToResponses(chatRequest(t, body), "m", stream)
		if got.Stream != stream {
			t.Fatalf("Stream = %v, want %v", got.Stream, stream)
		}
	}
}

// TestClaudeToResponses_Pivot pins the Anthropic direction, which reaches the
// Responses vocabulary through the OpenAI chat request rather than a second
// direct mapping: one mapping per pair is what keeps the translator set small.
func TestClaudeToResponses_Pivot(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantInstr string
		wantSeq   string
	}{
		{
			name:      "a system prompt becomes the instructions",
			body:      `{"model":"m","max_tokens":64,"system":"be terse","messages":[{"role":"user","content":"q"}]}`,
			wantInstr: "be terse",
			wantSeq:   "message",
		},
		{
			name:      "text blocks become one message item",
			body:      `{"model":"m","max_tokens":64,"messages":[{"role":"user","content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]}]}`,
			wantInstr: "",
			wantSeq:   "message",
		},
		{
			name:      "a tool result becomes a function_call_output item",
			body:      `{"model":"m","max_tokens":64,"messages":[{"role":"user","content":"q"},{"role":"assistant","content":[{"type":"tool_use","id":"tu_1","name":"lookup","input":{"q":1}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_1","content":"42"}]}]}`,
			wantInstr: "",
			wantSeq:   "message|function_call|function_call_output",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := schema.DecodeMessagesRequest([]byte(tc.body))
			if err != nil {
				t.Fatalf("decoding messages request: %v", err)
			}
			got := ClaudeToResponses(req, "upstream-model", false)
			if got.Instructions != tc.wantInstr {
				t.Fatalf("Instructions = %q, want %q", got.Instructions, tc.wantInstr)
			}
			if seq := itemTypes(got.Input); seq != tc.wantSeq {
				t.Fatalf("item sequence = %q, want %q (%+v)", seq, tc.wantSeq, got.Input)
			}
			if got.Model != "upstream-model" {
				t.Fatalf("Model = %q, want the resolved upstream id", got.Model)
			}
		})
	}
}
