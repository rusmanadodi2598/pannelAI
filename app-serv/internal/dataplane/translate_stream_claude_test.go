// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_stream_claude_test.go
// @for       The stream-state translation of an Anthropic stream onto the OpenAI
//
//	framing, in both directions.
//
// @uses      testing, internal/schema.
// @reason    Frame order is what a streaming client parses, so the role frame, the
//
//	deltas and the terminal marker are pinned apart from the retry policy
//	that shares the file's name.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04
package dataplane

import (
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestStreamState_ClaudeFrames pins the OpenAI re-framing of an Anthropic stream:
// the role frame, the text deltas, the tool-call index, and the finish frame.
func TestStreamState_ClaudeFrames(t *testing.T) {
	cases := []struct {
		name      string
		events    []string
		wantParts []string
		wantNot   []string
	}{
		{
			name: "message_start opens the assistant role",
			events: []string{
				`{"type":"message_start","message":{"id":"msg_1","usage":{"input_tokens":7,"output_tokens":0}}}`,
			},
			wantParts: []string{`"role":"assistant"`, `"id":"msg_1"`},
		},
		{
			name: "text deltas become content deltas",
			events: []string{
				`{"type":"message_start","message":{"id":"msg_1"}}`,
				`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`,
			},
			wantParts: []string{`"content":"hello"`},
		},
		{
			name: "a thinking delta becomes a reasoning delta",
			events: []string{
				`{"type":"message_start","message":{"id":"msg_1"}}`,
				`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"hmm"}}`,
			},
			wantParts: []string{`"reasoning_content":"hmm"`},
		},
		{
			name: "the stop reason maps onto the OpenAI finish reason",
			events: []string{
				`{"type":"message_start","message":{"id":"msg_1"}}`,
				`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":1,"output_tokens":2}}`,
			},
			wantParts: []string{`"finish_reason":"stop"`},
		},
		{
			name: "a tool_use block opens a tool call with its name",
			events: []string{
				`{"type":"message_start","message":{"id":"msg_1"}}`,
				`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"t1","name":"Read"}}`,
			},
			wantParts: []string{`"tool_calls"`, `"name":"Read"`, `"id":"t1"`},
		},
		{
			name: "argument fragments attach to the opened tool call",
			events: []string{
				`{"type":"message_start","message":{"id":"msg_1"}}`,
				`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"t1","name":"Read"}}`,
				`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"a\":1}"}}`,
			},
			wantParts: []string{`"arguments":"{\"a\":1}"`},
		},
		{
			name: "a ping carries nothing and is dropped",
			events: []string{
				`{"type":"message_start","message":{"id":"msg_1"}}`,
				`{"type":"ping"}`,
			},
			wantParts: []string{`"role":"assistant"`},
			wantNot:   []string{`"ping"`},
		},
		{
			name: "a tool_use stop reason maps onto tool_calls",
			events: []string{
				`{"type":"message_start","message":{"id":"msg_1"}}`,
				`{"type":"message_delta","delta":{"stop_reason":"tool_use"}}`,
			},
			wantParts: []string{`"finish_reason":"tool_calls"`},
		},
		{
			name: "a max_tokens stop reason maps onto length",
			events: []string{
				`{"type":"message_start","message":{"id":"msg_1"}}`,
				`{"type":"message_delta","delta":{"stop_reason":"max_tokens"}}`,
			},
			wantParts: []string{`"finish_reason":"length"`},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := NewStreamState("", "claude-x", 1700000000, false)
			var frames [][]byte
			for _, event := range tc.events {
				frames = append(frames, state.Frames(TargetClaude, []byte(event))...)
			}
			rendered := frameText(frames)
			for _, want := range tc.wantParts {
				if !strings.Contains(rendered, want) {
					t.Fatalf("frames = %s, want them to contain %s", rendered, want)
				}
			}
			for _, unwanted := range tc.wantNot {
				if strings.Contains(rendered, unwanted) {
					t.Fatalf("frames = %s, want them NOT to contain %s", rendered, unwanted)
				}
			}
		})
	}
}

// TestClaudeStreamState_FrameOrder pins Anthropic's block discipline: message_start
// comes first, each content block is opened before it is fed and closed before the
// next opens, and message_stop ends the stream.
func TestClaudeStreamState_FrameOrder(t *testing.T) {
	cases := []struct {
		name      string
		chunks    []string
		wantOrder []string
	}{
		{
			name:   "text opens, feeds, and closes around the finish",
			chunks: []string{`{"id":"c1","model":"gpt","choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":null}]}`},
			wantOrder: []string{
				schema.EventMessageStart, schema.EventContentBlockStart, schema.EventContentBlockDelta, schema.EventContentBlockStop,
				schema.EventMessageDelta, schema.EventMessageStop,
			},
		},
		{
			name: "a tool call opens a tool_use block and feeds its arguments",
			chunks: []string{
				`{"id":"c1","model":"gpt","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"t1","function":{"name":"Read"}}]},"finish_reason":null}]}`,
				`{"id":"c1","model":"gpt","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`,
			},
			wantOrder: []string{
				schema.EventMessageStart, schema.EventContentBlockStart, schema.EventContentBlockDelta,
				schema.EventContentBlockStop, schema.EventMessageDelta, schema.EventMessageStop,
			},
		},
		{
			name:      "a stream with no content still opens and closes",
			chunks:    []string{`{"id":"c1","model":"gpt","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`},
			wantOrder: []string{schema.EventMessageStart, schema.EventMessageDelta, schema.EventMessageStop},
		},
		{
			name:   "a usage-only frame carries no content block",
			chunks: []string{`{"id":"c1","model":"gpt","choices":[],"usage":{"prompt_tokens":3,"completion_tokens":4}}`},
			// The usage chunk opens the message and nothing else; Finish still
			// closes it, because an Anthropic client needs message_stop to
			// release the connection.
			wantOrder: []string{schema.EventMessageStart, schema.EventMessageDelta, schema.EventMessageStop},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := NewClaudeStreamState("", "claude-x")
			var order []string
			for _, chunk := range tc.chunks {
				for _, frame := range state.Frames(TargetOpenAI, []byte(chunk)) {
					if event := eventNameOf(frame); event != "" {
						order = append(order, event)
					}
				}
			}
			for _, frame := range state.Finish() {
				if event := eventNameOf(frame); event != "" {
					order = append(order, event)
				}
			}
			if strings.Join(order, ",") != strings.Join(tc.wantOrder, ",") {
				t.Fatalf("event order = %v, want %v", order, tc.wantOrder)
			}
			// Every block that opened must close before the stream ends.
			opens := countOf(order, schema.EventContentBlockStart)
			stops := countOf(order, schema.EventContentBlockStop)
			if opens != stops {
				t.Fatalf("%d content blocks opened but %d closed: %v", opens, stops, order)
			}
		})
	}
}
