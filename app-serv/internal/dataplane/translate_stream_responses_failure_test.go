// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_stream_responses_failure_test.go
// @for       Table-driven tests for how a Responses stream failure reaches a client.
// @uses      testing.
// @reason    SPEC-API-001 §10 makes the Responses API a P3 deliverable, and a
//
//	client already mid-stream has exactly one channel left for an
//	upstream failure: the content of a frame. Dropping the event would
//	leave the client waiting for an answer that already ended, so the
//	frame it becomes is pinned here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import "testing"

// TestResponsesStreamState_Failure pins how an upstream failure reaches a client
// that is already mid-stream: as content carrying the message, which is the only
// channel an open stream has. Dropping it would leave the client waiting for an
// answer that already ended.
func TestResponsesStreamState_Failure(t *testing.T) {
	cases := []struct {
		name  string
		event string
		want  string
	}{
		{
			name:  "a failed event surfaces its error message",
			event: `{"type":"response.failed","response":{"status":"failed","error":{"message":"model overloaded"}}}`,
			want:  "[Error] model overloaded",
		},
		{
			name:  "a top-level error event surfaces its message",
			event: `{"type":"error","error":{"message":"bad request"}}`,
			want:  "[Error] bad request",
		},
		{
			name:  "a failure with no message still tells the client it failed",
			event: `{"type":"response.failed","response":{"status":"failed"}}`,
			want:  "[Error] the upstream stream failed",
		},
		{
			name:  "a nested error on the response is read when the event has none",
			event: `{"type":"error","response":{"error":{"message":"nested"}}}`,
			want:  "[Error] nested",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := NewStreamState("", "m", 1700000000, false)
			frames := state.Frames(TargetResponses, []byte(tc.event))
			if len(frames) == 0 {
				t.Fatal("want a frame carrying the failure")
			}
			if got := frameDelta(t, frames[0])["content"]; got != tc.want {
				t.Fatalf("content = %v, want %q", got, tc.want)
			}
			finish := frameFinishReason(t, finishFrame(t, frames))
			if finish == nil || *finish != FinishStop {
				t.Fatalf("finish reason = %v, want a stop so the client closes the stream", finish)
			}
		})
	}
}

// TestResponsesStreamState_FailureClearsToolCall pins that a failure after a call
// opened does not report tool_calls: the answer failed, so there is no call for
// the client to dispatch.
func TestResponsesStreamState_FailureClearsToolCall(t *testing.T) {
	state := NewStreamState("", "m", 1700000000, false)
	state.Frames(TargetResponses, []byte(`{"type":"response.output_item.added","item":{"type":"function_call","call_id":"c1","name":"lookup"}}`))
	frames := state.Frames(TargetResponses, []byte(`{"type":"response.failed","response":{"error":{"message":"gone"}}}`))
	if len(frames) == 0 {
		t.Fatal("want a failure frame")
	}
	finish := frameFinishReason(t, finishFrame(t, frames))
	if finish == nil || *finish != FinishStop {
		t.Fatalf("finish reason = %v, want stop rather than tool_calls", finish)
	}
}
