// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_responses_body_test.go
// @for       The Responses body rules in one table: the members the connector
//
//	sets before a request leaves.
//
// @uses      testing, internal/registry.
// @reason    Each case is a single member the reference sets, so one table states
//
//	them together and a reader sees the whole list at once. Keeping it
//	apart from the tool_choice and chat-decoy tests also holds both files
//	inside the AGENTS.md section 1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-24
package provider

import (
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestOpenCode_ResponsesNormalizesTheBody pins the remaining field rules in one
// table, because each is a single member the reference sets before sending.
func TestOpenCode_ResponsesNormalizesTheBody(t *testing.T) {
	connector := responsesConnector()
	const model = "muse-spark-1.3-contributor-free"

	cases := []struct {
		name  string
		body  string
		check func(t *testing.T, body map[string]any)
	}{
		{
			name: "store is always false",
			body: `{"model":"` + model + `","input":[{"type":"message","role":"user","content":[]}]}`,
			check: func(t *testing.T, body map[string]any) {
				if got, ok := body["store"]; !ok || got != false {
					t.Fatalf("store = %v (present=%v), want false", got, ok)
				}
			},
		},
		{
			name: "a chat-shaped reasoning_effort becomes reasoning",
			body: `{"model":"` + model + `","input":[{"type":"message","role":"user","content":[]}],"reasoning_effort":"high"}`,
			check: func(t *testing.T, body map[string]any) {
				if _, present := body["reasoning_effort"]; present {
					t.Fatal("reasoning_effort survived, want it folded into reasoning")
				}
				reasoning, ok := body["reasoning"].(map[string]any)
				if !ok {
					t.Fatalf("reasoning = %T, want an object", body["reasoning"])
				}
				if reasoning["effort"] != "high" {
					t.Fatalf("reasoning.effort = %v, want high", reasoning["effort"])
				}
				if reasoning["summary"] != "auto" {
					t.Fatalf("reasoning.summary = %v, want auto", reasoning["summary"])
				}
			},
		},
		{
			name: "an empty input array gains a placeholder turn",
			body: `{"model":"` + model + `","input":[]}`,
			check: func(t *testing.T, body map[string]any) {
				items, ok := body["input"].([]any)
				if !ok || len(items) != 1 {
					t.Fatalf("input = %v, want one placeholder item", body["input"])
				}
				item, _ := items[0].(map[string]any)
				if item["type"] != "message" || item["role"] != "user" {
					t.Fatalf("placeholder = %v, want a user message", item)
				}
			},
		},
		{
			name: "an empty string input gains a placeholder turn",
			body: `{"model":"` + model + `","input":""}`,
			check: func(t *testing.T, body map[string]any) {
				items, ok := body["input"].([]any)
				if !ok || len(items) != 1 {
					t.Fatalf("input = %v, want one placeholder item", body["input"])
				}
			},
		},
		{
			name: "a bare string input becomes an item array",
			body: `{"model":"` + model + `","input":"hello"}`,
			check: func(t *testing.T, body map[string]any) {
				items, ok := body["input"].([]any)
				if !ok || len(items) != 1 {
					t.Fatalf("input = %v, want an item array", body["input"])
				}
				item, _ := items[0].(map[string]any)
				content, _ := item["content"].([]any)
				if len(content) != 1 {
					t.Fatalf("content = %v, want one text part", item["content"])
				}
				part, _ := content[0].(map[string]any)
				if part["text"] != "hello" {
					t.Fatalf("text = %v, want the client's own string", part["text"])
				}
			},
		},
		{
			name: "an overlong call_id is clamped and an object arguments is stringified",
			body: `{"model":"` + model + `","input":[{"type":"function_call","name":"t","call_id":"` +
				strings.Repeat("c", 80) + `","arguments":{"a":1}}]}`,
			check: func(t *testing.T, body map[string]any) {
				items, _ := body["input"].([]any)
				item, _ := items[0].(map[string]any)
				callID, _ := item["call_id"].(string)
				if len(callID) != 64 {
					t.Fatalf("call_id length = %d, want 64 (got %q)", len(callID), callID)
				}
				if args, ok := item["arguments"].(string); !ok || args != `{"a":1}` {
					t.Fatalf("arguments = %#v, want the object stringified", item["arguments"])
				}
			},
		},
		{
			name: "a chat-shaped tool is flattened to the Responses shape",
			body: `{"model":"` + model + `","input":[{"type":"message","role":"user","content":[]}],"tools":[{"type":"function","function":{"name":"my_tool","description":"d"}}]}`,
			check: func(t *testing.T, body map[string]any) {
				items, _ := body["tools"].([]any)
				if len(items) == 0 {
					t.Fatal("tools is empty")
				}
				tool, _ := items[0].(map[string]any)
				if tool["name"] != "my_tool" {
					t.Fatalf("tool name = %v, want it flattened onto the tool itself", tool["name"])
				}
				if _, nested := tool["function"]; nested {
					t.Fatal("the chat-shaped function wrapper survived, want it flattened")
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := Request{
				Model: registry.Model{ID: model, TargetFormat: registry.FormatOpenAIResponses},
				Body:  []byte(tc.body),
			}
			if err := connector.TransformRequest(&request); err != nil {
				t.Fatalf("TransformRequest() error = %v", err)
			}
			tc.check(t, decodeBody(t, request.Body))
		})
	}
}

// TestOpenCode_ChatWireForbidsTheDecoysWhenTheClientDeclaredNoTools pins the last
// delta: a chat request with no client tools gets `tool_choice: "none"`, so the
// decoys the gate requires can never be selected. Without it the model is free to
// call a tool whose own description says it is unavailable.
func TestOpenCode_ChatWireForbidsTheDecoysWhenTheClientDeclaredNoTools(t *testing.T) {
	connector := responsesConnector()

	cases := []struct {
		name      string
		tools     string
		want      string
		wantFixed bool
	}{
		{name: "no tools at all", tools: ``, want: "none"},
		{name: "an empty tool list", tools: `,"tools":[]`, want: "none"},
		{name: "a client tool is present, so no choice is written", tools: `,"tools":[{"type":"function","function":{"name":"glob"}}]`, wantFixed: true},
		{name: "the client's own choice is kept", tools: `,"tools":[{"type":"function","function":{"name":"glob"}}],"tool_choice":"required"`, want: "required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := Request{
				Model: registry.Model{ID: "big-pickle"},
				Body:  []byte(`{"model":"big-pickle","messages":[]` + tc.tools + `}`),
			}
			if err := connector.TransformRequest(&request); err != nil {
				t.Fatalf("TransformRequest() error = %v", err)
			}
			got, present := decodeBody(t, request.Body)["tool_choice"]
			if tc.wantFixed {
				if present {
					t.Fatalf("tool_choice = %v, want no choice written for a client that declared tools", got)
				}
				return
			}
			if !present || got != tc.want {
				t.Fatalf("tool_choice = %v (present=%v), want %q", got, present, tc.want)
			}
		})
	}
}
