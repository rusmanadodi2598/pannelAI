// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_gemini_answer_test.go
// @for       The inbound Gemini direction: candidates, parts, finish reasons, usage, and streamed chunks.
// @uses      encoding/json, strings, testing.
// @reason    The request direction of this format was ported long before anything could call it, and the answer direction never was, so a client reading a Gemini upstream would get an OpenAI parser pointed at a body shaped nothing like OpenAI. The shapes worth pinning are the ones that fail quietly: a usage block under a different key reading as zero, a thought part billed as answer text, and a blocked prompt arriving as an empty completion.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-17
package dataplane

import (
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

func TestGeminiToOpenAIResponse_AnswerTextAndFinishReason(t *testing.T) {
	raw := []byte(`{"candidates":[{"content":{"role":"model","parts":[
		{"text":"hello"},{"text":" world"}]},"finishReason":"STOP"}],
		"usageMetadata":{"promptTokenCount":11,"candidatesTokenCount":5,"totalTokenCount":16}}`)

	response, err := GeminiToOpenAIResponse(raw, "gemini-2.5-flash", 1700000000)
	if err != nil {
		t.Fatalf("GeminiToOpenAIResponse() error = %v", err)
	}
	if len(response.Choices) != 1 {
		t.Fatalf("choices = %d, want the one answer the client asked for", len(response.Choices))
	}
	if got := response.Choices[0].Message.Content.TextContent(); got != "hello world" {
		t.Fatalf("content = %q, want the two text parts joined", got)
	}
	if got := response.Choices[0].FinishReason; got != FinishStop {
		t.Fatalf("finish_reason = %q, want %q", got, FinishStop)
	}
	if response.Model != "gemini-2.5-flash" || response.Object != "chat.completion" {
		t.Fatalf("identity = %q/%q, want the name the client asked and the OpenAI object", response.Model, response.Object)
	}
	if response.Usage == nil {
		t.Fatal("usage is nil, want the numbers usageMetadata reported")
	}
	if response.Usage.PromptTokens != 11 || response.Usage.CompletionTokens != 5 || response.Usage.TotalTokens != 16 {
		t.Fatalf("usage = %+v, want 11/5/16 read from usageMetadata rather than the OpenAI key", response.Usage)
	}
}

// TestGeminiToOpenAIResponse_ThoughtsAreNotAnswerText holds the billing and the
// rendering apart: thinking tokens belong to completion tokens in OpenAI's
// accounting, while the thought text belongs to reasoning_content, not content.
func TestGeminiToOpenAIResponse_ThoughtsAreNotAnswerText(t *testing.T) {
	raw := []byte(`{"candidates":[{"content":{"role":"model","parts":[
		{"text":"pondering","thought":true},{"text":"the answer"}]},
		"finishReason":"STOP"}],
		"usageMetadata":{"promptTokenCount":2,"candidatesTokenCount":3,"thoughtsTokenCount":7,"totalTokenCount":12}}`)

	response, err := GeminiToOpenAIResponse(raw, "gemini-2.5-flash", 1700000000)
	if err != nil {
		t.Fatalf("GeminiToOpenAIResponse() error = %v", err)
	}
	message := response.Choices[0].Message
	if message.Content.TextContent() != "the answer" {
		t.Fatalf("content = %q, want only the text the model meant as its answer", message.Content.TextContent())
	}
	if message.Reasoning != "pondering" {
		t.Fatalf("reasoning_content = %q, want the part marked thought", message.Reasoning)
	}
	// 3 candidates + 7 thinking = 10 completion tokens, the sum OpenAI counts.
	if response.Usage.CompletionTokens != 10 {
		t.Fatalf("completion_tokens = %d, want thinking folded into the billed completion", response.Usage.CompletionTokens)
	}
}

func TestGeminiToOpenAIResponse_FunctionCallsBecomeToolCalls(t *testing.T) {
	raw := []byte(`{"candidates":[{"content":{"role":"model","parts":[
		{"text":"looking it up"},
		{"functionCall":{"name":"get_weather","args":{"city":"Bandung","days":2}}},
		{"functionCall":{"name":"get_time","args":{}}}]},
		"finishReason":"STOP"}]}`)

	response, err := GeminiToOpenAIResponse(raw, "gemini-2.5-flash", 1700000000)
	if err != nil {
		t.Fatalf("GeminiToOpenAIResponse() error = %v", err)
	}
	calls := response.Choices[0].Message.ToolCalls
	if len(calls) != 2 {
		t.Fatalf("tool_calls = %d, want both function calls", len(calls))
	}
	if calls[0].Function.Name != "get_weather" {
		t.Fatalf("first call name = %q", calls[0].Function.Name)
	}
	if !strings.Contains(calls[0].Function.Arguments, `"city":"Bandung"`) {
		t.Fatalf("first call arguments = %q, want the args object encoded as the JSON string OpenAI carries", calls[0].Function.Arguments)
	}
	if !strings.Contains(calls[0].Function.Arguments, `"days":2`) {
		t.Fatalf("first call arguments = %q, want numbers kept as numbers", calls[0].Function.Arguments)
	}
	if calls[0].Type != schema.BlockFunction {
		t.Fatalf("call type = %q, want the function block name the client switches on", calls[0].Type)
	}
	if calls[0].ID == "" || calls[1].ID == "" || calls[0].ID == calls[1].ID {
		t.Fatalf("call ids = %q / %q, want one distinct id per call", calls[0].ID, calls[1].ID)
	}
	// A Gemini answer that stops on a tool call has no finish reason of its own,
	// and a client that reads "stop" would never send the result back.
	if response.Choices[0].FinishReason != FinishToolCalls {
		t.Fatalf("finish_reason = %q, want %q for an answer whose work is a call", response.Choices[0].FinishReason, FinishToolCalls)
	}
}

// TestGeminiToOpenAIResponse_BlockedPromptIsNotAnEmptyAnswer keeps a refusal
// readable: the upstream answers 200 with no candidate at all, and the only
// honest thing a completion can say is that content was filtered.
func TestGeminiToOpenAIResponse_BlockedPromptIsNotAnEmptyAnswer(t *testing.T) {
	raw := []byte(`{"promptFeedback":{"blockReason":"SAFETY"},"usageMetadata":{"promptTokenCount":4}}`)

	response, err := GeminiToOpenAIResponse(raw, "gemini-2.5-flash", 1700000000)
	if err != nil {
		t.Fatalf("GeminiToOpenAIResponse() error = %v", err)
	}
	if response.Choices[0].FinishReason != FinishContentFilter {
		t.Fatalf("finish_reason = %q, want the filter reason a blocked prompt is", response.Choices[0].FinishReason)
	}
	if got := response.Choices[0].Message.Content.TextContent(); got != "" {
		t.Fatalf("content = %q, want nothing claimed for a blocked answer", got)
	}
}

func TestGeminiToOpenAIResponse_MapsLengthAndSafetyStops(t *testing.T) {
	cases := []struct {
		reason string
		want   string
	}{
		{reason: "MAX_TOKENS", want: FinishLength},
		{reason: "SAFETY", want: FinishContentFilter},
		{reason: "RECITATION", want: FinishContentFilter},
		{reason: "PROHIBITED_CONTENT", want: FinishContentFilter},
		{reason: "NO_REASON", want: FinishStop},
	}

	for _, tc := range cases {
		t.Run(tc.reason, func(t *testing.T) {
			raw := []byte(`{"candidates":[{"content":{"role":"model","parts":[{"text":"x"}]},` +
				`"finishReason":"` + tc.reason + `"}]}`)
			response, err := GeminiToOpenAIResponse(raw, "gemini-2.5-flash", 1700000000)
			if err != nil {
				t.Fatalf("GeminiToOpenAIResponse() error = %v", err)
			}
			if got := response.Choices[0].FinishReason; got != tc.want {
				t.Fatalf("finish_reason = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGeminiToOpenAIResponse_RefusesAnUnreadableBody(t *testing.T) {
	for _, raw := range []string{`not json`, `[{"candidates":[]}]`, `"a string"`} {
		if _, err := GeminiToOpenAIResponse([]byte(raw), "gemini-2.5-flash", 1700000000); err == nil {
			t.Fatalf("GeminiToOpenAIResponse(%q) error = nil, want the unreadable body refused", raw)
		} else if !strings.Contains(err.Error(), "unreadable") {
			t.Fatalf("GeminiToOpenAIResponse(%q) error = %v, want the reason a client can act on", raw, err)
		}
	}
}
