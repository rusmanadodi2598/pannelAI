// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_responses_tools_test.go
// @for       Table-driven tests for Responses tool declarations and call items.
// @uses      testing, encoding/json, internal/schema.
// @reason    SPEC-API-001 §10 makes the Responses API a P3 deliverable, and its
//
//	tool vocabulary is where the two wires differ most: declarations
//	lose OpenAI's `function` wrapper, calls travel as items rather than
//	inside a message, and call_id is capped. A translation that gets
//	any of these wrong is rejected by the upstream outright.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"encoding/json"
	"strings"
	"testing"
)

// boolPointer gives a table case a distinct value for "declared false", which a
// nil pointer has to stay distinguishable from.
func boolPointer(value bool) *bool { return &value }

// TestOpenAIToResponses_CallItems pins the function-call item rules.
//
// The API rejects a call with no name and caps call_id at 64 characters, so both
// are handled here: a nameless call is dropped (forwarding it fails the request),
// and a long id is clamped (sending it unclamped is a guaranteed rejection).
func TestOpenAIToResponses_CallItems(t *testing.T) {
	longID := strings.Repeat("x", ResponsesCallIDMaxLen+8)
	cases := []struct {
		name       string
		body       string
		wantItems  int
		wantCallID string
		wantArgs   string
	}{
		{
			name:       "a named call is forwarded with its id and arguments",
			body:       `{"model":"m","messages":[{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":1}"}}]}]}`,
			wantItems:  1,
			wantCallID: "call_1",
			wantArgs:   `{"q":1}`,
		},
		{
			name:       "an id over the ceiling is clamped to it",
			body:       `{"model":"m","messages":[{"role":"assistant","content":"","tool_calls":[{"id":"` + longID + `","type":"function","function":{"name":"lookup","arguments":"{}"}}]}]}`,
			wantItems:  1,
			wantCallID: strings.Repeat("x", ResponsesCallIDMaxLen),
			wantArgs:   "{}",
		},
		{
			name:       "an id exactly at the ceiling is kept whole",
			body:       `{"model":"m","messages":[{"role":"assistant","content":"","tool_calls":[{"id":"` + strings.Repeat("y", ResponsesCallIDMaxLen) + `","type":"function","function":{"name":"lookup","arguments":"{}"}}]}]}`,
			wantItems:  1,
			wantCallID: strings.Repeat("y", ResponsesCallIDMaxLen),
			wantArgs:   "{}",
		},
		{
			name:      "a nameless call is dropped rather than sent to be rejected",
			body:      `{"model":"m","messages":[{"role":"assistant","content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"","arguments":"{}"}}]}]}`,
			wantItems: 0,
		},
		{
			name:       "empty arguments default to an empty object",
			body:       `{"model":"m","messages":[{"role":"assistant","content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"lookup"}}]}]}`,
			wantItems:  1,
			wantCallID: "c1",
			wantArgs:   "{}",
		},
		{
			name:       "two calls each become their own item",
			body:       `{"model":"m","messages":[{"role":"assistant","content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"a","arguments":"{}"}},{"id":"c2","type":"function","function":{"name":"b","arguments":"{}"}}]}]}`,
			wantItems:  2,
			wantCallID: "c1",
			wantArgs:   "{}",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := OpenAIToResponses(chatRequest(t, tc.body), "m", false)
			calls := make([]ResponsesItem, 0, len(got.Input))
			for _, item := range got.Input {
				if item.Type == ItemFunctionCall {
					calls = append(calls, item)
				}
			}
			if len(calls) != tc.wantItems {
				t.Fatalf("function_call items = %d, want %d (%+v)", len(calls), tc.wantItems, got.Input)
			}
			if tc.wantItems == 0 {
				return
			}
			if calls[0].CallID != tc.wantCallID {
				t.Fatalf("call_id = %q, want %q", calls[0].CallID, tc.wantCallID)
			}
			if calls[0].Arguments != tc.wantArgs {
				t.Fatalf("arguments = %q, want %q", calls[0].Arguments, tc.wantArgs)
			}
			if calls[0].Name == "" {
				t.Fatal("name is empty, want the function name")
			}
		})
	}
}

// TestOpenAIToResponses_ToolDeclarations pins the flattened tool shape and the
// max_tokens rule: the Responses API takes the function's members directly, and
// an unset ceiling must be omitted rather than sent as zero, which some
// providers read as "produce nothing".
func TestOpenAIToResponses_ToolDeclarations(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantTools  int
		wantSchema string
		wantStrict *bool
		wantMax    int
		wantNoMax  bool
	}{
		{
			name:      "no tools leaves the field unset",
			body:      `{"model":"m","messages":[{"role":"user","content":"q"}]}`,
			wantTools: 0,
			wantNoMax: true,
		},
		{
			name:       "a declared tool is flattened with its schema",
			body:       `{"model":"m","messages":[{"role":"user","content":"q"}],"tools":[{"type":"function","function":{"name":"lookup","description":"d","parameters":{"type":"object","properties":{"q":{"type":"string"}}},"strict":true}}]}`,
			wantTools:  1,
			wantSchema: `{"type":"object","properties":{"q":{"type":"string"}}}`,
			wantStrict: boolPointer(true),
			wantNoMax:  true,
		},
		{
			name:       "a tool with no schema gets the empty object fallback",
			body:       `{"model":"m","messages":[{"role":"user","content":"q"}],"tools":[{"type":"function","function":{"name":"lookup"}}]}`,
			wantTools:  1,
			wantSchema: `{"type":"object","properties":{}}`,
			wantNoMax:  true,
		},
		{
			name:       "an explicit false strict is forwarded as declared",
			body:       `{"model":"m","messages":[{"role":"user","content":"q"}],"tools":[{"type":"function","function":{"name":"lookup","strict":false}}]}`,
			wantTools:  1,
			wantSchema: `{"type":"object","properties":{}}`,
			wantStrict: boolPointer(false),
			wantNoMax:  true,
		},
		{
			name:      "a max_tokens ceiling is carried through",
			body:      `{"model":"m","messages":[{"role":"user","content":"q"}],"max_tokens":512}`,
			wantTools: 0,
			wantMax:   512,
		},
		{
			name:      "a zero ceiling is omitted rather than sent",
			body:      `{"model":"m","messages":[{"role":"user","content":"q"}],"max_tokens":0}`,
			wantTools: 0,
			wantNoMax: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := OpenAIToResponses(chatRequest(t, tc.body), "m", false)
			if len(got.Tools) != tc.wantTools {
				t.Fatalf("len(Tools) = %d, want %d (%+v)", len(got.Tools), tc.wantTools, got.Tools)
			}
			if tc.wantTools > 0 {
				tool := got.Tools[0]
				if tool.Name != "lookup" {
					t.Fatalf("tool name = %q, want the function name", tool.Name)
				}
				if string(tool.Parameters) != tc.wantSchema {
					t.Fatalf("parameters = %s, want %s", tool.Parameters, tc.wantSchema)
				}
				if (tool.Strict == nil) != (tc.wantStrict == nil) {
					t.Fatalf("strict = %v, want %v: an undeclared flag must stay undeclared so the upstream default applies", tool.Strict, tc.wantStrict)
				}
				if tc.wantStrict != nil && *tool.Strict != *tc.wantStrict {
					t.Fatalf("strict = %v, want %v", *tool.Strict, *tc.wantStrict)
				}
			}
			if tc.wantNoMax {
				if got.MaxTokens != nil {
					t.Fatalf("MaxTokens = %d, want it omitted", *got.MaxTokens)
				}
				return
			}
			if got.MaxTokens == nil || *got.MaxTokens != tc.wantMax {
				t.Fatalf("MaxTokens = %v, want %d", got.MaxTokens, tc.wantMax)
			}
		})
	}
}

// TestResponsesRequest_WireShape pins the JSON the upstream actually receives:
// store must be present and false, and the input array must serialize as items
// with their content parts.
func TestResponsesRequest_WireShape(t *testing.T) {
	got := OpenAIToResponses(chatRequest(t, `{"model":"m","messages":[{"role":"user","content":"q"}]}`), "m", false)
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshalling the payload: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshalling the payload: %v", err)
	}
	if store, ok := decoded["store"]; !ok || store != false {
		t.Fatalf("store = %v (present=%v), want false", store, ok)
	}
	input, ok := decoded["input"].([]any)
	if !ok || len(input) != 1 {
		t.Fatalf("input = %v, want one item", decoded["input"])
	}
	item, ok := input[0].(map[string]any)
	if !ok || item["type"] != ItemMessage || item["role"] != "user" {
		t.Fatalf("item = %v, want a user message item", input[0])
	}
}
