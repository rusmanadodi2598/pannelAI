// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_stream_responses_ceiling_test.go
// @for       The output-ceiling signal crossing the wire translators in both directions of a stream.
// @uses      encoding/json, strings, testing, internal/schema.
// @reason    An upstream that ran out of output ceiling says so, and a coding client that reads a normal stop instead waits for the user to type "continue" (draft 054 F1, F2). The fold already reported the ceiling honestly; these tests pin that the stream now reports the same fact on both sides of the translator, so the two paths cannot drift back apart.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package dataplane

import (
	"strings"
	"testing"
)

// TestResponsesStreamCeiling_SourceIncompleteReachesTheClientAsALength pins F1:
// a Responses stream whose terminal event is `response.incomplete` must close the
// client's chunk stream with `length`, which is the same answer the fold gives a
// non-streaming client for the same upstream event.
func TestResponsesStreamCeiling_SourceIncompleteReachesTheClientAsALength(t *testing.T) {
	cases := []struct {
		name       string
		events     []string
		wantFinish string
	}{
		{
			name: "a ceiling stop on a text answer",
			events: []string{
				`{"type":"response.output_text.delta","delta":"half an"}`,
				`{"type":"response.incomplete","response":{"id":"resp_1","status":"incomplete",` +
					`"incomplete_details":{"reason":"max_output_tokens"}}}`,
			},
			wantFinish: FinishLength,
		},
		{
			name: "an incomplete that names no ceiling reason",
			events: []string{
				`{"type":"response.incomplete","response":{"id":"resp_2","status":"incomplete",` +
					`"incomplete_details":{"reason":"content_filter"}}}`,
			},
			wantFinish: FinishStop,
		},
		{
			// Measured live 2026-10-16 on opencode/muse-spark-1.3-contributor-free:
			// at ceilings of 512 and 1024 the answer carried a call name and no
			// arguments, and the client was told `tool_calls`. A dispatchable call
			// is the one thing that stop was not, so the ceiling is read first and
			// the client is told to ask again rather than handed a half call.
			name: "a ceiling stop while a tool call is open reports the cut",
			events: []string{
				`{"type":"response.output_item.added","item":{"type":"function_call","call_id":"call_1","name":"read"}}`,
				`{"type":"response.function_call_arguments.delta","delta":"{\"path\""}`,
				`{"type":"response.incomplete","response":{"id":"resp_3","status":"incomplete",` +
					`"incomplete_details":{"reason":"max_output_tokens"}}}`,
			},
			wantFinish: FinishLength,
		},
		{
			// The carve-out draft 055 asked for: a call the answer closed before the
			// ceiling stop is real, and reporting it as a cut would throw away a
			// dispatchable request.
			name: "a ceiling stop after a closed call keeps reporting the call",
			events: []string{
				`{"type":"response.output_item.added","item":{"type":"function_call","call_id":"call_1","name":"read"}}`,
				`{"type":"response.function_call_arguments.delta","delta":"{\"path\":\"a.go\"}"}`,
				`{"type":"response.output_item.done","item":{"type":"function_call","call_id":"call_1",` +
					`"name":"read","arguments":"{\"path\":\"a.go\"}"}}`,
				`{"type":"response.incomplete","response":{"id":"resp_4","status":"incomplete",` +
					`"incomplete_details":{"reason":"max_output_tokens"}}}`,
			},
			wantFinish: FinishToolCalls,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := NewStreamState("", "opencode/muse-spark-1.3-contributor-free", 1700000000, false)
			var frames [][]byte
			for _, event := range tc.events {
				frames = append(frames, state.Frames(TargetResponses, []byte(event))...)
			}
			frames = append(frames, state.Finish()...)

			finish := frameFinishReason(t, finishFrame(t, frames))
			if finish == nil || *finish != tc.wantFinish {
				t.Fatalf("finish reason = %v, want %q (%s)", finish, tc.wantFinish, frameText(frames))
			}
		})
	}
}

// TestResponsesClientStreamCeiling_LengthReachesTheClientAsAnIncomplete pins F2,
// the mirror: an upstream that stopped with `length` must close a Responses client
// with `response.incomplete` carrying the ceiling reason, not with the
// `response.completed` every answer used to get.
func TestResponsesClientStreamCeiling_LengthReachesTheClientAsAnIncomplete(t *testing.T) {
	cases := []struct {
		name       string
		finish     string
		wantEvent  string
		wantStatus string
		wantReason string
	}{
		{
			name: "a ceiling stop", finish: FinishLength,
			wantEvent: EventResponseIncomplete, wantStatus: "incomplete", wantReason: ResponsesCeilingReason,
		},
		{
			name: "an ordinary stop", finish: FinishStop,
			wantEvent: EventResponseCompleted, wantStatus: "completed", wantReason: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := NewResponsesStreamState("up-1", "pannelai-model", 1700000000)
			frames := feedClient(state, TargetOpenAI,
				openAIChunk(map[string]any{"content": "half an"}, ""),
				openAIChunk(map[string]any{}, tc.finish))
			frames = append(frames, state.Finish()...)

			labels := frameLabels(frames)
			if got := countOf(labels, tc.wantEvent); got != 1 {
				t.Fatalf("%s emitted %d times, want 1: %v", tc.wantEvent, got, labels)
			}
			if tc.wantEvent != EventResponseIncomplete {
				return
			}
			// A client that reads only the closing event must still learn the answer
			// was cut, so the status and the reason travel inside it.
			terminal := terminalResponse(t, frames, EventResponseIncomplete)
			if status := stringField(terminal, "status"); status != tc.wantStatus {
				t.Fatalf("status = %q, want %q", status, tc.wantStatus)
			}
			details, ok := objectField(terminal, "incomplete_details")
			if !ok {
				t.Fatalf("the incomplete event carries no incomplete_details: %s", mustJSON(terminal))
			}
			if reason := stringField(details, "reason"); reason != tc.wantReason {
				t.Fatalf("incomplete_details.reason = %q, want %q", reason, tc.wantReason)
			}
			if got := countOf(labels, EventResponseCompleted); got != 0 {
				t.Fatalf("a cut answer also reported %d completed events: %v", got, labels)
			}
		})
	}
}

// terminalResponse reads the response object one named event carried.
func terminalResponse(t *testing.T, frames [][]byte, name string) object {
	t.Helper()
	for _, frame := range frames {
		if !strings.Contains(string(frame), "event: "+name+"\n") {
			continue
		}
		payload, ok := decodeObject(sseData(frame))
		if !ok {
			continue
		}
		if response, found := objectField(payload, "response"); found {
			return response
		}
	}
	t.Fatalf("no %s event carrying a response among %v", name, frameLabels(frames))
	return nil
}
