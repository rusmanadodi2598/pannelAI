// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_wire_test.go
// @for       The Responses-wire rules the OpenCode connector applies: the output
//
//	ceiling's field name, the statelessness of prior reasoning items, and
//	the refusal of a body it cannot read.
//
// @uses      testing, encoding/json, internal/registry.
// @reason    The Responses wire is stricter than the chat one, so its rules are
//
//	pinned apart from the shape the two wires share. Keeping them here
//	also keeps both files inside the AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-21
package provider

import (
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestOpenCode_TransformNamesTheResponsesOutputCeiling pins the field rename the
// Responses wire requires: it answers 400 to `max_tokens`, so a chat-shaped
// ceiling has to become `max_output_tokens`, and an already-correct one must not
// be overwritten.
func TestOpenCode_TransformNamesTheResponsesOutputCeiling(t *testing.T) {
	connector := NewOpenCode(opencodeEntry("https://opencode.ai", "openai"))

	cases := []struct {
		name       string
		body       string
		wantCeil   float64
		wantAbsent []string
	}{
		{
			name:       "max_tokens becomes max_output_tokens",
			body:       `{"model":"muse-spark-1.3-contributor-free","input":[],"max_tokens":512}`,
			wantCeil:   512,
			wantAbsent: []string{"max_tokens"},
		},
		{
			name:       "max_completion_tokens becomes max_output_tokens",
			body:       `{"model":"muse-spark-1.3-contributor-free","input":[],"max_completion_tokens":256}`,
			wantCeil:   256,
			wantAbsent: []string{"max_tokens", "max_completion_tokens"},
		},
		{
			name:     "an existing max_output_tokens wins",
			body:     `{"model":"muse-spark-1.3-contributor-free","input":[],"max_output_tokens":128,"max_tokens":999}`,
			wantCeil: 128, wantAbsent: []string{"max_tokens"},
		},
		{
			name:       "no ceiling at all",
			body:       `{"model":"muse-spark-1.3-contributor-free","input":[]}`,
			wantAbsent: []string{"max_tokens", "max_completion_tokens", "max_output_tokens"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := Request{
				Model: registry.Model{ID: "muse-spark-1.3-contributor-free", TargetFormat: "openai-responses"},
				Body:  []byte(tc.body),
			}
			if err := connector.TransformRequest(&request); err != nil {
				t.Fatalf("TransformRequest() error = %v", err)
			}
			body := decodeBody(t, request.Body)
			if tc.wantCeil > 0 {
				if got := body["max_output_tokens"]; got != tc.wantCeil {
					t.Fatalf("max_output_tokens = %v, want %v", got, tc.wantCeil)
				}
			}
			for _, key := range tc.wantAbsent {
				if value, present := body[key]; present {
					t.Fatalf("%s = %v, want it absent", key, value)
				}
			}
		})
	}
}

// TestOpenCode_TransformDropsPriorReasoningItems pins the Responses statelessness
// rule: the free tier pools anonymous credentials, so a reasoning item issued to
// one caller cannot be replayed to another and its encrypted content must not be
// forwarded. The items that carry the conversation must survive.
func TestOpenCode_TransformDropsPriorReasoningItems(t *testing.T) {
	connector := NewOpenCode(opencodeEntry("https://opencode.ai", "openai"))

	body := `{"model":"muse-spark-1.3-contributor-free","input":[` +
		`{"type":"reasoning","encrypted_content":"secret","summary":[]},` +
		`{"type":"message","role":"user","content":[{"type":"input_text","text":"q"}],` +
		`"encrypted_content":"also-secret"},` +
		`{"type":"function_call","name":"bash","call_id":"call_1","arguments":"{}"},` +
		`{"type":"function_call_output","call_id":"call_1","output":"done"}]}`
	request := Request{
		Model: registry.Model{ID: "muse-spark-1.3-contributor-free", TargetFormat: "openai-responses"},
		Body:  []byte(body),
	}
	if err := connector.TransformRequest(&request); err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}

	items, ok := decodeBody(t, request.Body)["input"].([]any)
	if !ok {
		t.Fatalf("input = %T, want an array", decodeBody(t, request.Body)["input"])
	}
	types := make([]string, 0, len(items))
	for _, entry := range items {
		item, ok := entry.(map[string]any)
		if !ok {
			t.Fatalf("item = %T, want an object", entry)
		}
		if itemType, _ := item["type"].(string); itemType != "" {
			types = append(types, itemType)
		}
		if _, present := item["encrypted_content"]; present {
			t.Fatalf("item %v still carries encrypted_content", item["type"])
		}
	}
	for _, want := range []string{"message", "function_call", "function_call_output"} {
		if !contains(types, want) {
			t.Fatalf("item types = %v, want %q kept", types, want)
		}
	}
	if contains(types, "reasoning") {
		t.Fatalf("item types = %v, want no reasoning item", types)
	}
}

// TestOpenCode_TransformLeavesTheChatWireAlone pins the benign control: the chat
// wire has no max_output_tokens rename and no reasoning items to drop, so a chat
// body's own members must survive the transform untouched.
func TestOpenCode_TransformLeavesTheChatWireAlone(t *testing.T) {
	connector := NewOpenCode(opencodeEntry("https://opencode.ai", "openai"))

	body := `{"model":"big-pickle","messages":[{"role":"user","content":"q"}],"max_tokens":64,"temperature":0.5}`
	request := Request{Model: registry.Model{ID: "big-pickle"}, Body: []byte(body)}
	if err := connector.TransformRequest(&request); err != nil {
		t.Fatalf("TransformRequest() error = %v", err)
	}
	decoded := decodeBody(t, request.Body)
	if got := decoded["max_tokens"]; got != float64(64) {
		t.Fatalf("max_tokens = %v, want the client's 64 kept on the chat wire", got)
	}
	if _, present := decoded["max_output_tokens"]; present {
		t.Fatal("max_output_tokens must not be added to a chat body")
	}
	if got := decoded["temperature"]; got != 0.5 {
		t.Fatalf("temperature = %v, want the client's value kept", got)
	}
}

// TestOpenCode_TransformRefusesAnUnreadableBody pins the failure mode: a body the
// connector cannot read is reported as a client error, never forwarded half
// rewritten and never a panic.
func TestOpenCode_TransformRefusesAnUnreadableBody(t *testing.T) {
	connector := NewOpenCode(opencodeEntry("https://opencode.ai", "openai"))

	cases := []struct {
		name string
		body string
	}{
		{name: "empty", body: ""},
		{name: "not json", body: "not json at all"},
		{name: "a bare array", body: `[1,2,3]`},
		{name: "a scalar", body: `"a string"`},
		{name: "truncated", body: `{"model":"big-pickle"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := Request{Model: registry.Model{ID: "big-pickle"}, Body: []byte(tc.body)}
			if err := connector.TransformRequest(&request); err == nil {
				t.Fatalf("TransformRequest(%q) = nil error, want a refusal", tc.body)
			}
		})
	}
}

// TestOpenCode_ForcesStream pins the connector's declaration that it only
// answers a stream. The core reads it to decide whether a non-streaming client
// needs its answer folded back from the stream.
func TestOpenCode_ForcesStream(t *testing.T) {
	connector := NewOpenCode(opencodeEntry("https://opencode.ai", "openai"))
	if !connector.ForcesStream() {
		t.Fatal("ForcesStream() = false, want true: the free tier refuses a non-streaming request")
	}
}
