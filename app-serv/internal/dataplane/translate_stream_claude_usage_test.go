// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_stream_claude_usage_test.go
// @for       The accounting a Claude-to-Claude passthrough stream reports when the numbers arrive in two events.
// @uses      testing, internal/schema (the usage shape the stream reports).
// @reason    Anthropic states the prompt once on message_start and the output cumulatively on message_delta, so a passthrough that reads only the top level and then overwrites reports a prompt of zero and bills the call short.
//
//	The fold rule is already pinned for the translating direction; this pins it for the forwarded one.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04
package dataplane

import (
	"testing"
)

const claudeMessageStart = `{"type":"message_start","message":{"id":"msg_1","role":"assistant",` +
	`"usage":{"input_tokens":100,"output_tokens":1}}}`

const claudeTextDelta = `{"type":"content_block_delta","index":0,` +
	`"delta":{"type":"text_delta","text":"hi"}}`

const claudeMessageDelta = `{"type":"message_delta","delta":{"stop_reason":"end_turn"},` +
	`"usage":{"output_tokens":25}}`

func TestClaudeStream_ForwardedAccountingKeepsThePrompt(t *testing.T) {
	state := NewClaudeStreamState("", "claude-x")
	for _, payload := range []string{claudeMessageStart, claudeTextDelta, claudeMessageDelta} {
		if len(state.Frames(TargetClaude, []byte(payload))) == 0 {
			t.Fatalf("Frames(%s) forwarded nothing", payload[:24])
		}
	}
	usage := state.Usage()
	if usage == nil {
		t.Fatal("Usage() = nil, want the numbers both events reported")
	}
	if usage.PromptTokens != 100 {
		t.Fatalf("PromptTokens = %d, want the 100 message_start stated", usage.PromptTokens)
	}
	if usage.CompletionTokens != 25 {
		t.Fatalf("CompletionTokens = %d, want the 25 message_delta stated", usage.CompletionTokens)
	}
	if usage.TotalTokens != 125 {
		t.Fatalf("TotalTokens = %d, want 125", usage.TotalTokens)
	}
}

func TestClaudeStream_ForwardedAccountingIsNotResetByALaterEvent(t *testing.T) {
	state := NewClaudeStreamState("", "claude-x")
	state.Frames(TargetClaude, []byte(claudeMessageStart))
	state.Frames(TargetClaude, []byte(claudeMessageDelta))
	// A final message_stop restates nothing: the fold must leave the earlier
	// numbers standing rather than falling back to zero.
	state.Frames(TargetClaude, []byte(`{"type":"message_stop"}`))
	usage := state.Usage()
	if usage == nil || usage.PromptTokens != 100 || usage.CompletionTokens != 25 {
		t.Fatalf("Usage() = %+v, want prompt 100 and completion 25 to survive a restating event", usage)
	}
}
