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
