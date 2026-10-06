// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_openai_gemini_test.go
// @for       The OpenAI-to-Gemini builder and the Gemini usage fold: the shapes the direction promises, tested directly.
// @uses      encoding/json, strings, testing, internal/schema.
// @reason    target.go states the builder "is exercised directly by its own tests"; before this file that was false, the whole cluster had zero callers and zero tests (draft 042 R14). The builder stays unrouted by design (draft 027 F1), so direct tests are the only coverage it can have.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-03
package dataplane

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestOpenAIToGeminiMovesTheSystemPrompt pins the first shape difference: a
// system prompt has no place in contents, so it moves to systemInstruction.
func TestOpenAIToGeminiMovesTheSystemPrompt(t *testing.T) {
	request := schema.ChatRequest{Messages: []schema.ChatMessage{
		{Role: schema.RoleSystem, Content: schema.MessageContent{Text: "be brief"}},
		{Role: schema.RoleUser, Content: schema.MessageContent{Text: "ping"}},
	}}
	out := OpenAIToGemini(request, "gemini-2.5-pro")

	if out.Model != "gemini-2.5-pro" {
		t.Fatalf("Model = %q, want the upstream model", out.Model)
	}
	if out.SystemInstruction == nil || out.SystemInstruction.Parts[0].Text != "be brief" {
		t.Fatalf("SystemInstruction = %+v, want the system prompt", out.SystemInstruction)
	}
	if len(out.Contents) != 1 || out.Contents[0].Role != RoleUser || out.Contents[0].Parts[0].Text != "ping" {
		t.Fatalf("Contents = %+v, want one user turn", out.Contents)
	}
	if len(out.SafetySettings) == 0 {
		t.Fatal("SafetySettings empty, want the reference's OFF set")
	}
}

// TestOpenAIToGeminiKeepsALoneSystemMessageAsATurn pins the reference's reading:
// hoisting the only message would leave contents empty, which Gemini rejects.
func TestOpenAIToGeminiKeepsALoneSystemMessageAsATurn(t *testing.T) {
	request := schema.ChatRequest{Messages: []schema.ChatMessage{
		{Role: schema.RoleSystem, Content: schema.MessageContent{Text: "only"}},
	}}
	out := OpenAIToGemini(request, "m")

	if out.SystemInstruction != nil {
		t.Fatalf("SystemInstruction = %+v, want none for a lone system message", out.SystemInstruction)
	}
	if len(out.Contents) != 1 || out.Contents[0].Role != RoleUser {
		t.Fatalf("Contents = %+v, want the lone message as the user's turn", out.Contents)
	}
}

// TestOpenAIToGeminiPairsAToolResultByName pins the second shape difference: a
// tool result is a functionResponse part in a user turn, paired by call id with
// the name its functionCall declared, and arguments are an object.
func TestOpenAIToGeminiPairsAToolResultByName(t *testing.T) {
	request := schema.ChatRequest{Messages: []schema.ChatMessage{
		{Role: schema.RoleUser, Content: schema.MessageContent{Text: "weather?"}},
		{Role: schema.RoleAssistant, ToolCalls: []schema.ToolCall{{
			ID: "call_1", Function: schema.FunctionCall{Name: "get-weather", Arguments: `{"city":"Jakarta"}`},
		}}},
		{Role: schema.RoleTool, ToolCallID: "call_1", Content: schema.MessageContent{Text: `{"temp":31}`}},
	}}
	out := OpenAIToGemini(request, "m")

	if len(out.Contents) != 3 {
		t.Fatalf("contents = %d, want 3 turns", len(out.Contents))
	}
	assistant := out.Contents[1]
	if assistant.Role != RoleModel || assistant.Parts[0].FunctionCall == nil {
		t.Fatalf("assistant turn = %+v, want a functionCall part", assistant)
	}
	call := assistant.Parts[0].FunctionCall
	if call.Name != "get-weather" {
		t.Fatalf("functionCall name = %q, want the declared name", call.Name)
	}
	if got := call.Args["city"]; got != "Jakarta" {
		t.Fatalf("functionCall args = %+v, want the decoded object", call.Args)
	}
	tool := out.Contents[2]
	if tool.Role != RoleUser || tool.Parts[0].FunctionResponse == nil {
		t.Fatalf("tool turn = %+v, want a functionResponse part in a user turn", tool)
	}
	if tool.Parts[0].FunctionResponse.Name != "get-weather" {
		t.Fatalf("functionResponse name = %q, want the name the call declared", tool.Parts[0].FunctionResponse.Name)
	}
	// The reference carries every parsed result under `result`
	// (openai-to-gemini.js: response: { result: parsed }).
	wrapped, ok := tool.Parts[0].FunctionResponse.Response["result"].(map[string]any)
	if !ok || wrapped["temp"] != float64(31) {
		t.Fatalf("functionResponse = %+v, want the decoded result under `result`", tool.Parts[0].FunctionResponse.Response)
	}
}

// TestOpenAIToGeminiUnparseableArgumentsBecomeAnEmptyObject pins the third shape
// difference: a vector or garbage argument string must not become invalid JSON.
func TestOpenAIToGeminiUnparseableArgumentsBecomeAnEmptyObject(t *testing.T) {
	request := schema.ChatRequest{Messages: []schema.ChatMessage{
		{Role: schema.RoleUser, Content: schema.MessageContent{Text: "x"}},
		{Role: schema.RoleAssistant, ToolCalls: []schema.ToolCall{{
			ID: "c1", Function: schema.FunctionCall{Name: "f", Arguments: "not json"},
		}}},
	}}
	out := OpenAIToGemini(request, "m")

	call := out.Contents[1].Parts[0].FunctionCall
	if call == nil || call.Args == nil || len(call.Args) != 0 {
		t.Fatalf("args = %+v, want an empty object", call)
	}
}

// TestOpenAIToGeminiImages pins the image mapping: a data URI becomes inlineData,
// an http(s) reference becomes fileData, and an unusable reference is dropped.
func TestOpenAIToGeminiImages(t *testing.T) {
	request := schema.ChatRequest{Messages: []schema.ChatMessage{{
		Role: schema.RoleUser,
		Content: schema.MessageContent{Parts: []schema.ContentPart{
			{Type: schema.PartText, Text: "look"},
			{Type: schema.PartImageURL, ImageURL: &schema.ImageURL{URL: "data:image/png;base64,AAAA"}},
			{Type: schema.PartImageURL, ImageURL: &schema.ImageURL{URL: "https://example.com/a.png"}},
			{Type: schema.PartImageURL, ImageURL: &schema.ImageURL{URL: "ftp://nope"}},
		}},
	}}}
	out := OpenAIToGemini(request, "m")

	parts := out.Contents[0].Parts
	if len(parts) != 3 {
		t.Fatalf("parts = %d, want text + inlineData + fileData (the ftp reference dropped)", len(parts))
	}
	if parts[1].InlineData == nil || parts[1].InlineData.MIMEType != "image/png" || parts[1].InlineData.Data != "AAAA" {
		t.Fatalf("inlineData = %+v, want the split data URI", parts[1].InlineData)
	}
	if parts[2].FileData == nil || parts[2].FileData.FileURI != "https://example.com/a.png" {
		t.Fatalf("fileData = %+v, want the remote reference", parts[2].FileData)
	}
}

// TestGeminiUsageToOpenAIFoldsThoughtsAndDerivesTotal pins the accounting: the
// thinking tokens are counted as completion, the cached tokens survive, and a
// missing total is derived from the sum.
func TestGeminiUsageToOpenAIFoldsThoughtsAndDerivesTotal(t *testing.T) {
	usage := GeminiUsageToOpenAI(json.RawMessage(
		`{"promptTokenCount":10,"candidatesTokenCount":4,"thoughtsTokenCount":6,"cachedContentTokenCount":3}`))
	if usage.PromptTokens != 10 || usage.CompletionTokens != 10 || usage.TotalTokens != 20 {
		t.Fatalf("usage = %+v, want prompt 10 / completion 10 (candidates+thoughts) / total 20", usage)
	}
	if usage.PromptTokensDetails == nil || usage.PromptTokensDetails.CachedTokens != 3 {
		t.Fatalf("cached = %+v, want 3", usage.PromptTokensDetails)
	}
	if usage.CompletionTokensDetail == nil || usage.CompletionTokensDetail.ReasoningTokens != 6 {
		t.Fatalf("reasoning = %+v, want 6", usage.CompletionTokensDetail)
	}
}

// TestGeminiUsageToOpenAIDerivesCandidatesFromTotal pins the other derivation: an
// upstream that reports only a total still yields the right completion count.
func TestGeminiUsageToOpenAIDerivesCandidatesFromTotal(t *testing.T) {
	usage := GeminiUsageToOpenAI(json.RawMessage(`{"promptTokenCount":7,"thoughtsTokenCount":2,"totalTokenCount":15}`))
	if usage.CompletionTokens != 8 {
		t.Fatalf("completion = %d, want the 6 derived candidates plus 2 thoughts", usage.CompletionTokens)
	}
}

// TestSanitizeGeminiFunctionName pins Gemini's name rules, which the reference
// sanitizes rather than rejecting.
func TestSanitizeGeminiFunctionName(t *testing.T) {
	cases := []struct{ name, want string }{
		{"", "_unknown"},
		{"1st", "_1st"},
		{"get weather", "get_weather"},
		{"a.b:c-d", "a.b:c-d"},
	}
	for _, tc := range cases {
		if got := sanitizeGeminiFunctionName(tc.name); got != tc.want {
			t.Errorf("sanitize(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
	if got := sanitizeGeminiFunctionName(strings.Repeat("a", 100)); len(got) != geminiMaxFunctionNameLen {
		t.Errorf("long name = %d chars, want the %d cap", len(got), geminiMaxFunctionNameLen)
	}
}
