// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_responses_claude_test.go
// @for       Table-driven tests for Responses-to-Anthropic answer translation.
// @uses      testing, internal/schema.
// @reason    SPEC-API-001 §10 makes the Responses API a P3 deliverable, and a
//
//	client on /api/v1/messages can resolve to a provider that speaks
//	it. Anthropic's envelope differs in both block order and stop
//	reason, and its input count excludes what OpenAI's includes, so
//	each rule is pinned here rather than left to the shared fold.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestResponsesToClaudeResponse_Blocks pins the block sequence Anthropic
// requires: text first, then thinking, then one tool_use per call. An empty
// block is not emitted, because Anthropic rejects a text block with no text.
func TestResponsesToClaudeResponse_Blocks(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantTypes string
		wantText  string
		wantThink string
		wantCalls int
		wantStop  string
	}{
		{
			name:      "text alone becomes one text block",
			body:      `{"id":"resp_1","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"hello"}]}]}`,
			wantTypes: schema.BlockText,
			wantText:  "hello",
			wantStop:  schema.StopEndTurn,
		},
		{
			name:      "reasoning is carried as a thinking block",
			body:      `{"id":"resp_1","status":"completed","output":[{"type":"reasoning","summary":[{"type":"summary_text","text":"why"}]}]}`,
			wantTypes: schema.BlockThinking,
			wantThink: "why",
			wantStop:  schema.StopEndTurn,
		},
		{
			name:      "text precedes thinking",
			body:      `{"id":"resp_1","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"answer"}]},{"type":"reasoning","summary":[{"type":"summary_text","text":"why"}]}]}`,
			wantTypes: schema.BlockText + "|" + schema.BlockThinking,
			wantText:  "answer",
			wantThink: "why",
			wantStop:  schema.StopEndTurn,
		},
		{
			name:      "a call becomes a tool_use block and stops as tool use",
			body:      `{"id":"resp_1","status":"completed","output":[{"type":"function_call","call_id":"call_9","name":"lookup","arguments":"{\"q\":1}"}]}`,
			wantTypes: schema.BlockToolUse,
			wantCalls: 1,
			wantStop:  schema.StopToolUse,
		},
		{
			name:      "an answer cut by the ceiling stops as max tokens",
			body:      `{"id":"resp_1","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[{"type":"message","content":[{"type":"output_text","text":"half"}]}]}`,
			wantTypes: schema.BlockText,
			wantText:  "half",
			wantStop:  schema.StopMaxTokens,
		},
		{
			name:     "an empty output array yields no blocks at all",
			body:     `{"id":"resp_1","status":"completed","output":[]}`,
			wantStop: schema.StopEndTurn,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResponsesToClaudeResponse([]byte(tc.body), "m")
			if err != nil {
				t.Fatalf("translating the answer: %v", err)
			}
			if types := blockTypes(got.Content); types != tc.wantTypes {
				t.Fatalf("block sequence = %q, want %q (%+v)", types, tc.wantTypes, got.Content)
			}
			if tc.wantText != "" || tc.wantTypes == schema.BlockText {
				if len(got.Content) == 0 || got.Content[0].Text != tc.wantText {
					t.Fatalf("first text block = %+v, want text %q", got.Content, tc.wantText)
				}
			}
			for _, block := range got.Content {
				if block.Type == schema.BlockThinking && block.Thinking != tc.wantThink {
					t.Fatalf("thinking = %q, want %q", block.Thinking, tc.wantThink)
				}
			}
			calls := 0
			for _, block := range got.Content {
				if block.Type == schema.BlockToolUse {
					calls++
				}
			}
			if calls != tc.wantCalls {
				t.Fatalf("tool_use blocks = %d, want %d", calls, tc.wantCalls)
			}
			if got.StopReason != tc.wantStop {
				t.Fatalf("stop reason = %q, want %q", got.StopReason, tc.wantStop)
			}
			if got.Role != RoleAssistant || got.Type != "message" {
				t.Fatalf("envelope = %q/%q, want message/assistant", got.Type, got.Role)
			}
			if got.Model != "m" {
				t.Fatalf("model = %q, want the resolved id", got.Model)
			}
		})
	}
}

// blockTypes renders a block sequence compactly for a table assertion.
func blockTypes(blocks []schema.Block) string {
	if len(blocks) == 0 {
		return ""
	}
	out := blocks[0].Type
	for _, block := range blocks[1:] {
		out += "|" + block.Type
	}
	return out
}

// TestResponsesToClaudeResponse_ToolInput pins that a call's argument string
// becomes the structured input Anthropic expects, and that an unparseable one
// still yields a usable object rather than a nil the client would choke on.
func TestResponsesToClaudeResponse_ToolInput(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantID   string
		wantName string
		wantJSON string
	}{
		{
			name:     "an object argument string is decoded into the input",
			body:     `{"status":"completed","output":[{"type":"function_call","call_id":"call_9","name":"lookup","arguments":"{\"q\":1}"}]}`,
			wantID:   "call_9",
			wantName: "lookup",
			wantJSON: `{"q":1}`,
		},
		{
			name:     "an empty argument string becomes an empty object",
			body:     `{"status":"completed","output":[{"type":"function_call","call_id":"call_9","name":"lookup","arguments":""}]}`,
			wantID:   "call_9",
			wantName: "lookup",
			wantJSON: `{}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResponsesToClaudeResponse([]byte(tc.body), "m")
			if err != nil {
				t.Fatalf("translating the answer: %v", err)
			}
			if len(got.Content) != 1 || got.Content[0].Type != schema.BlockToolUse {
				t.Fatalf("want one tool_use block, got %+v", got.Content)
			}
			block := got.Content[0]
			if block.ID != tc.wantID {
				t.Fatalf("id = %q, want %q", block.ID, tc.wantID)
			}
			if block.Name != tc.wantName {
				t.Fatalf("name = %q, want %q", block.Name, tc.wantName)
			}
			if string(block.Input) != tc.wantJSON {
				t.Fatalf("input = %s, want %s", block.Input, tc.wantJSON)
			}
		})
	}
}
