// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_stream_openai_sanitize_test.go
// @for       The forwarded chunk's shape on the way to the client: the empty
//
//	members a vendor writes on every frame, and the cut at the caller's
//	stop sequences.
//
// @uses      strings, testing, internal/dataplane StreamState.
// @reason    Both halves are only visible in a stream, and both were measured
//
//	live rather than inferred: a five-token answer arrived with eleven
//	truthy-empty `tool_calls` members and an empty-string finish reason,
//	and a `stop` sequence changed nothing at all. The payloads below are
//	written as that vendor writes them, noise included, so the test fails
//	if the noise comes back.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-30
package dataplane

import (
	"strings"
	"testing"
)

// vendorChunk is one frame as codebuddy-intl writes it: every member present
// whether or not it carries anything, and the finish reason an empty string until
// the close.
func vendorChunk(delta string, finish string) string {
	reason := "null"
	if finish != "" {
		reason = `"` + finish + `"`
	}
	return `{"id":"cmb-1","object":"chat.completion.chunk","created":1700000000,` +
		`"model":"deepseek-v4.1-flash","choices":[{"index":0,"delta":{` + delta +
		`},"logprobs":null,"finish_reason":` + reason + `}],"usage":null}`
}

// noisyDelta is the vendor's noise without a thinking fragment, so a test can add a
// valued one without writing the member twice, a JSON object with a repeated key
// decodes to its last value, which is how this fixture once hid a real fragment.
const noisyDelta = `"content":"hello","function_call":null,` +
	`"refusal":"","tool_calls":[],"extra_fields":null`

// noisyDeltaWithThinking is the same frame with an empty thinking fragment added,
// which is what every frame this vendor sends carries.
const noisyDeltaWithThinking = `"reasoning_content":"",` + noisyDelta

func streamFrames(t *testing.T, state *StreamState, payloads ...string) string {
	t.Helper()
	var frames [][]byte
	for _, payload := range payloads {
		frames = append(frames, state.Frames(TargetOpenAI, []byte(payload))...)
	}
	frames = append(frames, state.Finish()...)
	return frameText(frames)
}

func TestOpenAIStream_ForwardedFrameLosesTheVendorsEmptyMembers(t *testing.T) {
	state := NewStreamState("", "codebuddy-intl/deepseek-v4.1-flash", 1700000000, false)
	rendered := streamFrames(t, state, vendorChunk(noisyDeltaWithThinking, ""))

	for _, member := range []string{`"function_call"`, `"refusal"`, `"extra_fields"`, `"tool_calls"`, `"reasoning_content"`} {
		if strings.Contains(rendered, member) {
			t.Fatalf("%s reached the client, want it dropped: %s", member, rendered)
		}
	}
	if strings.Contains(rendered, `"finish_reason":""`) {
		t.Fatalf("an empty finish reason reached the client: %s", rendered)
	}
	// The text that was actually there is untouched, and the frame still counts
	// as exactly one answer.
	if !strings.Contains(rendered, `"content":"hello"`) {
		t.Fatalf("the content was dropped along with the noise: %s", rendered)
	}
	if got := strings.Count(rendered, `"content":"hello"`); got != 1 {
		t.Fatalf("content frames = %d, want 1", got)
	}
}

func TestOpenAIStream_ValuedMembersSurviveThePrune(t *testing.T) {
	state := NewStreamState("", "m", 1700000000, false)
	rendered := streamFrames(t, state,
		vendorChunk(`"reasoning_content":"because",`+noisyDelta, ""),
		vendorChunk(`"content":"a","tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"get_weather","arguments":""}}]`, "tool_calls"),
	)
	if !strings.Contains(rendered, `"reasoning_content":"because"`) {
		t.Fatalf("a thinking fragment was dropped: %s", rendered)
	}
	if !strings.Contains(rendered, `"tool_calls":[{"index":0`) {
		t.Fatalf("a real tool call was dropped as noise: %s", rendered)
	}
	if !strings.Contains(rendered, `"finish_reason":"tool_calls"`) {
		t.Fatalf("the close lost its reason: %s", rendered)
	}
}

func TestOpenAIStream_DropsADeprecatedCallBlockThatNamesNothing(t *testing.T) {
	// This vendor's closing frame carries `function_call` as an object of two
	// empty strings, which is neither `{}` nor null and would otherwise read as a
	// call. A block that names a function is kept.
	empty := NewStreamState("", "m", 1700000000, false)
	rendered := streamFrames(t, empty,
		vendorChunk(`"content":"x","function_call":{"name":"","arguments":""}`, ""))
	if strings.Contains(rendered, "function_call") {
		t.Fatalf("an empty deprecated call block reached the client: %s", rendered)
	}

	valued := NewStreamState("", "m", 1700000000, false)
	kept := streamFrames(t, valued,
		vendorChunk(`"function_call":{"name":"get_weather","arguments":""}`, ""))
	if !strings.Contains(kept, `"function_call":{"name":"get_weather"`) {
		t.Fatalf("a real function call was dropped as noise: %s", kept)
	}
}

func TestOpenAIStream_CutsTheAnswerAtTheStopSequence(t *testing.T) {
	state := NewStreamState("", "m", 1700000000, false).WithStopSequences([]string{"STOPHERE"})
	rendered := streamFrames(t, state,
		vendorChunk(`"content":"keep me"`, ""),
		vendorChunk(`"content":" STOPHERE drop"`, ""),
		vendorChunk(`"content":" me too"`, ""),
		vendorChunk(`"content":""`, "stop"),
	)
	if strings.Contains(rendered, "drop") || strings.Contains(rendered, "me too") {
		t.Fatalf("text after the marker reached the client: %s", rendered)
	}
	if !strings.Contains(rendered, `"content":"keep me"`) {
		t.Fatalf("the text before the marker was lost: %s", rendered)
	}
	if got := strings.Count(rendered, `"finish_reason":"stop"`); got != 1 {
		t.Fatalf("stop reasons = %d, want exactly one: %s", got, rendered)
	}
	if !strings.HasSuffix(rendered, "data: [DONE]\n\n") {
		t.Fatalf("the stream does not terminate: %q", rendered)
	}
}

func TestOpenAIStream_CutHoldsAMarkerSplitAcrossFrames(t *testing.T) {
	state := NewStreamState("", "m", 1700000000, false).WithStopSequences([]string{"STOPHERE"})
	rendered := streamFrames(t, state,
		vendorChunk(`"content":"AB STOP"`, ""),
		vendorChunk(`"content":"HERE C"`, ""),
	)
	if strings.Contains(rendered, "STOP") {
		t.Fatalf("a piece of the marker reached the client: %s", rendered)
	}
	if !strings.Contains(rendered, `"content":"AB "`) {
		t.Fatalf("the text before the marker was not shown: %s", rendered)
	}
}

func TestOpenAIStream_ReleasesHeldTextWhenNoMarkerArrives(t *testing.T) {
	state := NewStreamState("", "m", 1700000000, false).WithStopSequences([]string{"STOPHERE"})
	rendered := streamFrames(t, state,
		vendorChunk(`"content":"the end is near STOP"`, ""),
		vendorChunk(`"content":""`, "stop"),
	)
	// "STOP" was withheld as a possible marker and was never proven one, so the
	// caller must still receive it.
	if !strings.Contains(rendered, `"content":"STOP"`) {
		t.Fatalf("the held-back tail was dropped rather than released: %s", rendered)
	}
}

func TestOpenAIStream_CutReportsStopEvenWhenTheVendorSaysLength(t *testing.T) {
	state := NewStreamState("", "m", 1700000000, false).WithStopSequences([]string{"X"})
	rendered := streamFrames(t, state,
		vendorChunk(`"content":"cutX here"`, ""),
		vendorChunk(`"content":"more"`, "length"),
	)
	if strings.Contains(rendered, `"finish_reason":"length"`) {
		t.Fatalf("the vendor's ceiling was reported for an answer the caller ended: %s", rendered)
	}
	if !strings.Contains(rendered, `"finish_reason":"stop"`) {
		t.Fatalf("the cut was not reported as a stop: %s", rendered)
	}
}
