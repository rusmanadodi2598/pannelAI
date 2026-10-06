// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/stop_claude_test.go
// @for       The caller's `stop_sequences` on the Anthropic wire: the streamed cut and the one-body cut, each reported as a stop sequence.
// @uses      encoding/json, strings, testing, internal/schema.
// @reason    Anthropic distinguishes an answer the caller ended from one the model finished, and only the marker itself carries that: `stop_reason: "stop_sequence"` plus `stop_sequence`. Measured live on 2026-09-30 the gateway answered `A STOPHERE B` with `end_turn` through /api/v1/messages. These pin both halves, the text the client must not receive, and the reason the client must be told, and that a `tool_use` block is never collateral damage of a text cut.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-30
package dataplane

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// openAIChunkOf builds one upstream OpenAI chunk with a text delta.
func openAIChunkOf(text, finish string) string {
	return `{"id":"c1","model":"gpt","choices":[{"index":0,"delta":{"content":"` + text +
		`"},"finish_reason":` + quoteOrNull(finish) + `}]}`
}

func quoteOrNull(value string) string {
	if value == "" {
		return "null"
	}
	return `"` + value + `"`
}

// claudeStream runs one streamed answer through the guard and returns the text the
// client saw, the closing stop reason and marker, and the block open/close counts.
type claudeStream struct {
	text         string
	stopReason   string
	stopSequence string
	opens        int
	stops        int
}

func readClaudeStream(t *testing.T, sequences []string, chunks ...string) claudeStream {
	t.Helper()
	state := NewClaudeStreamState("", "codebuddy-intl/deepseek-v4.1-flash").WithStopSequences(sequences)
	var got claudeStream
	feed := func(frames [][]byte) {
		for _, frame := range frames {
			name := eventNameOf(frame)
			switch name {
			case schema.EventContentBlockStart:
				got.opens++
			case schema.EventContentBlockStop:
				got.stops++
			case schema.EventContentBlockDelta:
				var event struct {
					Delta struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"delta"`
				}
				if err := json.Unmarshal(dataOf(t, frame), &event); err != nil {
					t.Fatalf("content_block_delta does not decode: %v", err)
				}
				if event.Delta.Type == schema.DeltaText {
					got.text += event.Delta.Text
				}
			case schema.EventMessageDelta:
				var event struct {
					Delta struct {
						StopReason   string  `json:"stop_reason"`
						StopSequence *string `json:"stop_sequence"`
					} `json:"delta"`
				}
				if err := json.Unmarshal(dataOf(t, frame), &event); err != nil {
					t.Fatalf("message_delta does not decode: %v", err)
				}
				got.stopReason = event.Delta.StopReason
				if event.Delta.StopSequence != nil {
					got.stopSequence = *event.Delta.StopSequence
				}
			}
		}
	}
	for _, chunk := range chunks {
		feed(state.Frames(TargetOpenAI, []byte(chunk)))
	}
	feed(state.Finish())
	return got
}

// dataOf returns the JSON payload of one rendered SSE frame.
func dataOf(t *testing.T, frame []byte) []byte {
	t.Helper()
	for _, line := range strings.Split(string(frame), "\n") {
		if payload, found := strings.CutPrefix(line, "data: "); found {
			return []byte(payload)
		}
	}
	t.Fatalf("frame carries no data line: %s", frame)
	return nil
}

func TestClaudeStream_CutsTheAnswerAtTheStopSequence(t *testing.T) {
	got := readClaudeStream(t, []string{"STOPHERE"},
		openAIChunkOf("keep me", ""),
		openAIChunkOf(" STOPHERE drop this", ""),
		openAIChunkOf(" and more", "stop"),
	)
	// The marker itself is withheld; the space before it is real answer text the
	// caller asked to see.
	if got.text != "keep me " {
		t.Fatalf("text the client saw = %q, want the marker and everything after it withheld", got.text)
	}
	if got.stopReason != StopStopSequence {
		t.Fatalf("stop_reason = %q, want stop_sequence", got.stopReason)
	}
	if got.stopSequence != "STOPHERE" {
		t.Fatalf("stop_sequence = %q, want the marker the caller sent", got.stopSequence)
	}
	if got.opens != got.stops {
		t.Fatalf("%d blocks opened but %d closed", got.opens, got.stops)
	}
}

func TestClaudeStream_CutReportsStopSequenceEvenWhenUpstreamSaysLength(t *testing.T) {
	got := readClaudeStream(t, []string{"X"},
		openAIChunkOf("cutX here", ""),
		openAIChunkOf("more", "length"),
	)
	if got.stopReason != StopStopSequence {
		t.Fatalf("stop_reason = %q, want the caller's cut to outrank the vendor's ceiling", got.stopReason)
	}
	if strings.Contains(got.text, "X") {
		t.Fatalf("the marker reached the client: %q", got.text)
	}
}

func TestClaudeStream_CutHoldsAMarkerSplitAcrossFragments(t *testing.T) {
	got := readClaudeStream(t, []string{"STOPHERE"},
		openAIChunkOf("AB STOP", ""),
		openAIChunkOf("HERE C", ""),
	)
	if strings.Contains(got.text, "STOP") {
		t.Fatalf("a piece of the marker reached the client: %q", got.text)
	}
	if got.text != "AB " {
		t.Fatalf("text = %q, want %q", got.text, "AB ")
	}
}

func TestClaudeStream_ReleasesHeldTextAndKeepsTheReason(t *testing.T) {
	got := readClaudeStream(t, []string{"STOPHERE"},
		openAIChunkOf("the end is near STOP", ""),
		openAIChunkOf("", "stop"),
	)
	// The withheld tail was never a marker, so it belongs to the answer, and with
	// no cut the upstream's own reason stands.
	if got.text != "the end is near STOP" {
		t.Fatalf("text = %q, want the released tail", got.text)
	}
	if got.stopReason != StopEndTurn {
		t.Fatalf("stop_reason = %q, want end_turn for an uncut answer", got.stopReason)
	}
	if got.stopSequence != "" {
		t.Fatalf("stop_sequence = %q, want none reported", got.stopSequence)
	}
	if got.opens != got.stops {
		t.Fatalf("%d blocks opened but %d closed", got.opens, got.stops)
	}
}

func TestClaudeStream_NoSequencesChangesNothing(t *testing.T) {
	got := readClaudeStream(t, nil, openAIChunkOf("all of it STOPHERE", "stop"))
	if got.text != "all of it STOPHERE" {
		t.Fatalf("text = %q, want the answer untouched with no sequences named", got.text)
	}
	if got.stopReason != StopEndTurn || got.stopSequence != "" {
		t.Fatalf("closing delta = %q/%q, want end_turn with no marker", got.stopReason, got.stopSequence)
	}
}
