// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_stream_responses_tools_test.go
// @for       Table-driven tests for streamed tool calls and stream completion.
// @uses      testing.
// @reason    SPEC-API-001 §10 makes the Responses API a P3 deliverable, and a
//
//	streamed call is where the two wires differ most: the Responses
//	stream opens a call by name and then feeds its arguments in
//	fragments, so the index the fragments attach to is state the mapper
//	has to keep. The finish reason and the failure frame are pinned
//	here with it, since all three close the same stream.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"testing"
)

// toolCallIn accumulates one tool call across the frames a stream produced,
// which is what a client does with them: the id, type, and name arrive with the
// opening frame, and the argument fragments arrive in the frames after it.
func toolCallIn(t *testing.T, frames [][]byte, index int) map[string]any {
	t.Helper()
	merged := map[string]any{"arguments": ""}
	seen := false
	for _, frame := range frames {
		choices, ok := decodeFrame(t, frame)["choices"].([]any)
		if !ok || len(choices) == 0 {
			continue
		}
		choice, _ := choices[0].(map[string]any)
		delta, _ := choice["delta"].(map[string]any)
		calls, _ := delta["tool_calls"].([]any)
		for _, raw := range calls {
			call, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if got, _ := call["index"].(float64); int(got) != index {
				continue
			}
			seen = true
			for _, key := range []string{"id", "type"} {
				if value, ok := call[key].(string); ok && value != "" {
					merged[key] = value
				}
			}
			function, _ := call["function"].(map[string]any)
			if name, ok := function["name"].(string); ok && name != "" {
				merged["name"] = name
			}
			if args, ok := function["arguments"].(string); ok {
				merged["arguments"] = merged["arguments"].(string) + args
			}
		}
	}
	if !seen {
		t.Fatalf("no tool call at index %d in %q", index, frameText(frames))
	}
	return merged
}

// TestResponsesStreamState_ToolCalls pins the call sequence: an added item opens
// the call by name, each argument delta feeds the open call, and the item being
// done advances the index so a second call does not overwrite the first.
func TestResponsesStreamState_ToolCalls(t *testing.T) {
	cases := []struct {
		name      string
		events    []string
		wantID    string
		wantName  string
		wantArgs  string
		wantIndex int
	}{
		{
			name: "an added function call opens a call by name",
			events: []string{
				`{"type":"response.output_item.added","item":{"type":"function_call","call_id":"call_7","name":"lookup"}}`,
			},
			wantID:   "call_7",
			wantName: "lookup",
			wantArgs: "",
		},
		{
			name: "argument fragments accumulate into the open call",
			events: []string{
				`{"type":"response.output_item.added","item":{"type":"function_call","call_id":"call_7","name":"lookup"}}`,
				`{"type":"response.function_call_arguments.delta","delta":"{\"q\":"}`,
				`{"type":"response.function_call_arguments.delta","delta":"1}"}`,
			},
			wantID:   "call_7",
			wantName: "lookup",
			wantArgs: `{"q":1}`,
		},
		{
			name: "a call with no id gets a deterministic placeholder",
			events: []string{
				`{"type":"response.output_item.added","item":{"type":"function_call","name":"lookup"}}`,
			},
			wantID:   "call_pannelai_0",
			wantName: "lookup",
			wantArgs: "",
		},
		{
			name: "a custom tool call is opened the same way",
			events: []string{
				`{"type":"response.output_item.added","item":{"type":"custom_tool_call","call_id":"call_8","name":"shell"}}`,
				`{"type":"response.custom_tool_call_input.delta","delta":"ls"}`,
			},
			wantID:   "call_8",
			wantName: "shell",
			wantArgs: "ls",
		},
		{
			name: "a second call is opened at the next index",
			events: []string{
				`{"type":"response.output_item.added","item":{"type":"function_call","call_id":"call_1","name":"first"}}`,
				`{"type":"response.output_item.done","item":{"type":"function_call","call_id":"call_1","name":"first"}}`,
				`{"type":"response.output_item.added","item":{"type":"function_call","call_id":"call_2","name":"second"}}`,
			},
			wantID:    "call_2",
			wantName:  "second",
			wantArgs:  "",
			wantIndex: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := NewStreamState("", "m", 1700000000, false)
			var frames [][]byte
			for _, event := range tc.events {
				frames = append(frames, state.Frames(TargetResponses, []byte(event))...)
			}
			if len(frames) == 0 {
				t.Fatalf("want frames to read the call from, got none for %+v", tc.events)
			}
			call := toolCallIn(t, frames, tc.wantIndex)
			if id, _ := call["id"].(string); id != tc.wantID {
				t.Fatalf("call id = %q, want %q (%q)", id, tc.wantID, frameText(frames))
			}
			if name, _ := call["name"].(string); name != tc.wantName {
				t.Fatalf("call name = %q, want %q", name, tc.wantName)
			}
			if args, _ := call["arguments"].(string); args != tc.wantArgs {
				t.Fatalf("call arguments = %q, want %q", args, tc.wantArgs)
			}
			if callType, _ := call["type"].(string); callType != "function" {
				t.Fatalf("call type = %q, want function", callType)
			}
		})
	}
}

// TestResponsesStreamState_Completed pins the close of a stream: the finish
// reason a client dispatches on, and the accounting the finish frame carries.
func TestResponsesStreamState_Completed(t *testing.T) {
	cases := []struct {
		name       string
		events     []string
		wantFinish string
		wantUsage  bool
		wantPrompt int
	}{
		{
			name:       "a text-only answer finishes as a stop",
			events:     []string{`{"type":"response.output_text.delta","delta":"x"}`, `{"type":"response.completed","response":{"status":"completed"}}`},
			wantFinish: FinishStop,
		},
		{
			name:       "an answer with a call finishes as tool_calls",
			events:     []string{`{"type":"response.output_item.added","item":{"type":"function_call","call_id":"c1","name":"lookup"}}`, `{"type":"response.completed","response":{"status":"completed"}}`},
			wantFinish: FinishToolCalls,
		},
		{
			name:       "the done event closes the stream the same way",
			events:     []string{`{"type":"response.done","response":{"status":"completed"}}`},
			wantFinish: FinishStop,
		},
		{
			name:       "a completed answer carries its accounting",
			events:     []string{`{"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":12,"output_tokens":3}}}`},
			wantFinish: FinishStop,
			wantUsage:  true,
			wantPrompt: 12,
		},
		{
			name:       "an answer cut by the ceiling still finishes as a stop",
			events:     []string{`{"type":"response.completed","response":{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"}}}`},
			wantFinish: FinishStop,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := NewStreamState("", "m", 1700000000, true)
			var frames [][]byte
			for _, event := range tc.events {
				frames = append(frames, state.Frames(TargetResponses, []byte(event))...)
			}
			if len(frames) == 0 {
				t.Fatal("want a finish frame")
			}
			finish := frameFinishReason(t, finishFrame(t, frames))
			if finish == nil || *finish != tc.wantFinish {
				t.Fatalf("finish reason = %v, want %q (%q)", finish, tc.wantFinish, frameText(frames))
			}
			if !tc.wantUsage {
				return
			}
			usage := state.Usage()
			if usage == nil {
				t.Fatal("usage = nil, want the accounting the event carried")
			}
			if usage.PromptTokens != tc.wantPrompt {
				t.Fatalf("prompt tokens = %d, want %d", usage.PromptTokens, tc.wantPrompt)
			}
		})
	}
}
