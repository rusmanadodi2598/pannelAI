// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_stream_responses_client_test.go
// @for       Table-driven tests for the Responses client stream lifecycle.
// @uses      testing, slices.
// @reason    SPEC-API-001 §7.15 serves POST /api/v1/responses, whose stream is a
//
//	named-event stream: a client dispatches on the event name and on the
//	sequence number, so a missing, repeated, or misordered event is a
//	broken answer even when every payload is right.
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

// TestResponsesClientStream_Lifecycle pins the event sequence a client sees: one
// opening pair, the item events, one closing event, and the terminal marker.
func TestResponsesClientStream_Lifecycle(t *testing.T) {
	cases := []struct {
		name     string
		payloads []string
		want     []string
	}{
		{
			name:     "a stream that produced nothing still opens and closes",
			payloads: nil,
			want: []string{
				EventResponseCreated, EventResponseInProgress,
				EventResponseCompleted, "[DONE]",
			},
		},
		{
			name: "a text answer opens and closes every item it reports",
			payloads: []string{
				openAIChunk(map[string]any{"role": RoleAssistant}, ""),
				openAIChunk(map[string]any{"content": "hello"}, ""),
				openAIChunk(map[string]any{}, FinishStop),
			},
			want: []string{
				EventResponseCreated, EventResponseInProgress,
				EventResponseItemAdded, EventResponseContentPartAdded, EventResponseOutputTextDelta,
				EventResponseOutputTextDone, EventResponseContentPartDone, EventResponseItemDone,
				EventResponseCompleted, "[DONE]",
			},
		},
		{
			name:     "an opening frame carrying only a role reports no item",
			payloads: []string{openAIChunk(map[string]any{"role": RoleAssistant}, "")},
			want: []string{
				EventResponseCreated, EventResponseInProgress,
				EventResponseCompleted, "[DONE]",
			},
		},
		{
			name:     "a usage-only frame reports accounting without an item",
			payloads: []string{usageOnlyChunk(11, 4)},
			want: []string{
				EventResponseCreated, EventResponseInProgress,
				EventResponseCompleted, "[DONE]",
			},
		},
		{
			name:     "an unreadable payload produces no frame at all",
			payloads: []string{"not json"},
			want: []string{
				EventResponseCreated, EventResponseInProgress,
				EventResponseCompleted, "[DONE]",
			},
		},
		{
			name: "two text fragments produce one item and two deltas",
			payloads: []string{
				openAIChunk(map[string]any{"content": "a"}, ""),
				openAIChunk(map[string]any{"content": "b"}, ""),
			},
			want: []string{
				EventResponseCreated, EventResponseInProgress,
				EventResponseItemAdded, EventResponseContentPartAdded, EventResponseOutputTextDelta,
				EventResponseOutputTextDelta,
				EventResponseOutputTextDone, EventResponseContentPartDone, EventResponseItemDone,
				EventResponseCompleted, "[DONE]",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := NewResponsesStreamState("", "pannelai-model", 1700000000)
			frames := feedClient(state, TargetOpenAI, tc.payloads...)
			frames = append(frames, state.Finish()...)
			if got := frameLabels(frames); !slices.Equal(got, tc.want) {
				t.Fatalf("event sequence = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestResponsesClientStream_SequenceNumbers pins the one property the API states
// about numbering: it increases by one across the stream, with no gap and no
// repeat, because a client uses it to order events it receives out of order.
func TestResponsesClientStream_SequenceNumbers(t *testing.T) {
	state := NewResponsesStreamState("up-1", "pannelai-model", 1700000000)
	frames := feedClient(state, TargetOpenAI,
		openAIChunk(map[string]any{"reasoning_content": "why"}, ""),
		openAIChunk(map[string]any{"content": "hi"}, ""),
		usageOnlyChunk(11, 4),
		openAIChunk(map[string]any{}, FinishStop))
	frames = append(frames, state.Finish()...)

	previous := 0
	for _, frame := range frames {
		if string(frame) == SSEDone {
			continue
		}
		sequence, ok := eventBodyOf(t, frame)["sequence_number"].(float64)
		if !ok {
			t.Fatalf("event %s carries no sequence_number: %s", eventNameOf(frame), frame)
		}
		if int(sequence) != previous+1 {
			t.Fatalf("sequence_number = %d after %d, want %d", int(sequence), previous, previous+1)
		}
		previous = int(sequence)
	}
	if previous == 0 {
		t.Fatal("no numbered event was emitted")
	}
}
