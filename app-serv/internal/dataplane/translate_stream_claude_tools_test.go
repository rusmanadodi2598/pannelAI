// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_stream_claude_tools_test.go
// @for       The buffered tool arguments of an Anthropic stream fed by an OpenAI upstream.
// @uses      strings, testing, internal/schema.
// @reason    A stream carries a tool's arguments in fragments, and a renamed parameter cannot be decided until the object is whole. The contract therefore changed shape: nothing is delivered between the block's start and its stop, and one corrected delta arrives at the stop. These tests pin that shape, including the repeated-id case, because a block reopened would move the buffer onto an index no stop ever closes.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-14
package dataplane

import (
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// claudeStreamOf feeds OpenAI-wire chunks through the Anthropic framer and
// returns everything the client would receive, Finish included.
func claudeStreamOf(t *testing.T, chunks ...string) string {
	t.Helper()
	state := NewClaudeStreamState("", "claude-x")
	var stream strings.Builder
	for _, chunk := range chunks {
		for _, frame := range state.Frames(TargetOpenAI, []byte(chunk)) {
			stream.Write(frame)
		}
	}
	for _, frame := range state.Finish() {
		stream.Write(frame)
	}
	return stream.String()
}

// toolChunk wraps one tool-call delta as the chunk an OpenAI upstream sends.
func toolChunk(delta string, reason string) string {
	return `{"id":"c1","model":"gpt","choices":[{"index":0,"delta":{"tool_calls":[` + delta + `]},"finish_reason":` + reason + `}]}`
}

// countEvent counts SSE event lines, not the name anywhere in the frame: every
// frame carries the event name twice, once as the line and once as the payload's
// `type`.
func countEvent(stream string, event string) int {
	return strings.Count(stream, "event: "+event+"\n")
}

func TestClaudeStream_BuffersArgumentsUntilTheBlockCloses(t *testing.T) {
	stream := claudeStreamOf(t,
		toolChunk(`{"index":0,"id":"t1","function":{"name":"Bash","arguments":"{\"cmd\":"}}`, "null"),
		toolChunk(`{"index":0,"function":{"arguments":"\"ls -l\"}"}}`, `"tool_calls"`),
	)
	if count := strings.Count(stream, `input_json_delta`); count != 1 {
		t.Fatalf("the stream carried %d argument deltas, want the buffered input delivered once:\n%s", count, stream)
	}
	if !strings.Contains(stream, `{\"command\":\"ls -l\"}`) {
		t.Fatalf("the buffered input was not renamed onto the parameter Bash declares:\n%s", stream)
	}
	if strings.Contains(stream, `cmd`) {
		t.Fatalf("the alias survived into the client's stream:\n%s", stream)
	}
}

func TestClaudeStream_CorrectsTheArgumentsInTheOrderTheClientReadsThem(t *testing.T) {
	stream := claudeStreamOf(t,
		toolChunk(`{"index":0,"id":"t1","function":{"name":"Read","arguments":"{\"path\":\"/x\"}"}}`, `"tool_calls"`),
	)
	start := strings.Index(stream, schema.EventContentBlockStart)
	feed := strings.Index(stream, `input_json_delta`)
	stop := strings.LastIndex(stream, schema.EventContentBlockStop)
	if start < 0 || feed < 0 || stop < 0 {
		t.Fatalf("stream is missing a start, a feed or a stop:\n%s", stream)
	}
	if start >= feed || feed >= stop {
		t.Fatalf("the corrected input must arrive between the block's start and its stop:\n%s", stream)
	}
	if !strings.Contains(stream, `{\"file_path\":\"/x\",\"path\":\"/x\"}`) {
		t.Fatalf("Read was fed a path it cannot dispatch:\n%s", stream)
	}
}

func TestClaudeStream_ARepeatedIDOpensOneBlock(t *testing.T) {
	// Some upstreams repeat the call id on every fragment. Re-opening the block
	// would leak an unclosed start and move the argument buffer onto a block no
	// stop ever closes.
	stream := claudeStreamOf(t,
		toolChunk(`{"index":0,"id":"t1","function":{"name":"Bash","arguments":"{\"cmd\":\"a\"}"}}`, "null"),
		toolChunk(`{"index":0,"id":"t1","function":{"name":"Bash","arguments":" more"}}`, `"tool_calls"`),
	)
	if count := countEvent(stream, schema.EventContentBlockStart); count != 1 {
		t.Fatalf("one tool call opened %d blocks:\n%s", count, stream)
	}
	if count := strings.Count(stream, `input_json_delta`); count != 1 {
		t.Fatalf("%d argument deltas for one block:\n%s", count, stream)
	}
}

func TestClaudeStream_ACallWithNoArgumentsCarriesNoDelta(t *testing.T) {
	stream := claudeStreamOf(t,
		toolChunk(`{"index":0,"id":"t1","function":{"name":"Bash"}}`, `"tool_calls"`),
	)
	if count := strings.Count(stream, `input_json_delta`); count != 0 {
		t.Fatalf("an empty tool input invented a delta:\n%s", stream)
	}
	if count := countEvent(stream, schema.EventContentBlockStop); count != 1 {
		t.Fatalf("the opened block was not closed:\n%s", stream)
	}
}

func TestClaudeStream_TwoCallsFlushInIndexOrder(t *testing.T) {
	stream := claudeStreamOf(t,
		toolChunk(`{"index":0,"id":"t1","function":{"name":"Bash","arguments":"{\"cmd\":\"one\"}"}}`, "null"),
		toolChunk(`{"index":1,"id":"t2","function":{"name":"Bash","arguments":"{\"script\":\"two\"}"}}`, `"tool_calls"`),
	)
	first := strings.Index(stream, `{\"command\":\"one\"}`)
	second := strings.Index(stream, `{\"command\":\"two\"}`)
	if first < 0 || second < 0 || first > second {
		t.Fatalf("two calls flushed out of order (one at %d, two at %d):\n%s", first, second, stream)
	}
	if !strings.Contains(stream, `"index":0`) || !strings.Contains(stream, `"index":1`) {
		t.Fatalf("the two calls share one block index:\n%s", stream)
	}
}

func TestClaudeStream_FinishWithoutAFinishReasonStillDeliversTheInput(t *testing.T) {
	// An upstream that ends the stream without naming a finish reason still has
	// to hand the client what it accumulated: the buffered input is flushed by
	// the block's close, not by the reason. The alias stays alongside the name
	// the tool declares, only Bash's aliases are retired by the reference, and
	// this port keeps that asymmetry rather than quietly improving it.
	stream := claudeStreamOf(t,
		toolChunk(`{"index":0,"id":"t1","function":{"name":"Grep","arguments":"{\"regex\":\"todo\"}"}}`, "null"),
	)
	if !strings.Contains(stream, `{\"pattern\":\"todo\",\"regex\":\"todo\"}`) {
		t.Fatalf("a stream cut short never delivered its tool input:\n%s", stream)
	}
}

func TestClaudeStream_AnthropicUpstreamIsForwardedUnchanged(t *testing.T) {
	// The passthrough is the reference's own asymmetry: an upstream that already
	// speaks Anthropic is forwarded, not rewritten.
	state := NewClaudeStreamState("", "claude-x")
	frames := state.Frames(TargetClaude, []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"cmd\":\"ls\"}"}}`))
	if len(frames) != 1 || !strings.Contains(string(frames[0]), `\"cmd\":\"ls\"`) {
		t.Fatalf("a Claude upstream's own frame was rewritten or dropped: %q", frames)
	}
}
