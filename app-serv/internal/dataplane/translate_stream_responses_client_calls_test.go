// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_stream_responses_client_calls_test.go
// @for       Table-driven tests for the function_call items a Responses client
//
//	stream reports.
//
// @uses      testing, slices.
// @reason    SPEC-API-001 §7.15 serves POST /api/v1/responses, and a call arrives
//
//	as fragments spread across deltas: the id and name on the opening one,
//	the arguments after it. A client dispatches on the assembled item, so
//	losing a fragment or reopening a closed item is a broken tool call.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"slices"
	"testing"
)

// callFragment builds one tool-call entry of a delta.
func callFragment(index int, id, name, arguments string) map[string]any {
	function := map[string]any{"arguments": arguments}
	if name != "" {
		function["name"] = name
	}
	entry := map[string]any{"index": index, "function": function}
	if id != "" {
		entry["id"] = id
		entry["type"] = "function"
	}
	return entry
}

// TestResponsesClientStream_CallItems pins the item a call produces: it opens on
// the fragment that names it, accumulates the argument fragments, and closes with
// the assembled arguments rather than the last one.
func TestResponsesClientStream_CallItems(t *testing.T) {
	cases := []struct {
		name      string
		deltas    []map[string]any
		wantTrace []string
		wantItems int
		wantID    string
		wantName  string
		wantArgs  string
	}{
		{
			name: "a call opens on its id and accumulates the fragments", wantItems: 1,
			deltas: []map[string]any{
				{"tool_calls": []any{callFragment(0, "call_1", "lookup", "")}},
				{"tool_calls": []any{callFragment(0, "", "", `{"q":`)}},
				{"tool_calls": []any{callFragment(0, "", "", `1}`)}},
			},
			wantTrace: []string{
				"response.output_item.added:fc_call_1@0",
				"response.function_call_arguments.delta:fc_call_1@0",
				"response.function_call_arguments.delta:fc_call_1@0",
				"response.function_call_arguments.done:fc_call_1@0",
				"response.output_item.done:fc_call_1@0",
			},
			wantID: "fc_call_1", wantName: "lookup", wantArgs: `{"q":1}`,
		},
		{
			name: "a call with no id gets a deterministic placeholder", wantItems: 1,
			deltas: []map[string]any{
				{"tool_calls": []any{callFragment(2, "", "lookup", "{}")}},
			},
			wantTrace: []string{
				"response.output_item.added:fc_call_pannelai_2@0",
				"response.function_call_arguments.delta:fc_call_pannelai_2@0",
				"response.function_call_arguments.done:fc_call_pannelai_2@0",
				"response.output_item.done:fc_call_pannelai_2@0",
			},
			wantID: "fc_call_pannelai_2", wantName: "lookup", wantArgs: "{}",
		},
		{
			name: "a call that never reported arguments closes with an empty object", wantItems: 1,
			deltas: []map[string]any{
				{"tool_calls": []any{callFragment(0, "call_1", "lookup", "")}},
			},
			wantTrace: []string{
				"response.output_item.added:fc_call_1@0",
				"response.function_call_arguments.done:fc_call_1@0",
				"response.output_item.done:fc_call_1@0",
			},
			wantID: "fc_call_1", wantName: "lookup", wantArgs: "{}",
		},
		{
			name: "two calls get one item each", wantItems: 2,
			deltas: []map[string]any{
				{"tool_calls": []any{
					callFragment(0, "call_1", "lookup", `{}`),
					callFragment(1, "call_2", "store", `{}`),
				}},
			},
			wantTrace: []string{
				"response.output_item.added:fc_call_1@0",
				"response.function_call_arguments.delta:fc_call_1@0",
				"response.output_item.added:fc_call_2@1",
				"response.function_call_arguments.delta:fc_call_2@1",
				"response.function_call_arguments.done:fc_call_1@0",
				"response.output_item.done:fc_call_1@0",
				"response.function_call_arguments.done:fc_call_2@1",
				"response.output_item.done:fc_call_2@1",
			},
			wantID: "fc_call_1", wantName: "lookup", wantArgs: "{}",
		},
		{
			name: "a name that arrives after the opening fragment fills the item in", wantItems: 1,
			deltas: []map[string]any{
				{"tool_calls": []any{callFragment(0, "call_1", "", "")}},
				{"tool_calls": []any{callFragment(0, "", "lookup", "{}")}},
			},
			wantTrace: []string{
				"response.output_item.added:fc_call_1@0",
				"response.function_call_arguments.delta:fc_call_1@0",
				"response.function_call_arguments.done:fc_call_1@0",
				"response.output_item.done:fc_call_1@0",
			},
			wantID: "fc_call_1", wantName: "lookup", wantArgs: "{}",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := NewResponsesStreamState("up-1", "pannelai-model", 1700000000)
			payloads := make([]string, 0, len(tc.deltas)+1)
			for _, delta := range tc.deltas {
				payloads = append(payloads, openAIChunk(delta, ""))
			}
			frames := feedClient(state, TargetOpenAI, payloads...)
			frames = append(frames, state.Finish()...)

			if got := itemTrace(t, frames); !slices.Equal(got, tc.wantTrace) {
				t.Fatalf("item trace =\n%v\nwant\n%v", got, tc.wantTrace)
			}
			items := completedOutput(t, frames)
			if len(items) != tc.wantItems {
				t.Fatalf("got %d items, want %d: %v", len(items), tc.wantItems, items)
			}
			item := items[0]
			for member, want := range map[string]string{
				"id": tc.wantID, "name": tc.wantName, "arguments": tc.wantArgs,
			} {
				if got, _ := item[member].(string); got != want {
					t.Fatalf("item %s = %q, want %q", member, got, want)
				}
			}
			if item["type"] != "function_call" {
				t.Fatalf("item type = %v, want function_call", item["type"])
			}
			if got, _ := item["call_id"].(string); got != tc.wantID[3:] {
				t.Fatalf("call_id = %q, want %q", got, tc.wantID[3:])
			}
		})
	}
}

// TestResponsesClientStream_CallClosingText pins the item order when text follows
// a call: the message closes before the call opens, and later text opens a fresh
// message item rather than feeding one the client already saw close.
func TestResponsesClientStream_CallClosingText(t *testing.T) {
	state := NewResponsesStreamState("up-1", "pannelai-model", 1700000000)
	frames := feedClient(state, TargetOpenAI,
		openAIChunk(map[string]any{"content": "before"}, ""),
		openAIChunk(map[string]any{"tool_calls": []any{callFragment(0, "call_1", "lookup", "{}")}}, ""),
		openAIChunk(map[string]any{"content": "after"}, ""),
		openAIChunk(map[string]any{}, FinishToolCalls))
	frames = append(frames, state.Finish()...)

	want := []string{
		"response.output_item.added:msg_resp_up-1_0@0",
		"response.content_part.added:msg_resp_up-1_0@0",
		"response.output_text.delta:msg_resp_up-1_0@0",
		"response.output_text.done:msg_resp_up-1_0@0",
		"response.content_part.done:msg_resp_up-1_0@0",
		"response.output_item.done:msg_resp_up-1_0@0",
		"response.output_item.added:fc_call_1@1",
		"response.function_call_arguments.delta:fc_call_1@1",
		"response.output_item.added:msg_resp_up-1_2@2",
		"response.content_part.added:msg_resp_up-1_2@2",
		"response.output_text.delta:msg_resp_up-1_2@2",
		"response.function_call_arguments.done:fc_call_1@1",
		"response.output_item.done:fc_call_1@1",
		"response.output_text.done:msg_resp_up-1_2@2",
		"response.content_part.done:msg_resp_up-1_2@2",
		"response.output_item.done:msg_resp_up-1_2@2",
	}
	if got := itemTrace(t, frames); !slices.Equal(got, want) {
		t.Fatalf("item trace =\n%v\nwant\n%v", got, want)
	}

	items := completedOutput(t, frames)
	kinds := make([]string, 0, len(items))
	for _, item := range items {
		kind, _ := item["type"].(string)
		kinds = append(kinds, kind)
	}
	if wantKinds := []string{"message", "function_call", "message"}; !slices.Equal(kinds, wantKinds) {
		t.Fatalf("item kinds = %v, want %v", kinds, wantKinds)
	}
}
