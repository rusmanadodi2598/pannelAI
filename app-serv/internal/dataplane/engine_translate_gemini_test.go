// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_translate_gemini_test.go
// @for       The Gemini target on both directions of the engine's translation seams.
// @uses      encoding/json, strings, testing, internal/schema.
// @reason    A fourth target is a compile-clean change: the engine's per-target switches each end in a default that assumes an OpenAI upstream, so a target added to one list and not the others routes requests and silently mis-reads answers, bills every call as zero tokens, or hands a Claude client a Gemini body. These tests walk the seams one by one, and pin the one property no switch can hide: Gemini carries its model in the URL, never in the payload.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-17
package dataplane

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

const geminiAnswerBody = `{"candidates":[{"content":{"role":"model","parts":[{"text":"pong"}]},` +
	`"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":11,"candidatesTokenCount":5,"totalTokenCount":16}}`

func geminiChatRequest(t *testing.T, raw string) Request {
	t.Helper()
	var chat schema.ChatRequest
	if err := json.Unmarshal([]byte(raw), &chat); err != nil {
		t.Fatalf("decoding the chat body: %v", err)
	}
	return Request{ClientFormat: schema.FormatOpenAI, Chat: &chat, Route: RouteChatCompletions}
}

func geminiMessagesRequest(t *testing.T, raw string) Request {
	t.Helper()
	var messages schema.MessagesRequest
	if err := json.Unmarshal([]byte(raw), &messages); err != nil {
		t.Fatalf("decoding the messages body: %v", err)
	}
	return Request{ClientFormat: schema.FormatAnthropic, Messages: &messages, Route: RouteMessages}
}

func geminiResponsesRequest(t *testing.T, raw string) Request {
	t.Helper()
	var responses schema.ResponsesRequest
	if err := json.Unmarshal([]byte(raw), &responses); err != nil {
		t.Fatalf("decoding the responses body: %v", err)
	}
	return Request{ClientFormat: schema.FormatOpenAIResponses, Responses: &responses, Route: RouteResponses}
}

// TestGeminiTarget_EveryClientWireReachesIt pins the request direction from each
// of the three client formats. A missing arm answers PROVIDER_NOT_ROUTABLE, so a
// provider the operator configured would work for one client and refuse another.
func TestGeminiTarget_EveryClientWireReachesIt(t *testing.T) {
	cases := []struct {
		name    string
		request Request
		wantIn  []string
	}{
		{
			name: "openai client",
			request: geminiChatRequest(t, `{"model":"gemini/gemini-2.5-flash","messages":[`+
				`{"role":"system","content":"be terse"},{"role":"user","content":"ping"}]}`),
			wantIn: []string{`"contents"`, `"systemInstruction"`, "ping", "be terse"},
		},
		{
			name: "anthropic client",
			request: geminiMessagesRequest(t, `{"model":"gemini/gemini-2.5-flash","max_tokens":32,`+
				`"system":"be terse","messages":[{"role":"user","content":"ping"}]}`),
			wantIn: []string{`"contents"`, `"systemInstruction"`, "ping", "be terse", `"maxOutputTokens":32`},
		},
		{
			name: "responses client",
			request: geminiResponsesRequest(t, `{"model":"gemini/gemini-2.5-flash","instructions":"be terse",`+
				`"input":"ping","max_output_tokens":32}`),
			wantIn: []string{`"contents"`, `"systemInstruction"`, "ping", "be terse"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			translated, err := translateUpstreamBody(tc.request, Resolution{
				ModelID: "gemini-2.5-flash", UpstreamID: "gemini-2.5-flash", Target: TargetGemini,
			})
			if err != nil {
				t.Fatalf("translateUpstreamBody() error = %v", err)
			}
			body := string(translated)
			for _, want := range tc.wantIn {
				if !strings.Contains(body, want) {
					t.Fatalf("body = %s, want it to carry %s", body, want)
				}
			}
			// Gemini names the model in the request URL, so a payload that carried
			// one would be a second, contradictable source for the same fact.
			if strings.Contains(body, `"model"`) {
				t.Fatalf("body = %s, want no model member: the model belongs in the URL", body)
			}
		})
	}
}

func TestGeminiTarget_UsageIsReadFromUsageMetadata(t *testing.T) {
	usage := readUsage([]byte(geminiAnswerBody), TargetGemini)
	if usage == nil {
		t.Fatal("usage = nil, want the numbers under usageMetadata")
	}
	if usage.PromptTokens != 11 || usage.CompletionTokens != 5 || usage.TotalTokens != 16 {
		t.Fatalf("usage = %+v, want 11/5/16 rather than the zero an OpenAI-key lookup would return", usage)
	}
}

// TestGeminiTarget_AnswerReachesEachClientWire pins the answer direction. The
// default arm of these three functions reads a Claude body, so without an arm a
// Gemini "candidates" payload would come back as an empty completion rather than
// an error, which is the failure mode worth refusing in a test.
func TestGeminiTarget_AnswerReachesEachClientWire(t *testing.T) {
	resolution := Resolution{
		ModelID: "gemini-2.5-flash", UpstreamID: "gemini-2.5-flash", Target: TargetGemini,
		Requested: "gemini-2.5-flash",
	}

	openAI, err := openAIAnswer([]byte(geminiAnswerBody), resolution, 1700000000)
	if err != nil {
		t.Fatalf("openAIAnswer() error = %v", err)
	}
	if got := openAI.Choices[0].Message.Content.TextContent(); got != "pong" {
		t.Fatalf("openai content = %q, want the candidate's text", got)
	}
	if openAI.Usage == nil || openAI.Usage.TotalTokens != 16 {
		t.Fatalf("openai usage = %+v, want the streamed numbers billed", openAI.Usage)
	}

	claude, err := claudeAnswer([]byte(geminiAnswerBody), resolution)
	if err != nil {
		t.Fatalf("claudeAnswer() error = %v", err)
	}
	if len(claude.Content) == 0 || !strings.Contains(string(mustJSON(claude.Content)), "pong") {
		t.Fatalf("claude blocks = %d, want the answer text carried as a content block", len(claude.Content))
	}
	if claude.StopReason != StopEndTurn {
		t.Fatalf("claude stop_reason = %q, want the Anthropic spelling of a finished turn", claude.StopReason)
	}

	responses, err := responsesClientAnswer([]byte(geminiAnswerBody), resolution, 1700000000)
	if err != nil {
		t.Fatalf("responsesClientAnswer() error = %v", err)
	}
	if !strings.Contains(string(mustJSON(responses)), "pong") {
		t.Fatal("responses answer lost the candidate text")
	}
}

// TestGeminiTarget_StreamFramesReachEachClientWire pins the three stream states.
// Their default arm frames a Gemini payload as if it were an OpenAI chunk, which
// renders an answer the client never sees, in a frame count a client can prove.
func TestGeminiTarget_StreamFramesReachEachClientWire(t *testing.T) {
	payload := []byte(`{"candidates":[{"content":{"parts":[{"text":"par"}]}}]}`)

	t.Run("openai client", func(t *testing.T) {
		state := NewStreamState("chatcmpl-s", "gemini-2.5-flash", 1700000000, true)
		frames := state.Frames(TargetGemini, payload)
		if len(frames) != 1 || !strings.Contains(string(frames[0]), `"content":"par"`) {
			t.Fatalf("frames = %s, want the delta text on the OpenAI wire", frames)
		}
	})

	t.Run("anthropic client", func(t *testing.T) {
		state := NewClaudeStreamState("msg_s", "gemini-2.5-flash")
		frames := state.Frames(TargetGemini, payload)
		body := strings.Join(frameStrings(frames), "\n")
		if !strings.Contains(body, "text_delta") || !strings.Contains(body, "par") {
			t.Fatalf("frames = %s, want the text as an Anthropic content block delta", body)
		}
	})

	t.Run("responses client", func(t *testing.T) {
		state := NewResponsesStreamState("resp_s", "gemini-2.5-flash", 1700000000)
		frames := state.Frames(TargetGemini, payload)
		body := strings.Join(frameStrings(frames), "\n")
		if !strings.Contains(body, "par") {
			t.Fatalf("frames = %s, want the text as a Responses output delta", body)
		}
	})
}

// TestGeminiTarget_FinishFrameClosesTheOpenAIStream pins the frame that carries a
// finish reason: Gemini's STOP has to reach the client as the terminal reason,
// and no second one after it.
func TestGeminiTarget_FinishFrameClosesTheOpenAIStream(t *testing.T) {
	state := NewStreamState("chatcmpl-f", "gemini-2.5-flash", 1700000000, false)

	first := state.Frames(TargetGemini, []byte(`{"candidates":[{"content":{"parts":[{"text":"hi"}]}}]}`))
	if len(first) != 1 {
		t.Fatalf("first frames = %d, want the delta forwarded", len(first))
	}
	closing := state.Frames(TargetGemini, []byte(`{"candidates":[{"finishReason":"MAX_TOKENS"}]}`))
	body := strings.Join(frameStrings(closing), "")
	if !strings.Contains(body, `"finish_reason":"length"`) {
		t.Fatalf("closing frame = %s, want MAX_TOKENS mapped onto OpenAI's length", body)
	}

	frames := state.Frames(TargetGemini, []byte(`{"candidates":[{"finishReason":"STOP"}]}`))
	again := strings.Join(frameStrings(frames), "")
	if strings.Contains(again, `"finish_reason":"stop"`) {
		t.Fatalf("second finish = %s, want the duplicate reason nulled so a client counts one answer", again)
	}
}

func frameStrings(frames [][]byte) []string {
	out := make([]string, 0, len(frames))
	for _, frame := range frames {
		out = append(out, string(frame))
	}
	return out
}
