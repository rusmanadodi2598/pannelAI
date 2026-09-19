// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_responses_client_items_test.go
// @for       Table-driven tests for call, result, and reasoning item mapping.
// @uses      testing, internal/schema.
// @reason    SPEC-API-001 §7.15 serves POST /api/v1/responses, whose item array
//
//	assembles one assistant turn from several items and carries tool
//	results and reasoning as items of their own. Getting that wrong
//	breaks every tool-using client on the wire, so each rule is pinned
//	here against a decoded body (AGENTS.md §2.1).
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

// TestResponsesToOpenAI_CallAssembly pins how call items become an assistant
// turn.
//
// Consecutive calls share one assistant message, because that is the shape the
// chat wire uses for parallel calls, and a message between two calls ends the
// turn. A call with no name is skipped: the API rejects one, and forwarding it
// would fail the whole request (reference #444).
func TestResponsesToOpenAI_CallAssembly(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantSeq   string
		wantCalls int
		wantFirst int
		wantID    string
		wantName  string
		wantArgs  string
	}{
		{
			name:      "one call becomes one assistant turn",
			body:      `{"model":"m","input":[{"type":"message","role":"user","content":"q"},{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{\"q\":1}"}]}`,
			wantSeq:   "user|assistant",
			wantCalls: 1,
			wantFirst: 1,
			wantID:    "call_1",
			wantName:  "lookup",
			wantArgs:  `{"q":1}`,
		},
		{
			name:      "two consecutive calls share one assistant turn",
			body:      `{"model":"m","input":[{"type":"function_call","call_id":"c1","name":"a","arguments":"{}"},{"type":"function_call","call_id":"c2","name":"b","arguments":"{}"}]}`,
			wantSeq:   "assistant",
			wantCalls: 2,
			wantFirst: 2,
			wantID:    "c1",
			wantName:  "a",
			wantArgs:  "{}",
		},
		{
			name:      "a nameless call is dropped rather than sent to be rejected",
			body:      `{"model":"m","input":[{"type":"function_call","call_id":"c1","name":"","arguments":"{}"},{"type":"function_call","call_id":"c2","name":"b","arguments":"{}"}]}`,
			wantSeq:   "assistant",
			wantCalls: 1,
			wantFirst: 1,
			wantID:    "c2",
			wantName:  "b",
			wantArgs:  "{}",
		},
		{
			name:      "a lone nameless call produces no turn at all",
			body:      `{"model":"m","input":[{"type":"function_call","call_id":"c1","name":"","arguments":"{}"}]}`,
			wantSeq:   "",
			wantCalls: 0,
			wantFirst: 0,
		},
		{
			name:      "empty arguments default to an empty object",
			body:      `{"model":"m","input":[{"type":"function_call","call_id":"c1","name":"lookup"}]}`,
			wantSeq:   "assistant",
			wantCalls: 1,
			wantFirst: 1,
			wantID:    "c1",
			wantName:  "lookup",
			wantArgs:  "{}",
		},
		{
			name:      "a user turn between calls ends the assistant turn",
			body:      `{"model":"m","input":[{"type":"function_call","call_id":"c1","name":"a","arguments":"{}"},{"type":"message","role":"user","content":"q"},{"type":"function_call","call_id":"c2","name":"b","arguments":"{}"}]}`,
			wantSeq:   "assistant|user|assistant",
			wantCalls: 2,
			wantFirst: 1,
			wantID:    "c1",
			wantName:  "a",
			wantArgs:  "{}",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResponsesToOpenAI(responsesRequest(t, tc.body), "m", false)
			if seq := roleSequence(got.Messages); seq != tc.wantSeq {
				t.Fatalf("role sequence = %q, want %q (%+v)", seq, tc.wantSeq, got.Messages)
			}
			calls := collectCalls(got.Messages)
			if len(calls) != tc.wantCalls {
				t.Fatalf("tool calls = %d, want %d (%+v)", len(calls), tc.wantCalls, calls)
			}
			if first := firstTurnCalls(got.Messages); first != tc.wantFirst {
				t.Fatalf("calls on the first assistant turn = %d, want %d", first, tc.wantFirst)
			}
			if tc.wantCalls == 0 {
				return
			}
			if calls[0].ID != tc.wantID || calls[0].Function.Name != tc.wantName {
				t.Fatalf("first call = %+v, want id %q name %q", calls[0], tc.wantID, tc.wantName)
			}
			if calls[0].Function.Arguments != tc.wantArgs {
				t.Fatalf("arguments = %q, want %q", calls[0].Function.Arguments, tc.wantArgs)
			}
			if calls[0].Type != schema.BlockFunction {
				t.Fatalf("call type = %q, want function", calls[0].Type)
			}
		})
	}
}

// collectCalls gathers every tool call in a message list, in order.
func collectCalls(messages []schema.ChatMessage) []schema.ToolCall {
	out := make([]schema.ToolCall, 0, len(messages))
	for _, message := range messages {
		out = append(out, message.ToolCalls...)
	}
	return out
}

// firstTurnCalls counts the calls on the first assistant turn, which is what
// distinguishes one turn carrying two calls from two turns carrying one each.
func firstTurnCalls(messages []schema.ChatMessage) int {
	for _, message := range messages {
		if message.Role == schema.RoleAssistant {
			return len(message.ToolCalls)
		}
	}
	return 0
}

// TestResponsesToOpenAI_ToolResults pins how a result item becomes a tool
// message. The chat wire requires a string, so a JSON string is unwrapped and
// anything else keeps its JSON text rather than being dropped.
func TestResponsesToOpenAI_ToolResults(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantText string
	}{
		{
			name:     "a string result is unwrapped",
			body:     `{"model":"m","input":[{"type":"function_call_output","call_id":"c1","output":"42"}]}`,
			wantText: "42",
		},
		{
			name:     "an object result keeps its json text",
			body:     `{"model":"m","input":[{"type":"function_call_output","call_id":"c1","output":{"ok":true}}]}`,
			wantText: `{"ok":true}`,
		},
		{
			name:     "an absent result is an empty string",
			body:     `{"model":"m","input":[{"type":"function_call_output","call_id":"c1"}]}`,
			wantText: "",
		},
		{
			name:     "a result with an escaped payload survives the round trip",
			body:     `{"model":"m","input":[{"type":"function_call_output","call_id":"c1","output":"{\"rows\":[1,2]}"}]}`,
			wantText: `{"rows":[1,2]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResponsesToOpenAI(responsesRequest(t, tc.body), "m", false)
			if len(got.Messages) != 1 || got.Messages[0].Role != schema.RoleTool {
				t.Fatalf("want one tool message, got %+v", got.Messages)
			}
			message := got.Messages[0]
			if message.ToolCallID != "c1" {
				t.Fatalf("tool_call_id = %q, want the call it answers", message.ToolCallID)
			}
			if message.Content.TextContent() != tc.wantText {
				t.Fatalf("content = %q, want %q", message.Content.TextContent(), tc.wantText)
			}
		})
	}
}
