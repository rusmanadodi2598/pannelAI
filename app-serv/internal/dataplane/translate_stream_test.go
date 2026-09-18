// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_stream_test.go
// @for       Tests for the SSE frame shape, the usage chunk, and retry decisions.
// @uses      testing, internal/schema, net/http.
// @reason    SPEC-API-001 §4 fixes both the SSE frame shape and the usage chunk
//
//	stream_options.include_usage asks for, and §4 also fixes the retry
//	policy per status. Both are pure functions of their inputs, so they are
//	pinned here with no network and no clock (AGENTS.md §2.1).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// frameText renders the frames a test compares against, so a failure shows the
// exact bytes a client would receive.
func frameText(frames [][]byte) string {
	parts := make([]string, 0, len(frames))
	for _, frame := range frames {
		parts = append(parts, string(frame))
	}
	return strings.Join(parts, "")
}

// TestFrameShape pins the SSE framing: `data: <json>\n\n`, and the named-event
// form Anthropic uses. The blank line is the event terminator, so a frame without
// it never flushes at a client.
func TestFrameShape(t *testing.T) {
	cases := []struct {
		name string
		got  []byte
		want string
	}{
		{name: "a data frame ends with a blank line", got: Frame([]byte(`{"a":1}`)), want: "data: {\"a\":1}\n\n"},
		{name: "an empty payload still frames", got: Frame(nil), want: "data: \n\n"},
		{name: "a named event carries its name first", got: EventFrame("message_delta", []byte(`{"t":1}`)), want: "event: message_delta\ndata: {\"t\":1}\n\n"},
		{name: "the terminal marker is the documented constant", got: []byte(SSEDone), want: "data: [DONE]\n\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.got) != tc.want {
				t.Fatalf("frame = %q, want %q", tc.got, tc.want)
			}
		})
	}
}

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

// TestStreamState_UsageChunk pins the usage-chunk rule: it is emitted only when the
// client asked for it, it is a frame of its own with an empty choices array, and
// the terminal marker always closes the stream.
func TestStreamState_UsageChunk(t *testing.T) {
	cases := []struct {
		name         string
		includeUsage bool
		events       []string
		wantUsage    bool
		wantDone     bool
	}{
		{
			name:         "a client that asked for usage gets a usage frame",
			includeUsage: true,
			events: []string{
				`{"type":"message_start","message":{"id":"m","usage":{"input_tokens":5,"output_tokens":0}}}`,
				`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":5,"output_tokens":9}}`,
			},
			wantUsage: true, wantDone: true,
		},
		{
			name:         "a client that did not ask gets no usage frame",
			includeUsage: false,
			events: []string{
				`{"type":"message_start","message":{"id":"m","usage":{"input_tokens":5,"output_tokens":0}}}`,
				`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":5,"output_tokens":9}}`,
			},
			wantUsage: false, wantDone: true,
		},
		{
			name:         "an upstream that reported nothing emits no fabricated usage",
			includeUsage: true,
			events:       []string{`{"type":"message_start","message":{"id":"m"}}`},
			wantUsage:    false, wantDone: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := NewStreamState("", "model-x", 1700000000, tc.includeUsage)
			for _, event := range tc.events {
				state.Frames(TargetClaude, []byte(event))
			}
			rendered := frameText(state.Finish())
			hasUsage := strings.Contains(rendered, `"usage"`) && strings.Contains(rendered, `"prompt_tokens"`)
			if hasUsage != tc.wantUsage {
				t.Fatalf("usage frame present = %v, want %v (frames: %s)", hasUsage, tc.wantUsage, rendered)
			}
			if got := strings.Contains(rendered, "[DONE]"); got != tc.wantDone {
				t.Fatalf("[DONE] present = %v, want %v", got, tc.wantDone)
			}
			if tc.wantUsage && !strings.Contains(rendered, `"choices":[]`) {
				t.Fatalf("usage frame must carry an empty choices array: %s", rendered)
			}
		})
	}
}

// TestStreamState_FinishIsIdempotent pins that a stream which already emitted its
// finish frame does not emit a second one, which a client counting frames would
// read as a second answer.
func TestStreamState_FinishIsIdempotent(t *testing.T) {
	state := NewStreamState("", "m", 1, false)
	var firstFrames [][]byte
	for _, payload := range []string{
		`{"type":"message_start","message":{"id":"m"}}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`,
	} {
		firstFrames = append(firstFrames, state.Frames(TargetClaude, []byte(payload))...)
	}
	// The finish frame rides on the message_delta that carried the stop reason,
	// so the first pass is Frames plus Finish: Finish terminates the stream, it
	// does not finish it.
	firstFrames = append(firstFrames, state.Finish()...)
	first := frameText(firstFrames)

	second := frameText(state.Finish())
	if got := strings.Count(first, `"finish_reason":"stop"`); got != 1 {
		t.Fatalf("first pass emitted %d finish frames, want 1: %s", got, first)
	}
	if strings.Contains(second, `"finish_reason":"stop"`) {
		t.Fatalf("second Finish emitted a finish frame again: %s", second)
	}
	if !strings.Contains(second, "[DONE]") {
		t.Fatalf("second Finish must still terminate the stream: %s", second)
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

// eventNameOf reads the event name from a rendered frame.
func eventNameOf(frame []byte) string {
	const prefix = "event: "
	text := string(frame)
	if !strings.HasPrefix(text, prefix) {
		return ""
	}
	end := strings.IndexByte(text, '\n')
	if end < 0 {
		return ""
	}
	return text[len(prefix):end]
}

// countOf counts occurrences of a value.
func countOf(values []string, want string) int {
	total := 0
	for _, value := range values {
		if value == want {
			total++
		}
	}
	return total
}

// retryPlugin answers the retry question for the statuses a test drives, which is
// what the transport reads from a connector.
type retryPlugin struct {
	provider.Base
	// retryable is the status set this plugin considers worth repeating.
	retryable map[int]bool
	// after is the wait it asks for.
	after int
}

func (p retryPlugin) ShouldRetry(status int, _ http.Header) provider.RetryDecision {
	if !p.retryable[status] {
		return provider.RetryDecision{}
	}
	return provider.RetryDecision{Retry: true}
}

// Endpoint and ApplyAuth are required by the seam but irrelevant to the retry
// question this plugin answers, so they return the zero answer rather than making
// the fixture pretend to be a full connector.
func (p retryPlugin) Endpoint(provider.Request, provider.Credential) (string, error) { return "", nil }

func (p retryPlugin) ApplyAuth(*http.Request, provider.Credential) error { return nil }

// TestDecideRetry pins the retry policy per status (SPEC-API-001 §4): only an
// outcome the plugin calls transient is repeated, the entry's attempt count bounds
// it, and a non-idempotent POST is capped at one retry.
func TestDecideRetry(t *testing.T) {
	transient := retryPlugin{retryable: map[int]bool{429: true, 500: true, 502: true, 503: true, 504: true}}

	cases := []struct {
		name       string
		entry      registry.Provider
		attempt    Attempt
		plugin     provider.Plugin
		wantRetry  bool
		wantAfterU bool
	}{
		{
			name:    "a 429 is retried while attempts remain",
			entry:   registry.Provider{Transport: registry.Transport{Retry: registry.Retry{DefaultAttempts: 3}}},
			attempt: Attempt{Retries: 0, Status: 429}, plugin: transient, wantRetry: true,
		},
		{
			name:    "a 502 is retried",
			entry:   registry.Provider{Transport: registry.Transport{Retry: registry.Retry{DefaultAttempts: 3}}},
			attempt: Attempt{Retries: 0, Status: 502}, plugin: transient, wantRetry: true,
		},
		{
			name:    "a 400 is never retried",
			entry:   registry.Provider{Transport: registry.Transport{Retry: registry.Retry{DefaultAttempts: 3}}},
			attempt: Attempt{Retries: 0, Status: 400}, plugin: transient, wantRetry: false,
		},
		{
			name:    "a 401 is never retried",
			entry:   registry.Provider{Transport: registry.Transport{Retry: registry.Retry{DefaultAttempts: 3}}},
			attempt: Attempt{Retries: 0, Status: 401}, plugin: transient, wantRetry: false,
		},
		{
			name:    "the attempt budget bounds a retryable status",
			entry:   registry.Provider{Transport: registry.Transport{Retry: registry.Retry{DefaultAttempts: 3}}},
			attempt: Attempt{Retries: 3, Status: 503}, plugin: transient, wantRetry: false,
		},
		{
			name:    "an entry allowing a single attempt retries none",
			entry:   registry.Provider{Transport: registry.Transport{Retry: registry.Retry{DefaultAttempts: 1}}},
			attempt: Attempt{Retries: 0, Status: 429}, plugin: transient, wantRetry: false,
		},
		{
			name:    "a non-idempotent POST is capped at one retry",
			entry:   registry.Provider{Transport: registry.Transport{Retry: registry.Retry{DefaultAttempts: 6}}},
			attempt: Attempt{Retries: 1, Status: 429, Idempotent: false}, plugin: transient, wantRetry: false,
		},
		{
			// Three attempts allow two retries, so the second one is the last the
			// entry's budget admits; a third would be one attempt too many.
			name:    "an idempotent request may use the entry's full budget",
			entry:   registry.Provider{Transport: registry.Transport{Retry: registry.Retry{DefaultAttempts: 3}}},
			attempt: Attempt{Retries: 1, Status: 429, Idempotent: true}, plugin: transient, wantRetry: true,
		},
		{
			name:    "a transport failure is classified by the plugin's transient status",
			entry:   registry.Provider{Transport: registry.Transport{Retry: registry.Retry{DefaultAttempts: 3}}},
			attempt: Attempt{Retries: 0, Status: 0}, plugin: transient, wantRetry: true,
		},
		{
			name: "a per-status override is what bounds its own status",
			entry: registry.Provider{Transport: registry.Transport{Retry: registry.Retry{
				DefaultAttempts: 5, ByStatus: map[int]int{429: 1},
			}}},
			attempt: Attempt{Retries: 0, Status: 429}, plugin: transient, wantRetry: false,
		},
		{
			// A second retry is only available when the default is three attempts,
			// which is what makes this case pin the default rather than restate it.
			name:    "an entry declaring no retry policy gets the documented default",
			entry:   registry.Provider{},
			attempt: Attempt{Retries: 1, Status: 503, Idempotent: true}, plugin: transient, wantRetry: true,
		},
		{
			name:    "a nil plugin is never retried",
			entry:   registry.Provider{},
			attempt: Attempt{Retries: 0, Status: 429}, plugin: nil, wantRetry: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DecideRetry(tc.entry, tc.plugin, tc.attempt)
			if got.Retry != tc.wantRetry {
				t.Fatalf("Retry = %v, want %v", got.Retry, tc.wantRetry)
			}
			if tc.wantRetry && got.After < 0 {
				t.Fatalf("After = %v, want a non-negative wait", got.After)
			}
		})
	}
}

// TestBackoffBounds pins the backoff window: it grows with the attempt number and
// stays inside the ceiling, so a long chain cannot park a request for minutes.
func TestBackoffBounds(t *testing.T) {
	cases := []struct {
		retries int
		maxWait int64
	}{
		{retries: 0, maxWait: int64(backoffBase)},
		{retries: 1, maxWait: int64(backoffBase) * 2},
		{retries: 5, maxWait: int64(backoffBase) * 32},
		{retries: 40, maxWait: int64(backoffCeiling)},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("retries=%d", tc.retries), func(t *testing.T) {
			for attempt := 0; attempt < 8; attempt++ {
				got := int64(Backoff(tc.retries))
				if got < 0 {
					t.Fatalf("Backoff(%d) = %d, want a non-negative wait", tc.retries, got)
				}
				if got > tc.maxWait {
					t.Fatalf("Backoff(%d) = %d, want at most %d", tc.retries, got, tc.maxWait)
				}
			}
		})
	}
	if got := Backoff(-3); got < 0 {
		t.Fatalf("Backoff(-3) = %d, want it clamped to a non-negative wait", got)
	}
}
