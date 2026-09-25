// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_forced_stream_incomplete_test.go
// @for       Folding a Responses stream that ended on the output ceiling.
// @uses      context, encoding/json, net/http, testing, internal/provider,
//
//	internal/registry, internal/schema.
//
// @reason    OpenCode Free answers `response.incomplete` whenever a request
//
//	stops on max_output_tokens, which the live upstream did on every
//	short-budget call measured on 2026-09-24. A fold that only read
//	`response.completed` treated that finished answer as a broken
//	stream and answered 502 UPSTREAM_ERROR, so a correct answer became
//	a failure the client could not use. The case lives in its own file
//	because it pins a terminal-event vocabulary rather than the fold's
//	shape (AGENTS.md §1.1 keeps each file to one concern).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package dataplane

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// responsesIncompleteStreamBody is the SSE a Responses upstream sends when the
// answer reached the output ceiling. The terminal event is `response.incomplete`
// rather than `response.completed`, which is the shape OpenCode Free measured on
// 2026-09-24 for muse-spark-1.3-contributor-free: every request that stopped on
// max_output_tokens ended this way, so a fold that only reads `response.completed`
// answered 502 UPSTREAM_ERROR for a stream that was complete and correct.
const responsesIncompleteStreamBody = "event: response.created\n" +
	`data: {"type":"response.created","response":{"id":"resp_1","model":"muse"}}` + "\n\n" +
	"event: response.output_text.delta\n" +
	`data: {"type":"response.output_text.delta","delta":"PO"}` + "\n\n" +
	"event: response.incomplete\n" +
	`data: {"type":"response.incomplete","response":{"id":"resp_1","object":"response","status":"incomplete",` +
	`"incomplete_details":{"reason":"max_output_tokens"},` +
	`"model":"muse","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"PO"}]}],` +
	`"usage":{"input_tokens":570,"output_tokens":175,"total_tokens":745}}}` + "\n\n" +
	"data: [DONE]\n\n"

// TestRelay_ForcedStreamFoldsAResponsesCeilingStop pins the measured defect: a
// stream whose terminal event is `response.incomplete` is a finished answer, not
// an upstream failure. The client that asked for one body must receive the
// completion, and its finish reason must be the ceiling stop (`length`) rather
// than a fabricated `stop`.
func TestRelay_ForcedStreamFoldsAResponsesCeilingStop(t *testing.T) {
	var sawStream bool
	server := newStreamingUpstream(t, responsesIncompleteStreamBody, &sawStream)
	defer server.Close()

	entry := registry.Provider{
		ID: "forced-incomplete", Priority: 1, Category: "free", PassthroughModels: true,
		Transport: registry.Transport{BaseURL: server.URL, Format: registry.FormatOpenAIResponses},
	}
	engine := newForcedStreamEngine(t, server.URL, entry, &forcedStreamConnector{
		Base: provider.Base{ID: entry.ID, Auth: "no_auth", Format: registry.FormatOpenAIResponses},
		url:  server.URL,
	})

	in := relayRequest("forced-incomplete/muse")
	in.Stream = false

	outcome, err := engine.Relay(context.Background(), in, nil)
	if err != nil {
		t.Fatalf("Relay() error = %v, want an incomplete stream to fold rather than fail", err)
	}
	var answer schema.ChatCompletionResponse
	if err := json.Unmarshal(outcome.Body, &answer); err != nil {
		t.Fatalf("the answer is not a chat completion: %v (body=%s)", err, outcome.Body)
	}
	if len(answer.Choices) != 1 {
		t.Fatalf("choices = %d, want one", len(answer.Choices))
	}
	if got := answer.Choices[0].Message.Content.Text; got != "PO" {
		t.Fatalf("content = %q, want PO", got)
	}
	if got := answer.Choices[0].FinishReason; got != FinishLength {
		t.Fatalf("finish_reason = %q, want %q for a ceiling stop", got, FinishLength)
	}
	if outcome.Usage == nil || outcome.Usage.CompletionTokens != 175 {
		t.Fatalf("usage = %+v, want the terminal event's 570/175", outcome.Usage)
	}
}

// TestFoldResponsesEvents_ReadsEveryTerminalEvent pins the vocabulary directly,
// so the accepted terminal set is stated rather than implied by one integration
// case: an upstream that closes with `response.incomplete`, `response.completed`,
// `response.done`, or `response.failed` has all reported a final answer, and only
// a stream with none of them is the failure.
func TestFoldResponsesEvents_ReadsEveryTerminalEvent(t *testing.T) {
	cases := []struct {
		name  string
		event string
	}{
		{name: "response.completed", event: EventResponseCompleted},
		{name: "response.done", event: EventResponseDone},
		{name: "response.incomplete", event: EventResponseIncomplete},
		{name: "response.failed", event: EventResponseFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := `{"type":"` + tc.event + `","response":{"id":"resp_1","status":"incomplete",` +
				`"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}`
			body, usage, err := foldResponsesEvents([][]byte{[]byte(payload)})
			if err != nil {
				t.Fatalf("foldResponsesEvents() error = %v, want %s to close the stream", err, tc.event)
			}
			if len(body) == 0 {
				t.Fatal("foldResponsesEvents() returned no body")
			}
			if usage == nil || usage.PromptTokens != 5 || usage.CompletionTokens != 2 {
				t.Fatalf("usage = %+v, want the terminal event's 5/2", usage)
			}
		})
	}
}

// TestFoldResponsesEvents_RefusesAStreamWithNoTerminalEvent is the control: a
// stream that never stated an answer is still an upstream failure, so widening
// the accepted set must not turn a truncated connection into a silent success.
func TestFoldResponsesEvents_RefusesAStreamWithNoTerminalEvent(t *testing.T) {
	events := [][]byte{[]byte(`{"type":"response.output_text.delta","delta":"PO"}`)}
	if _, _, err := foldResponsesEvents(events); err == nil {
		t.Fatal("foldResponsesEvents() error = nil, want a refusal for a stream with no terminal event")
	}
}
