// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_stream_responses_test.go
// @for       Table-driven tests for mapping Responses SSE events onto frames.
// @uses      testing, encoding/json, strings.
// @reason    SPEC-API-001 §10 makes the Responses API a P3 deliverable, and its
//
//	stream is a named-event stream rather than a chunk stream. The
//	mapping is what a streaming client sees, and an event that carries
//	nothing must produce no frame at all: an empty frame makes a client
//	that counts frames mis-count the answer.
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

// decodeFrame unwraps one SSE data frame into the object it carries.
func decodeFrame(t *testing.T, frame []byte) map[string]any {
	t.Helper()
	text := strings.TrimSuffix(strings.TrimPrefix(string(frame), "data: "), "\n\n")
	var decoded map[string]any
	if err := json.Unmarshal([]byte(text), &decoded); err != nil {
		t.Fatalf("decoding frame %q: %v", frame, err)
	}
	return decoded
}

// frameDelta reads the first choice's delta object from a frame.
func frameDelta(t *testing.T, frame []byte) map[string]any {
	t.Helper()
	choices, ok := decodeFrame(t, frame)["choices"].([]any)
	if !ok || len(choices) == 0 {
		t.Fatalf("frame has no choices: %s", frame)
	}
	choice, _ := choices[0].(map[string]any)
	delta, _ := choice["delta"].(map[string]any)
	if delta == nil {
		t.Fatalf("frame choice has no delta object: %s", frame)
	}
	return delta
}

// TestResponsesStreamState_TextMapping pins which events become a frame and
// which do not. Every event the gateway has no reader for is deliberately
// silent, because a client showing an empty delta is worse than showing nothing.
func TestResponsesStreamState_TextMapping(t *testing.T) {
	cases := []struct {
		name       string
		event      string
		wantFrames int
		wantKey    string
		wantValue  string
	}{
		{
			name:       "an output text delta becomes a content delta",
			event:      `{"type":"response.output_text.delta","delta":"hello"}`,
			wantFrames: 1,
			wantKey:    "content",
			wantValue:  "hello",
		},
		{
			name:       "a reasoning delta becomes reasoning content",
			event:      `{"type":"response.reasoning_summary_text.delta","delta":"why"}`,
			wantFrames: 1,
			wantKey:    "reasoning_content",
			wantValue:  "why",
		},
		{
			name:       "an empty text delta produces no frame",
			event:      `{"type":"response.output_text.delta","delta":""}`,
			wantFrames: 0,
		},
		{
			name:       "an empty reasoning delta produces no frame",
			event:      `{"type":"response.reasoning_summary_text.delta","delta":""}`,
			wantFrames: 0,
		},
		{
			name:       "the created event produces no frame",
			event:      `{"type":"response.created","response":{"id":"resp_1"}}`,
			wantFrames: 0,
		},
		{
			name:       "a message item being added produces no frame",
			event:      `{"type":"response.output_item.added","item":{"type":"message","role":"assistant"}}`,
			wantFrames: 0,
		},
		{
			name:       "a message item being done produces no frame",
			event:      `{"type":"response.output_item.done","item":{"type":"message","role":"assistant"}}`,
			wantFrames: 0,
		},
		{
			name:       "an event the gateway does not model produces no frame",
			event:      `{"type":"response.web_search_call.in_progress"}`,
			wantFrames: 0,
		},
		{
			name:       "a payload that is not an object produces no frame",
			event:      `[1,2,3]`,
			wantFrames: 0,
		},
		{
			name:       "malformed json produces no frame rather than a panic",
			event:      `{"type":`,
			wantFrames: 0,
		},
		{
			name:       "an empty payload produces no frame",
			event:      ``,
			wantFrames: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := NewStreamState("", "m", 1700000000, false)
			frames := state.Frames(TargetResponses, []byte(tc.event))
			if len(frames) != tc.wantFrames {
				t.Fatalf("frames = %d, want %d (%q)", len(frames), tc.wantFrames, frameText(frames))
			}
			if tc.wantFrames == 0 {
				return
			}
			if got := frameDelta(t, frames[0])[tc.wantKey]; got != tc.wantValue {
				t.Fatalf("delta[%q] = %v, want %q", tc.wantKey, got, tc.wantValue)
			}
			if reason := frameFinishReason(t, frames[0]); reason != nil {
				t.Fatalf("mid-stream finish_reason = %v, want null", *reason)
			}
		})
	}
}

// frameFinishReason reads a frame's first-choice finish reason, or nil when the
// frame carries none: a mid-stream frame renders it as null, and a usage chunk
// carries no choices at all.
func frameFinishReason(t *testing.T, frame []byte) *string {
	t.Helper()
	choices, ok := decodeFrame(t, frame)["choices"].([]any)
	if !ok || len(choices) == 0 {
		return nil
	}
	choice, _ := choices[0].(map[string]any)
	raw, ok := choice["finish_reason"]
	if !ok || raw == nil {
		return nil
	}
	reason, ok := raw.(string)
	if !ok {
		t.Fatalf("finish_reason is not a string: %v", raw)
	}
	return &reason
}

// finishFrame returns the frame that closes an answer, which is the one carrying
// a finish reason. Reading a position instead would break as soon as an upstream
// reports accounting, since the usage chunk is appended after the finish.
func finishFrame(t *testing.T, frames [][]byte) []byte {
	t.Helper()
	for _, frame := range frames {
		if frameFinishReason(t, frame) != nil {
			return frame
		}
	}
	t.Fatalf("no frame carries a finish reason: %q", frameText(frames))
	return nil
}

// TestResponsesStreamState_Identity pins that the response id is taken from
// whichever event carries one, so every frame a client sees reports the same
// identifier even when the first events omit it.
func TestResponsesStreamState_Identity(t *testing.T) {
	cases := []struct {
		name     string
		events   []string
		wantID   string
		wantLast string
	}{
		{
			name:     "an id on the created event is reported on later frames",
			events:   []string{`{"type":"response.created","response":{"id":"resp_abc"}}`, `{"type":"response.output_text.delta","delta":"x"}`},
			wantID:   "resp_abc",
			wantLast: "resp_abc",
		},
		{
			name:     "an id that arrives late is picked up",
			events:   []string{`{"type":"response.output_text.delta","delta":"x"}`, `{"type":"response.completed","response":{"id":"resp_late"}}`},
			wantID:   "resp_late",
			wantLast: "resp_late",
		},
		{
			name:     "a stream that never reports an id leaves the state unset",
			events:   []string{`{"type":"response.output_text.delta","delta":"x"}`},
			wantID:   "",
			wantLast: "chatcmpl-pannelai",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := NewStreamState("", "m", 1700000000, false)
			var last [][]byte
			for _, event := range tc.events {
				last = state.Frames(TargetResponses, []byte(event))
			}
			if state.ID != tc.wantID {
				t.Fatalf("state id = %q, want %q", state.ID, tc.wantID)
			}
			if len(last) == 0 {
				t.Fatal("want a final frame to read the id from")
			}
			if id, _ := decodeFrame(t, last[0])["id"].(string); id != tc.wantLast {
				t.Fatalf("frame id = %q, want %q", id, tc.wantLast)
			}
		})
	}
}
