// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_stream_responses_client_text_test.go
// @for       Table-driven tests for the reasoning and message items a Responses
//
//	client stream reports.
//
// @uses      testing, slices.
// @reason    SPEC-API-001 §7.15 serves POST /api/v1/responses, and reasoning and
//
//	text are the two item kinds a coding agent renders. A reasoning item
//	that shares an index with the message, or a message that reopens after
//	it closed, is a malformed answer even when the text is right.
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

// TestResponsesClientStream_ReasoningThenText pins the item structure of an
// answer that thinks before it speaks: reasoning opens at its own index, the
// message follows at the next one, and both close in the order they opened.
func TestResponsesClientStream_ReasoningThenText(t *testing.T) {
	state := NewResponsesStreamState("up-1", "pannelai-model", 1700000000)
	frames := feedClient(state, TargetOpenAI,
		openAIChunk(map[string]any{"reasoning_content": "why"}, ""),
		openAIChunk(map[string]any{"content": "because"}, ""),
		openAIChunk(map[string]any{}, FinishStop))
	frames = append(frames, state.Finish()...)

	want := []string{
		"response.output_item.added:rs_resp_up-1_0@0",
		"response.reasoning_summary_part.added:rs_resp_up-1_0@0",
		"response.reasoning_summary_text.delta:rs_resp_up-1_0@0",
		"response.output_item.added:msg_resp_up-1_1@1",
		"response.content_part.added:msg_resp_up-1_1@1",
		"response.output_text.delta:msg_resp_up-1_1@1",
		"response.reasoning_summary_text.done:rs_resp_up-1_0@0",
		"response.reasoning_summary_part.done:rs_resp_up-1_0@0",
		"response.output_item.done:rs_resp_up-1_0@0",
		"response.output_text.done:msg_resp_up-1_1@1",
		"response.content_part.done:msg_resp_up-1_1@1",
		"response.output_item.done:msg_resp_up-1_1@1",
	}
	if got := itemTrace(t, frames); !slices.Equal(got, want) {
		t.Fatalf("item trace =\n%v\nwant\n%v", got, want)
	}
}

// TestResponsesClientStream_TextItems pins what each item ends up carrying: the
// closing events report the whole text the deltas built, not the last fragment.
func TestResponsesClientStream_TextItems(t *testing.T) {
	cases := []struct {
		name     string
		deltas   []map[string]any
		wantKind []string
		wantText []string
	}{
		{
			name:     "a single reasoning fragment",
			deltas:   []map[string]any{{"reasoning_content": "why"}},
			wantKind: []string{"reasoning"},
			wantText: []string{"why"},
		},
		{
			name:     "fragments accumulate into one item",
			deltas:   []map[string]any{{"content": "one "}, {"content": "two"}},
			wantKind: []string{"message"},
			wantText: []string{"one two"},
		},
		{
			name:     "reasoning and text in one delta are two items",
			deltas:   []map[string]any{{"reasoning_content": "why", "content": "because"}},
			wantKind: []string{"reasoning", "message"},
			wantText: []string{"why", "because"},
		},
		{
			name: "a later reasoning fragment joins the item still open",
			deltas: []map[string]any{
				{"reasoning_content": "first"}, {"content": "answer"},
				{"reasoning_content": "second"},
			},
			wantKind: []string{"reasoning", "message"},
			wantText: []string{"firstsecond", "answer"},
		},
		{
			name:     "a delta with no text reports no item",
			deltas:   []map[string]any{{"role": RoleAssistant}},
			wantKind: nil,
			wantText: nil,
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

			items := completedOutput(t, frames)
			kinds := make([]string, 0, len(items))
			texts := make([]string, 0, len(items))
			for _, item := range items {
				kind, _ := item["type"].(string)
				kinds = append(kinds, kind)
				texts = append(texts, itemText(t, item))
			}
			if !slices.Equal(kinds, tc.wantKind) {
				t.Fatalf("item kinds = %v, want %v", kinds, tc.wantKind)
			}
			if !slices.Equal(texts, tc.wantText) {
				t.Fatalf("item texts = %v, want %v", texts, tc.wantText)
			}
		})
	}
}

// TestResponsesClientStream_InlineReasoning pins the inline markup an upstream may
// wrap its thinking in: it is routed to the reasoning item, split across deltas
// when the tags are, and stripped from the answer either way.
func TestResponsesClientStream_InlineReasoning(t *testing.T) {
	cases := []struct {
		name     string
		deltas   []map[string]any
		wantKind []string
		wantText []string
	}{
		{
			name:     "both tags in one delta",
			deltas:   []map[string]any{{"content": "<think>why</think>because"}},
			wantKind: []string{"reasoning", "message"},
			wantText: []string{"why", "because"},
		},
		{
			name: "the tags split across deltas",
			deltas: []map[string]any{
				{"content": "<think>why"},
				{"content": "</think>because"},
			},
			wantKind: []string{"reasoning", "message"},
			wantText: []string{"why", "because"},
		},
		{
			name: "an opening tag with no closing one keeps thinking",
			deltas: []map[string]any{
				{"content": "<think>why"},
				{"content": " more"},
			},
			wantKind: []string{"reasoning"},
			wantText: []string{"why more"},
		},
		{
			name:     "a closing tag with no opening one is stripped as markup",
			deltas:   []map[string]any{{"content": "plain </think> text"}},
			wantKind: []string{"message"},
			wantText: []string{"plain  text"},
		},
		{
			name: "a second reasoning block opens a new item, the open message keeps its own",
			deltas: []map[string]any{
				{"content": "<think>first</think>answer"},
				{"content": "<think>second</think>more"},
			},
			wantKind: []string{"reasoning", "message", "reasoning"},
			wantText: []string{"first", "answermore", "second"},
		},
		{
			name:     "a delta carrying no markup is untouched",
			deltas:   []map[string]any{{"content": "a < b and c > d"}},
			wantKind: []string{"message"},
			wantText: []string{"a < b and c > d"},
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

			items := completedOutput(t, frames)
			kinds := make([]string, 0, len(items))
			texts := make([]string, 0, len(items))
			for _, item := range items {
				kind, _ := item["type"].(string)
				kinds = append(kinds, kind)
				texts = append(texts, itemText(t, item))
			}
			if !slices.Equal(kinds, tc.wantKind) {
				t.Fatalf("item kinds = %v, want %v", kinds, tc.wantKind)
			}
			if !slices.Equal(texts, tc.wantText) {
				t.Fatalf("item texts = %v, want %v", texts, tc.wantText)
			}
		})
	}
}

// TestResponsesClientStream_MessageItemShape pins the fields a client reads off a
// message item: the role, and a content part that reports the annotations and
// logprobs arrays it expects to find rather than omitting them.
func TestResponsesClientStream_MessageItemShape(t *testing.T) {
	state := NewResponsesStreamState("up-1", "pannelai-model", 1700000000)
	frames := feedClient(state, TargetOpenAI, openAIChunk(map[string]any{"content": "hi"}, ""))
	frames = append(frames, state.Finish()...)

	items := completedOutput(t, frames)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	item := items[0]
	if role, _ := item["role"].(string); role != RoleAssistant {
		t.Fatalf("role = %v, want %q", item["role"], RoleAssistant)
	}
	parts, _ := item["content"].([]any)
	if len(parts) != 1 {
		t.Fatalf("content parts = %v, want 1", item["content"])
	}
	part, _ := parts[0].(map[string]any)
	if part["type"] != "output_text" {
		t.Fatalf("part type = %v, want output_text", part["type"])
	}
	for _, member := range []string{"annotations", "logprobs"} {
		if _, ok := part[member].([]any); !ok {
			t.Fatalf("part %s = %v, want an array", member, part[member])
		}
	}
}
