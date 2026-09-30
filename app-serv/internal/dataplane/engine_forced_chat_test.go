// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_forced_chat_test.go
// @for       Folding an OpenAI chat stream back into the single completion it
//
//	stands for.
//
// @uses      testing, context, encoding/json, internal/schema.
// @reason    A provider that forces streaming still has to answer a client that
//
//	asked for one JSON body, and a chat stream reports that answer as
//	deltas rather than as one object. The accumulation is a separate
//	concern from reading the events, so its test lives beside it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-21
package dataplane

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestRelay_ForcedStreamServesANonStreamingClientFromChat pins the same fold for
// the chat wire, where the answer is assembled from deltas rather than read from
// one terminal event: the content fragments join, the finish reason survives, and
// the usage frame is picked up.
func TestRelay_ForcedStreamServesANonStreamingClientFromChat(t *testing.T) {
	var sawStream bool
	server := newStreamingUpstream(t, chatStreamBody, &sawStream)
	defer server.Close()

	entry := registry.Provider{
		ID: "forced-chat", Priority: 1, Category: "free", PassthroughModels: true,
		Transport: registry.Transport{BaseURL: server.URL, Format: registry.DefaultFormat},
	}
	engine := newForcedStreamEngine(t, server.URL, entry, &forcedStreamConnector{
		Base: provider.Base{ID: entry.ID, Auth: "no_auth", Format: registry.DefaultFormat},
		url:  server.URL,
	})

	in := relayRequest("forced-chat/big-pickle")
	in.Stream = false

	outcome, err := engine.Relay(context.Background(), in, nil)
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if outcome.Streamed {
		t.Fatal("Outcome.Streamed = true, want false")
	}
	if !sawStream {
		t.Fatal("the outbound request did not ask for a stream")
	}

	var answer schema.ChatCompletionResponse
	if err := json.Unmarshal(outcome.Body, &answer); err != nil {
		t.Fatalf("the answer is not a chat completion: %v (body=%s)", err, outcome.Body)
	}
	if len(answer.Choices) != 1 {
		t.Fatalf("choices = %d, want one", len(answer.Choices))
	}
	if got := answer.Choices[0].Message.Content.Text; got != "PONG" {
		t.Fatalf("content = %q, want the joined PONG", got)
	}
	if got := answer.Choices[0].FinishReason; got != FinishStop {
		t.Fatalf("finish_reason = %q, want %q", got, FinishStop)
	}
	if outcome.Usage == nil || outcome.Usage.PromptTokens != 10 {
		t.Fatalf("usage = %+v, want the stream's 10 prompt tokens", outcome.Usage)
	}
}

// TestFoldedChat_AbsorbsReasoningFromEveryVendorShape pins the fold's reasoning
// accumulation across the vendor shapes the reference's extractReasoningText
// reads (open-sse/translator/concerns/reasoning.js): reasoning_content, then
// reasoning, then the joined reasoning_details entries, decided per delta so a
// frame carrying two shapes is not counted twice. A forced stream that reports
// reasoning under a compat layer's field has to reach the non-streaming client
// rather than vanish between the frames.
func TestFoldedChat_AbsorbsReasoningFromEveryVendorShape(t *testing.T) {
	chunk := func(delta string) object {
		payload := `{"id":"c1","object":"chat.completion.chunk","created":1,"model":"m",` +
			`"choices":[{"index":0,"delta":` + delta + `,"finish_reason":null}]}`
		decoded, ok := decodeObject([]byte(payload))
		if !ok {
			t.Fatalf("the chunk is not a JSON object: %s", payload)
		}
		return decoded
	}
	tests := []struct {
		name   string
		deltas []object
		want   string
	}{
		{"reasoning_content fragments join", []object{chunk(`{"reasoning_content":"th"}`), chunk(`{"reasoning_content":"ink"}`)}, "think"},
		{"reasoning fragments join", []object{chunk(`{"reasoning":"th"}`), chunk(`{"reasoning":"ink"}`)}, "think"},
		{"reasoning_details text entries join", []object{chunk(`{"reasoning_details":[{"type":"reasoning.text","text":"th","index":0},{"type":"reasoning.text","text":"ink","index":1}]}`)}, "think"},
		{"reasoning_details content entries join", []object{chunk(`{"reasoning_details":[{"content":"th"}]}`)}, "th"},
		{"reasoning_details bare strings join", []object{chunk(`{"reasoning_details":["th","ink"]}`)}, "think"},
		{"reasoning outranks reasoning_details in one delta", []object{chunk(`{"reasoning":"one","reasoning_details":[{"text":"two"}]}`)}, "one"},
		{"reasoning_content outranks reasoning", []object{chunk(`{"reasoning_content":"a","reasoning":"b"}`)}, "a"},
		{"shapes mix across deltas", []object{chunk(`{"reasoning":"a"}`), chunk(`{"reasoning_content":"b"}`)}, "ab"},
		{"a content-only delta carries no reasoning", []object{chunk(`{"content":"pong"}`)}, ""},
		{"empty shapes stay empty", []object{chunk(`{"reasoning":""}`), chunk(`{"reasoning_details":[]}`), chunk(`{"reasoning_details":[{}]}`)}, ""},
		{
			name:   "reasoning stays beside content in the folded answer",
			deltas: []object{chunk(`{"reasoning":"th"}`), chunk(`{"content":"pong"}`)},
			want:   "th",
		},
		{
			name:   "reasoning stays when the answer carries none",
			deltas: []object{chunk(`{"reasoning":"th"}`)},
			want:   "th",
		},
		{
			name: "reasoning stays beside tool calls with no content",
			deltas: []object{
				chunk(`{"reasoning":"th"}`),
				chunk(`{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{}"}}]}`),
			},
			want: "th",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fold := foldedChat{calls: map[int]*schema.ToolCall{}}
			for _, delta := range tt.deltas {
				fold.absorb(delta)
			}
			if got := fold.response().Choices[0].Message.Reasoning; got != tt.want {
				t.Fatalf("Reasoning = %q, want %q", got, tt.want)
			}
		})
	}
}
