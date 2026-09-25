// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_stream_framing_test.go
// @for       The SSE shape of the frames the gateway builds itself: each one is
//
//	a complete event, the usage chunk is emitted once, and the finish
//	frame is emitted once (draft 021 F1, F2, F3).
//
// @uses      encoding/json, strings, testing.
// @reason    The gateway's own frames left without `data: ` and without the
//
//	blank-line terminator, so a client glued them onto the next frame and
//	never read a standalone `data: [DONE]` (F1); a null usage member
//	became a zero-valued usage chunk (F2); and the upstream's finish frame
//	was followed by a second, synthetic one (F3). These tests pin the
//	client-visible bytes, which is where the panel's reader measured the
//	truncated stream.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-25
package dataplane

import (
	"encoding/json"
	"strings"
	"testing"
)

// assertSSEStream fails unless rendered is a sequence of complete SSE events:
// every event is `data: <one json object>` or the terminal marker, and every
// event ends at the blank line a client flushes on.
func assertSSEStream(t *testing.T, rendered string) {
	t.Helper()
	if !strings.HasSuffix(rendered, "\n\n") {
		t.Fatalf("the stream does not end at an event terminator: %q", rendered)
	}
	for index, event := range strings.Split(strings.TrimSuffix(rendered, "\n\n"), "\n\n") {
		if event == "data: [DONE]" {
			continue
		}
		payload, found := strings.CutPrefix(event, "data: ")
		if !found {
			t.Fatalf("event %d is not SSE-framed: %q", index, event)
		}
		var document map[string]json.RawMessage
		if err := json.Unmarshal([]byte(payload), &document); err != nil {
			t.Fatalf("event %d is not exactly one JSON object: %q (%v)", index, payload, err)
		}
	}
}

// TestClaudeUpstreamToOpenAIClientStaysSingleFramed is the guard for the framing
// change: the Anthropic-upstream path builds the same chunks and frames them at
// its own call site, so a frame added inside chunk() would reach that client as
// `data: data: ...` and break its reader.
func TestClaudeUpstreamToOpenAIClientStaysSingleFramed(t *testing.T) {
	state := NewStreamState("", "model-x", 1700000000, false)
	frames := state.Frames(TargetClaude,
		[]byte(`{"type":"message_start","message":{"id":"m","usage":{"input_tokens":5,"output_tokens":0}}}`))
	frames = append(frames, state.Frames(TargetClaude,
		[]byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"pong"}}`))...)
	rendered := frameText(frames)

	if strings.Contains(rendered, "data: data:") {
		t.Fatalf("a chunk was framed twice: %q", rendered)
	}
	assertSSEStream(t, rendered)
	if !strings.Contains(rendered, `"content":"pong"`) {
		t.Fatalf("the text delta is missing from the re-framed stream: %q", rendered)
	}
}

// TestOpenAIStream_GatewayFramesAreSSEFramed pins F1 and F3 together on the shape
// the OpenCode free tier sends: the upstream's finish frame is the only finish
// frame, and the synthetic one the gateway adds when the upstream never finished
// is framed like every other event.
func TestOpenAIStream_GatewayFramesAreSSEFramed(t *testing.T) {
	state := NewStreamState("", "model-x", 1700000000, false)
	var frames [][]byte
	for _, payload := range []string{
		`{"id":"up-1","object":"chat.completion.chunk","created":1,"model":"model-x",` +
			`"choices":[{"index":0,"delta":{"content":"pong"},"finish_reason":null}]}`,
		`{"id":"up-1","object":"chat.completion.chunk","created":1,"model":"model-x",` +
			`"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
	} {
		frames = append(frames, state.Frames(TargetOpenAI, []byte(payload))...)
	}
	rendered := frameText(append(frames, state.Finish()...))

	assertSSEStream(t, rendered)
	if got := strings.Count(rendered, `"finish_reason":"stop"`); got != 1 {
		t.Fatalf("finish frames = %d, want the upstream's one: %s", got, rendered)
	}
	if !strings.HasSuffix(rendered, "data: [DONE]\n\n") {
		t.Fatalf("the stream must end with the terminal marker as its own event: %q", rendered)
	}
}

// TestOpenAIStream_SyntheticFinishIsFramed pins the other half of F1: a stream
// whose upstream never reported a finish still closes with a framed finish event,
// and the terminal marker stays its own event rather than gluing to the JSON.
func TestOpenAIStream_SyntheticFinishIsFramed(t *testing.T) {
	state := NewStreamState("", "model-x", 1700000000, false)
	frames := state.Frames(TargetOpenAI,
		[]byte(`{"id":"up-1","choices":[{"index":0,"delta":{"content":"pong"},"finish_reason":null}]}`))
	rendered := frameText(append(frames, state.Finish()...))

	assertSSEStream(t, rendered)
	if got := strings.Count(rendered, `"finish_reason":"stop"`); got != 1 {
		t.Fatalf("synthetic finish frames = %d, want 1: %s", got, rendered)
	}
}

// TestOpenAIStream_UsageChunkIsEmittedOnce pins F2 for the shape where the
// upstream states its numbers on the finish frame: the client sees the upstream's
// frame and exactly one usage chunk the gateway adds, never a second copy.
func TestOpenAIStream_UsageChunkIsEmittedOnce(t *testing.T) {
	state := NewStreamState("", "model-x", 1700000000, true)
	var frames [][]byte
	for _, payload := range []string{
		`{"id":"up-1","choices":[{"index":0,"delta":{"content":"pong"},"finish_reason":null}]}`,
		`{"id":"up-1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],` +
			`"usage":{"prompt_tokens":7,"completion_tokens":2,"total_tokens":9}}`,
	} {
		frames = append(frames, state.Frames(TargetOpenAI, []byte(payload))...)
	}
	rendered := frameText(append(frames, state.Finish()...))

	assertSSEStream(t, rendered)
	// Two usage objects reach the client, both the upstream's numbers: the frame
	// the upstream sent, and the single usage chunk the gateway adds for a client
	// that asked for one.
	if got := strings.Count(rendered, `"total_tokens"`); got != 2 {
		t.Fatalf("usage objects = %d, want the upstream's frame plus one chunk: %s", got, rendered)
	}
	if !strings.Contains(rendered, `"choices":[]`) {
		t.Fatalf("the usage chunk must carry an empty choices array: %s", rendered)
	}
}

// TestOpenAIStream_NullUsageNeverBecomesAZeroChunk pins F2's other half on the
// shape the free tier actually sent: the finish frame marks `usage` null and the
// numbers arrive in a later frame, so the gateway must not read the null as a
// zero-valued usage and publish a 0/0 chunk before the real numbers land.
func TestOpenAIStream_NullUsageNeverBecomesAZeroChunk(t *testing.T) {
	state := NewStreamState("", "model-x", 1700000000, true)
	var frames [][]byte
	for _, payload := range []string{
		`{"id":"up-1","choices":[{"index":0,"delta":{"content":"pong"},"finish_reason":null}]}`,
		`{"id":"up-1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":null}`,
		`{"id":"up-1","choices":[],"usage":{"prompt_tokens":7,"completion_tokens":2,"total_tokens":9}}`,
	} {
		frames = append(frames, state.Frames(TargetOpenAI, []byte(payload))...)
	}
	rendered := frameText(append(frames, state.Finish()...))

	assertSSEStream(t, rendered)
	if got := strings.Count(rendered, `"prompt_tokens":0`); got != 0 {
		t.Fatalf("a null usage became a zero usage chunk (%d): %s", got, rendered)
	}
	if got := strings.Count(rendered, `"total_tokens"`); got != 2 {
		t.Fatalf("usage objects = %d, want the upstream's frame plus one chunk: %s", got, rendered)
	}
	if !strings.Contains(rendered, `"prompt_tokens":7`) {
		t.Fatalf("the usage chunk must carry the numbers the upstream reported: %s", rendered)
	}
}
