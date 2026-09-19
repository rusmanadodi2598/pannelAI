// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_stream_responses_client_fixtures_test.go
// @for       Shared helpers for the Responses client stream tests.
// @uses      testing, encoding/json, strconv, strings.
// @reason    SPEC-API-001 §7.15 serves POST /api/v1/responses, whose stream is a
//
//	named-event stream. Reading a named event needs its own unwrapper, and
//	every table in the sibling test files builds the same chunks, so both
//	live here rather than being restated per file.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// eventBodyOf unwraps one named SSE frame into the object it carries.
func eventBodyOf(t *testing.T, frame []byte) map[string]any {
	t.Helper()
	text := string(frame)
	const marker = "\ndata: "
	start := strings.Index(text, marker)
	if start < 0 {
		t.Fatalf("frame carries no data line: %q", frame)
	}
	payload := strings.TrimSuffix(text[start+len(marker):], "\n\n")
	var decoded map[string]any
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("decoding frame %q: %v", frame, err)
	}
	return decoded
}

// feedClient runs payloads through one client stream state and returns every
// frame it produced, which is what a lifecycle test compares.
func feedClient(state *ResponsesStreamState, target string, payloads ...string) [][]byte {
	frames := make([][]byte, 0, len(payloads))
	for _, payload := range payloads {
		frames = append(frames, state.Frames(target, []byte(payload))...)
	}
	return frames
}

// frameLabels lists what each frame is: its event name, or the terminal marker.
func frameLabels(frames [][]byte) []string {
	labels := make([]string, 0, len(frames))
	for _, frame := range frames {
		if string(frame) == SSEDone {
			labels = append(labels, "[DONE]")
			continue
		}
		labels = append(labels, eventNameOf(frame))
	}
	return labels
}

// openAIChunk renders one upstream OpenAI chunk carrying the given delta.
func openAIChunk(delta map[string]any, finishReason string) string {
	choice := map[string]any{"index": 0, "delta": delta, "finish_reason": nil}
	if finishReason != "" {
		choice["finish_reason"] = finishReason
	}
	return string(mustJSON(map[string]any{
		"id": "chatcmpl-up", "object": "chat.completion.chunk",
		"model": "upstream-model", "choices": []any{choice},
	}))
}

// usageOnlyChunk renders the frame stream_options.include_usage produces, which
// carries accounting and no choice.
func usageOnlyChunk(prompt, completion int) string {
	return string(mustJSON(map[string]any{
		"id": "chatcmpl-up", "object": "chat.completion.chunk",
		"model": "upstream-model", "choices": []any{},
		"usage": map[string]any{
			"prompt_tokens": prompt, "completion_tokens": completion,
			"total_tokens": prompt + completion,
		},
	}))
}

// itemTrace lists the item-level events as "name:item_id@index", so a table can
// compare an item's structure without restating every field it carries.
func itemTrace(t *testing.T, frames [][]byte) []string {
	t.Helper()
	trace := make([]string, 0, len(frames))
	for _, frame := range frames {
		name := eventNameOf(frame)
		switch name {
		case "", EventResponseCreated, EventResponseInProgress, EventResponseCompleted:
			continue
		}
		body := eventBodyOf(t, frame)
		id, _ := body["item_id"].(string)
		if id == "" {
			if item, ok := body["item"].(map[string]any); ok {
				id, _ = item["id"].(string)
			}
		}
		index, _ := body["output_index"].(float64)
		trace = append(trace, name+":"+id+"@"+strconv.Itoa(int(index)))
	}
	return trace
}

// completedOutput reads the items the closing event reports, which is the
// assembled answer a client that never reads the deltas would see.
func completedOutput(t *testing.T, frames [][]byte) []map[string]any {
	t.Helper()
	for _, frame := range frames {
		if eventNameOf(frame) != EventResponseCompleted {
			continue
		}
		response, ok := eventBodyOf(t, frame)["response"].(map[string]any)
		if !ok {
			t.Fatalf("closing event carries no response object: %s", frame)
		}
		raw, _ := response["output"].([]any)
		items := make([]map[string]any, 0, len(raw))
		for _, entry := range raw {
			item, ok := entry.(map[string]any)
			if !ok {
				t.Fatalf("output entry is not an object: %v", entry)
			}
			items = append(items, item)
		}
		return items
	}
	t.Fatalf("no closing event in %v", frameLabels(frames))
	return nil
}

// itemText reads the text an output item carries: a reasoning item's first
// summary part, or a message item's first content part.
func itemText(t *testing.T, item map[string]any) string {
	t.Helper()
	parts, ok := item["summary"].([]any)
	if !ok {
		parts, ok = item["content"].([]any)
	}
	if !ok || len(parts) == 0 {
		t.Fatalf("item %v carries no part", item["id"])
	}
	part, _ := parts[0].(map[string]any)
	text, _ := part["text"].(string)
	return text
}
