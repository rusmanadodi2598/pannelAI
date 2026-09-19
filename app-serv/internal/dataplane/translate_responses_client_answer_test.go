// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_responses_client_answer_test.go
// @for       Table-driven tests for building a Responses answer from the pivot
//
//	completion.
//
// @uses      testing, encoding/json, internal/schema.
// @reason    SPEC-API-001 §7.15 serves POST /api/v1/responses, so the answer a
//
//	client receives has to be assembled whenever the provider speaks
//	another format. The output array is what the client acts on, so
//	every item kind and its order are pinned here (AGENTS.md §2.1).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"encoding/json"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// completionFrom decodes an OpenAI completion body the way the translator does.
func completionFrom(t *testing.T, body string) schema.ChatCompletionResponse {
	t.Helper()
	var completion schema.ChatCompletionResponse
	if err := json.Unmarshal([]byte(body), &completion); err != nil {
		t.Fatalf("decoding completion: %v", err)
	}
	return completion
}

// itemKinds renders an answer's output item kinds compactly for a table.
func itemKinds(items []schema.ResponsesOutputItem) string {
	if len(items) == 0 {
		return ""
	}
	out := items[0].Type
	for _, item := range items[1:] {
		out += "|" + item.Type
	}
	return out
}

// TestResponsesAnswerFromCompletion_Output pins the output array: which items an
// answer carries and in what order.
//
// Reasoning comes first because it precedes the answer, the message follows, and
// each call is an item of its own. The reference's own assembler keys items by
// the choice index, so an answer with both reasoning and text loses one of them;
// this builder emits each at its own index instead, which is why the order is
// asserted here rather than left to the reference's behaviour.
func TestResponsesAnswerFromCompletion_Output(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantKind string
	}{
		{
			name:     "text becomes one message item",
			body:     `{"id":"c1","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`,
			wantKind: "message",
		},
		{
			name:     "reasoning precedes the message",
			body:     `{"id":"c1","choices":[{"index":0,"message":{"role":"assistant","content":"answer","reasoning_content":"why"},"finish_reason":"stop"}]}`,
			wantKind: "reasoning|message",
		},
		{
			name:     "a call becomes a function_call item",
			body:     `{"id":"c1","choices":[{"index":0,"message":{"role":"assistant","tool_calls":[{"id":"call_9","type":"function","function":{"name":"lookup","arguments":"{\"q\":1}"}}]},"finish_reason":"tool_calls"}]}`,
			wantKind: "function_call",
		},
		{
			name:     "text and two calls are emitted in order",
			body:     `{"id":"c1","choices":[{"index":0,"message":{"role":"assistant","content":"thinking","tool_calls":[{"id":"c1","type":"function","function":{"name":"a","arguments":"{}"}},{"id":"c2","type":"function","function":{"name":"b","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`,
			wantKind: "message|function_call|function_call",
		},
		{
			name:     "reasoning, text, and a call together",
			body:     `{"id":"c1","choices":[{"index":0,"message":{"role":"assistant","content":"answer","reasoning_content":"why","tool_calls":[{"id":"c9","type":"function","function":{"name":"lookup","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`,
			wantKind: "reasoning|message|function_call",
		},
		{
			name:     "a nameless call is dropped rather than reported",
			body:     `{"id":"c1","choices":[{"index":0,"message":{"role":"assistant","tool_calls":[{"id":"c1","type":"function","function":{"name":"","arguments":"{}"}},{"id":"c2","type":"function","function":{"name":"b","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`,
			wantKind: "function_call",
		},
		{
			name:     "an empty answer carries no item",
			body:     `{"id":"c1","choices":[{"index":0,"message":{"role":"assistant","content":""},"finish_reason":"stop"}]}`,
			wantKind: "",
		},
		{
			name:     "a completion with no choices carries no item",
			body:     `{"id":"c1","choices":[]}`,
			wantKind: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			answer := ResponsesAnswerFromCompletion(completionFrom(t, tc.body), "m", 1700000000)
			if kinds := itemKinds(answer.Output); kinds != tc.wantKind {
				t.Fatalf("item kinds = %q, want %q (%+v)", kinds, tc.wantKind, answer.Output)
			}
			if answer.Output == nil {
				t.Fatal("output = nil, want an empty array so a client can iterate it")
			}
		})
	}
}

// TestResponsesAnswerFromCompletion_ItemShapes pins the fields a client reads
// from each item kind.
func TestResponsesAnswerFromCompletion_ItemShapes(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantIndex int
		wantType  string
		wantID    string
		wantText  string
		wantArgs  string
		wantCall  string
		wantName  string
	}{
		{
			name:     "a message item carries one output_text part",
			body:     `{"id":"c1","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`,
			wantType: schema.ResponsesItemMessage,
			wantText: "hello",
		},
		{
			name:     "a reasoning item carries one summary part",
			body:     `{"id":"c1","choices":[{"index":0,"message":{"role":"assistant","reasoning_content":"why"},"finish_reason":"stop"}]}`,
			wantType: schema.ResponsesItemReasoning,
			wantText: "why",
		},
		{
			name:     "a call item carries its id, name, and arguments",
			body:     `{"id":"c1","choices":[{"index":0,"message":{"role":"assistant","tool_calls":[{"id":"call_9","type":"function","function":{"name":"lookup","arguments":"{\"q\":1}"}}]},"finish_reason":"tool_calls"}]}`,
			wantType: schema.ResponsesItemFunctionCall,
			wantID:   "fc_call_9",
			wantCall: "call_9",
			wantName: "lookup",
			wantArgs: `{"q":1}`,
		},
		{
			name:     "an empty argument string becomes an empty object",
			body:     `{"id":"c1","choices":[{"index":0,"message":{"role":"assistant","tool_calls":[{"id":"call_9","type":"function","function":{"name":"lookup"}}]},"finish_reason":"tool_calls"}]}`,
			wantType: schema.ResponsesItemFunctionCall,
			wantID:   "fc_call_9",
			wantCall: "call_9",
			wantName: "lookup",
			wantArgs: "{}",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			answer := ResponsesAnswerFromCompletion(completionFrom(t, tc.body), "m", 0)
			if len(answer.Output) != 1 {
				t.Fatalf("output = %+v, want one item", answer.Output)
			}
			item := answer.Output[0]
			if item.Type != tc.wantType {
				t.Fatalf("item type = %q, want %q", item.Type, tc.wantType)
			}
			switch tc.wantType {
			case schema.ResponsesItemMessage:
				if len(item.Content) != 1 || item.Content[0].Type != schema.ResponsesPartOutputText {
					t.Fatalf("content = %+v, want one output_text part", item.Content)
				}
				if item.Content[0].Text != tc.wantText {
					t.Fatalf("text = %q, want %q", item.Content[0].Text, tc.wantText)
				}
				if item.Role != schema.RoleAssistant {
					t.Fatalf("role = %q, want assistant", item.Role)
				}
			case schema.ResponsesItemReasoning:
				if len(item.Summary) != 1 || item.Summary[0].Text != tc.wantText {
					t.Fatalf("summary = %+v, want one part carrying %q", item.Summary, tc.wantText)
				}
			case schema.ResponsesItemFunctionCall:
				if item.ID != tc.wantID || item.CallID != tc.wantCall || item.Name != tc.wantName {
					t.Fatalf("call item = %+v, want id %q call %q name %q", item, tc.wantID, tc.wantCall, tc.wantName)
				}
				if item.Arguments != tc.wantArgs {
					t.Fatalf("arguments = %q, want %q", item.Arguments, tc.wantArgs)
				}
			}
		})
	}
}
