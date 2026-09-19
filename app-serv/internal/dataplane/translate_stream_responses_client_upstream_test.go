// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_stream_responses_client_upstream_test.go
// @for       Table-driven tests for a Responses client stream served by a Claude
//
//	or a Responses upstream.
//
// @uses      testing, slices.
// @reason    SPEC-API-001 §7.15 serves POST /api/v1/responses from any resolved
//
//	provider, so the lifecycle has to come out the same whichever format
//	the upstream wrote. Both upstreams reach it through the chunk
//	vocabulary, which is the property these tests hold.
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

// claudeTextStream is one Anthropic answer: a prompt side reported once, a text
// delta, the output side reported cumulatively, and the terminator.
var claudeTextStream = []string{
	`{"type":"message_start","message":{"id":"msg_claude","usage":{"input_tokens":9,"output_tokens":1}}}`,
	`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`,
	`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3}}`,
	`{"type":"message_stop"}`,
}

// TestResponsesClientStream_ClaudeUpstream pins that an Anthropic provider reaches
// the same lifecycle: its events are mapped onto the chunk vocabulary the item
// mapping reads, and the id and accounting come from the Anthropic envelope rather
// than from the chunk wire's own placeholders.
func TestResponsesClientStream_ClaudeUpstream(t *testing.T) {
	state := NewResponsesStreamState("", "pannelai-model", 1700000000)
	frames := feedClient(state, TargetClaude, claudeTextStream...)
	frames = append(frames, state.Finish()...)

	want := []string{
		EventResponseCreated, EventResponseInProgress,
		EventResponseItemAdded, EventResponseContentPartAdded, EventResponseOutputTextDelta,
		EventResponseOutputTextDone, EventResponseContentPartDone, EventResponseItemDone,
		EventResponseCompleted, "[DONE]",
	}
	if got := frameLabels(frames); !slices.Equal(got, want) {
		t.Fatalf("event sequence = %v, want %v", got, want)
	}

	response, _ := eventBodyOf(t, frames[0])["response"].(map[string]any)
	if got, _ := response["id"].(string); got != "resp_msg_claude" {
		t.Fatalf("opening id = %q, want resp_msg_claude", got)
	}
	items := completedOutput(t, frames)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1: %v", len(items), items)
	}
	if got := itemText(t, items[0]); got != "hello" {
		t.Fatalf("item text = %q, want hello", got)
	}
}

// TestResponsesClientStream_ClaudeUsageIsMerged pins the accounting rule for an
// Anthropic upstream: the prompt side arrives once and the output side
// cumulatively, so the later block must not reset the prompt to zero.
func TestResponsesClientStream_ClaudeUsageIsMerged(t *testing.T) {
	state := NewResponsesStreamState("", "pannelai-model", 1700000000)
	// The accounting is settled by the deltas themselves, so the closing events
	// are not part of this case.
	feedClient(state, TargetClaude, claudeTextStream...)

	usage := state.Usage()
	if usage == nil {
		t.Fatal("the stream reported no usage")
	}
	if usage.PromptTokens != 9 || usage.CompletionTokens != 3 {
		t.Fatalf("usage = %+v, want 9 prompt and 3 completion", *usage)
	}
	if usage.TotalTokens != 12 {
		t.Fatalf("total = %d, want 12", usage.TotalTokens)
	}
}

// TestResponsesClientStream_ResponsesUpstream pins that a provider already
// speaking the Responses wire reaches the lifecycle too, and that its own response
// id is reported rather than a second one being minted for it.
func TestResponsesClientStream_ResponsesUpstream(t *testing.T) {
	state := NewResponsesStreamState("", "pannelai-model", 1700000000)
	frames := feedClient(state, TargetResponses,
		`{"type":"response.created","response":{"id":"resp_up","model":"upstream-model"}}`,
		`{"type":"response.output_text.delta","delta":"hi"}`,
		`{"type":"response.completed","response":{"id":"resp_up","usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}}`)
	frames = append(frames, state.Finish()...)

	want := []string{
		EventResponseCreated, EventResponseInProgress,
		EventResponseItemAdded, EventResponseContentPartAdded, EventResponseOutputTextDelta,
		EventResponseOutputTextDone, EventResponseContentPartDone, EventResponseItemDone,
		EventResponseCompleted, "[DONE]",
	}
	if got := frameLabels(frames); !slices.Equal(got, want) {
		t.Fatalf("event sequence = %v, want %v", got, want)
	}
	response, _ := eventBodyOf(t, frames[0])["response"].(map[string]any)
	if got, _ := response["id"].(string); got != "resp_up" {
		t.Fatalf("opening id = %q, want resp_up", got)
	}
	usage := state.Usage()
	if usage == nil || usage.PromptTokens != 3 || usage.CompletionTokens != 2 {
		t.Fatalf("usage = %+v, want 3 prompt and 2 completion", usage)
	}
}
