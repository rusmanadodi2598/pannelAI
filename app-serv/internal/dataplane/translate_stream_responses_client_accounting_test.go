// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_stream_responses_client_accounting_test.go
// @for       Table-driven tests for the identity and accounting a Responses client
//
//	stream reports.
//
// @uses      testing.
// @reason    SPEC-API-001 §7.15 serves POST /api/v1/responses, and the id and the
//
//	token counts are what a client logs and bills against. An answer that
//	mints a second id, or reports a zero block where the upstream reported
//	nothing, is wrong in a way the text alone would not show.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import "testing"

// TestResponsesClientStream_Identity pins the two identifiers the client sees: the
// response id, which is derived from the upstream's so the two can be correlated,
// and the model, which is reported as the upstream named it.
func TestResponsesClientStream_Identity(t *testing.T) {
	cases := []struct {
		name      string
		payload   string
		wantID    string
		wantModel string
	}{
		{
			name:      "the upstream id is carried under the Responses prefix",
			payload:   openAIChunk(map[string]any{"content": "hi"}, ""),
			wantID:    "resp_chatcmpl-up",
			wantModel: "upstream-model",
		},
		{
			name:      "an upstream that reported no id falls back to a stable one",
			payload:   `{"choices":[{"index":0,"delta":{"content":"hi"}}]}`,
			wantID:    "resp_pannelai",
			wantModel: "pannelai-model",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := NewResponsesStreamState("", "pannelai-model", 1700000000)
			frames := feedClient(state, TargetOpenAI, tc.payload)
			frames = append(frames, state.Finish()...)

			response, _ := eventBodyOf(t, frames[0])["response"].(map[string]any)
			if got, _ := response["id"].(string); got != tc.wantID {
				t.Fatalf("opening id = %q, want %q", got, tc.wantID)
			}
			closing, _ := eventBodyOf(t, frames[len(frames)-2])["response"].(map[string]any)
			if got, _ := closing["id"].(string); got != tc.wantID {
				t.Fatalf("closing id = %q, want %q", got, tc.wantID)
			}
			if got, _ := closing["model"].(string); got != tc.wantModel {
				t.Fatalf("closing model = %q, want %q", got, tc.wantModel)
			}
		})
	}
}

// TestResponsesClientStream_ClosersAreIdempotent pins that an item closes once:
// a finish reason already closed it, and Finish runs afterwards on every stream.
func TestResponsesClientStream_ClosersAreIdempotent(t *testing.T) {
	state := NewResponsesStreamState("up-1", "pannelai-model", 1700000000)
	frames := feedClient(state, TargetOpenAI,
		openAIChunk(map[string]any{"content": "hi"}, ""),
		openAIChunk(map[string]any{}, FinishStop))
	frames = append(frames, state.Finish()...)

	labels := frameLabels(frames)
	for _, name := range []string{
		EventResponseCreated, EventResponseItemAdded, EventResponseOutputTextDone,
		EventResponseCompleted, "[DONE]",
	} {
		if got := countOf(labels, name); got != 1 {
			t.Fatalf("%s emitted %d times, want 1: %v", name, got, labels)
		}
	}
}

// TestResponsesClientStream_SecondFinishRepeatsNothing pins that the once-only
// guards hold across a second close: only the terminal marker repeats, because
// Finish is the one-shot closer the relay calls when the upstream body ends.
func TestResponsesClientStream_SecondFinishRepeatsNothing(t *testing.T) {
	state := NewResponsesStreamState("up-1", "pannelai-model", 1700000000)
	frames := feedClient(state, TargetOpenAI,
		openAIChunk(map[string]any{"content": "hi"}, ""),
		openAIChunk(map[string]any{}, FinishStop))
	frames = append(frames, state.Finish()...)
	frames = append(frames, state.Finish()...)

	labels := frameLabels(frames)
	for _, name := range []string{
		EventResponseCreated, EventResponseItemAdded, EventResponseOutputTextDone,
		EventResponseCompleted,
	} {
		if got := countOf(labels, name); got != 1 {
			t.Fatalf("%s emitted %d times, want 1: %v", name, got, labels)
		}
	}
	if got := countOf(labels, "[DONE]"); got != 2 {
		t.Fatalf("terminal marker emitted %d times, want 2: %v", got, labels)
	}
}

// TestResponsesClientStream_Accounting pins that the closing event reports the
// accounting the upstream sent, and reports none at all when it sent none: a zero
// block would claim the call was measured and cost nothing.
func TestResponsesClientStream_Accounting(t *testing.T) {
	cases := []struct {
		name       string
		payloads   []string
		wantUsage  bool
		wantInput  int
		wantOutput int
	}{
		{
			name:       "a reported usage is carried into the closing event",
			payloads:   []string{openAIChunk(map[string]any{"content": "hi"}, ""), usageOnlyChunk(11, 4)},
			wantUsage:  true,
			wantInput:  11,
			wantOutput: 4,
		},
		{
			name:      "an unreported usage is absent rather than zero",
			payloads:  []string{openAIChunk(map[string]any{"content": "hi"}, FinishStop)},
			wantUsage: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := NewResponsesStreamState("up-1", "pannelai-model", 1700000000)
			frames := feedClient(state, TargetOpenAI, tc.payloads...)
			frames = append(frames, state.Finish()...)

			response, _ := eventBodyOf(t, frames[len(frames)-2])["response"].(map[string]any)
			usage, ok := response["usage"].(map[string]any)
			if ok != tc.wantUsage {
				t.Fatalf("usage present = %v, want %v: %v", ok, tc.wantUsage, response)
			}
			if !ok {
				if state.Usage() != nil {
					t.Fatalf("state reports usage %v for a stream that sent none", state.Usage())
				}
				return
			}
			if got, _ := usage["input_tokens"].(float64); int(got) != tc.wantInput {
				t.Fatalf("input_tokens = %v, want %d", usage["input_tokens"], tc.wantInput)
			}
			if got, _ := usage["output_tokens"].(float64); int(got) != tc.wantOutput {
				t.Fatalf("output_tokens = %v, want %d", usage["output_tokens"], tc.wantOutput)
			}
			if state.Usage() == nil {
				t.Fatal("state reports no usage for a stream that sent one")
			}
		})
	}
}
