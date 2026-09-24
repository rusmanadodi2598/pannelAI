// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_responses_test.go
// @for       The Responses-wire field rules the reference applies before a
//
//	request leaves for the OpenCode upstream.
//
// @uses      testing, internal/registry.
// @reason    Six byte-level differences were measured between this connector and
//
//	the reference's transformRequest, each one a request the upstream
//	refuses or answers differently: a tool_choice forced to auto on a model
//	that does not have the quirk, an absent store, a chat-shaped
//	reasoning_effort, an empty input array, an overlong call_id with an
//	object arguments, and a chat-shaped tool declaration. They are pinned
//	together because they share one decision (how far the connector may
//	rewrite a same-format body) and because the reference applies them in
//	one function.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-24
package provider

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// responsesConnector builds a connector whose entry declares the quirk the
// reference gives opencode, so a test can tell a quirk model from a plain one.
func responsesConnector() *OpenCode {
	entry := opencodeEntry("https://opencode.ai", registry.DefaultFormat)
	entry.Transport.Quirks.ForceAutoToolChoiceModels = []string{"muse-spark-1.3-contributor-free"}
	return NewOpenCode(entry)
}

// TestOpenCode_ResponsesForcesAutoOnlyForTheQuirkModel pins the first delta: the
// reference demotes tool_choice to auto only for the models its own registry
// lists (registry/opencode.js:22-24), because the wire accepts other values on
// the rest. Forcing it everywhere silently changes a client's request.
func TestOpenCode_ResponsesForcesAutoOnlyForTheQuirkModel(t *testing.T) {
	connector := responsesConnector()

	cases := []struct {
		name     string
		model    string
		choice   string
		wantAuto bool
	}{
		{name: "the quirk model is demoted from required", model: "muse-spark-1.3-contributor-free", choice: `"required"`, wantAuto: true},
		{name: "the quirk model is demoted from a function", model: "muse-spark-1.3-contributor-free", choice: `{"type":"function","name":"t"}`, wantAuto: true},
		{name: "the quirk model keeps auto", model: "muse-spark-1.3-contributor-free", choice: `"auto"`, wantAuto: true},
		{name: "a plain model keeps required", model: "muse-spark-1.2-contributor-free", choice: `"required"`, wantAuto: false},
		{name: "a plain model keeps a function choice", model: "muse-spark-1.2-contributor-free", choice: `{"type":"function","name":"t"}`, wantAuto: false},
		{name: "an absent choice becomes auto", model: "muse-spark-1.2-contributor-free", choice: "", wantAuto: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"model":"` + tc.model + `","input":[]`
			if tc.choice != "" {
				body += `,"tool_choice":` + tc.choice
			}
			body += `}`
			request := Request{
				Model: registry.Model{ID: tc.model, TargetFormat: registry.FormatOpenAIResponses},
				Body:  []byte(body),
			}
			if err := connector.TransformRequest(&request); err != nil {
				t.Fatalf("TransformRequest() error = %v", err)
			}
			got, present := decodeBody(t, request.Body)["tool_choice"]
			if !present {
				t.Fatal("tool_choice is absent, want the wire to carry one")
			}
			if tc.wantAuto && got != "auto" {
				t.Fatalf("tool_choice = %v, want auto", got)
			}
			if !tc.wantAuto {
				// Deep-compare the decoded forms, because a tool_choice may be an
				// object and two decoded maps are not comparable with ==.
				var want any
				if err := json.Unmarshal([]byte(tc.choice), &want); err != nil {
					t.Fatalf("decoding the case's own choice: %v", err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("tool_choice = %#v, want the client's %#v kept", got, want)
				}
			}
		})
	}
}

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
