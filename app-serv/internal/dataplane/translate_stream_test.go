// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_stream_test.go
// @for       Tests for the SSE frame shape, the usage chunk, and retry decisions.
// @uses      testing, internal/schema, net/http.
// @reason    SPEC-API-001 §4 fixes both the SSE frame shape and the usage chunk
//
//	stream_options.include_usage asks for, and §4 also fixes the retry
//	policy per status. Both are pure functions of their inputs, so they are
//	pinned here with no network and no clock (AGENTS.md §2.1).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-17
package dataplane

import (
	"strings"
	"testing"
)

// frameText renders the frames a test compares against, so a failure shows the
// exact bytes a client would receive.
func frameText(frames [][]byte) string {
	parts := make([]string, 0, len(frames))
	for _, frame := range frames {
		parts = append(parts, string(frame))
	}
	return strings.Join(parts, "")
}

// TestFrameShape pins the SSE framing: `data: <json>\n\n`, and the named-event
// form Anthropic uses. The blank line is the event terminator, so a frame without
// it never flushes at a client.
func TestFrameShape(t *testing.T) {
	cases := []struct {
		name string
		got  []byte
		want string
	}{
		{name: "a data frame ends with a blank line", got: Frame([]byte(`{"a":1}`)), want: "data: {\"a\":1}\n\n"},
		{name: "an empty payload still frames", got: Frame(nil), want: "data: \n\n"},
		{name: "a named event carries its name first", got: EventFrame("message_delta", []byte(`{"t":1}`)), want: "event: message_delta\ndata: {\"t\":1}\n\n"},
		{name: "the terminal marker is the documented constant", got: []byte(SSEDone), want: "data: [DONE]\n\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.got) != tc.want {
				t.Fatalf("frame = %q, want %q", tc.got, tc.want)
			}
		})
	}
}

// TestStreamState_UsageChunk pins the usage-chunk rule: it is emitted only when the
// client asked for it, it is a frame of its own with an empty choices array, and
// the terminal marker always closes the stream.
func TestStreamState_UsageChunk(t *testing.T) {
	cases := []struct {
		name         string
		includeUsage bool
		events       []string
		wantUsage    bool
		wantDone     bool
	}{
		{
			name:         "a client that asked for usage gets a usage frame",
			includeUsage: true,
			events: []string{
				`{"type":"message_start","message":{"id":"m","usage":{"input_tokens":5,"output_tokens":0}}}`,
				`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":5,"output_tokens":9}}`,
			},
			wantUsage: true, wantDone: true,
		},
		{
			name:         "a client that did not ask gets no usage frame",
			includeUsage: false,
			events: []string{
				`{"type":"message_start","message":{"id":"m","usage":{"input_tokens":5,"output_tokens":0}}}`,
				`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":5,"output_tokens":9}}`,
			},
			wantUsage: false, wantDone: true,
		},
		{
			name:         "an upstream that reported nothing emits no fabricated usage",
			includeUsage: true,
			events:       []string{`{"type":"message_start","message":{"id":"m"}}`},
			wantUsage:    false, wantDone: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := NewStreamState("", "model-x", 1700000000, tc.includeUsage)
			for _, event := range tc.events {
				state.Frames(TargetClaude, []byte(event))
			}
			rendered := frameText(state.Finish())
			hasUsage := strings.Contains(rendered, `"usage"`) && strings.Contains(rendered, `"prompt_tokens"`)
			if hasUsage != tc.wantUsage {
				t.Fatalf("usage frame present = %v, want %v (frames: %s)", hasUsage, tc.wantUsage, rendered)
			}
			if got := strings.Contains(rendered, "[DONE]"); got != tc.wantDone {
				t.Fatalf("[DONE] present = %v, want %v", got, tc.wantDone)
			}
			if tc.wantUsage && !strings.Contains(rendered, `"choices":[]`) {
				t.Fatalf("usage frame must carry an empty choices array: %s", rendered)
			}
		})
	}
}

// TestStreamState_FinishIsIdempotent pins that a stream which already emitted its
// finish frame does not emit a second one, which a client counting frames would
// read as a second answer.
func TestStreamState_FinishIsIdempotent(t *testing.T) {
	state := NewStreamState("", "m", 1, false)
	var firstFrames [][]byte
	for _, payload := range []string{
		`{"type":"message_start","message":{"id":"m"}}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`,
	} {
		firstFrames = append(firstFrames, state.Frames(TargetClaude, []byte(payload))...)
	}
	// The finish frame rides on the message_delta that carried the stop reason,
	// so the first pass is Frames plus Finish: Finish terminates the stream, it
	// does not finish it.
	firstFrames = append(firstFrames, state.Finish()...)
	first := frameText(firstFrames)

	second := frameText(state.Finish())
	if got := strings.Count(first, `"finish_reason":"stop"`); got != 1 {
		t.Fatalf("first pass emitted %d finish frames, want 1: %s", got, first)
	}
	if strings.Contains(second, `"finish_reason":"stop"`) {
		t.Fatalf("second Finish emitted a finish frame again: %s", second)
	}
	if !strings.Contains(second, "[DONE]") {
		t.Fatalf("second Finish must still terminate the stream: %s", second)
	}
}

// eventNameOf reads the event name from a rendered frame.
func eventNameOf(frame []byte) string {
	const prefix = "event: "
	text := string(frame)
	if !strings.HasPrefix(text, prefix) {
		return ""
	}
	end := strings.IndexByte(text, '\n')
	if end < 0 {
		return ""
	}
	return text[len(prefix):end]
}

// countOf counts occurrences of a value.
func countOf(values []string, want string) int {
	total := 0
	for _, value := range values {
		if value == want {
			total++
		}
	}
	return total
}
