// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_responses_client_parts_test.go
// @for       Table-driven tests for content-part and tool-declaration mapping.
// @uses      testing, internal/schema.
// @reason    SPEC-API-001 §7.15 serves POST /api/v1/responses, whose image part
//
//	carries a bare URL string rather than OpenAI's object and whose
//	tool vocabulary is flattened. Both differ from the chat wire in a
//	way that silently loses a client's input when mapped carelessly.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestResponsesToOpenAI_ImageParts pins the image mapping: the URL is a bare
// string on the Responses wire, a file reference stands in for one, and an
// omitted detail takes the API default rather than being sent as empty.
func TestResponsesToOpenAI_ImageParts(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantURL    string
		wantDetail string
	}{
		{
			name:       "an image url takes the default detail",
			body:       `{"model":"m","input":[{"type":"message","role":"user","content":[{"type":"input_image","image_url":"https://example.test/i.png"}]}]}`,
			wantURL:    "https://example.test/i.png",
			wantDetail: "auto",
		},
		{
			name:       "an explicit detail is kept",
			body:       `{"model":"m","input":[{"type":"message","role":"user","content":[{"type":"input_image","image_url":"https://example.test/i.png","detail":"high"}]}]}`,
			wantURL:    "https://example.test/i.png",
			wantDetail: "high",
		},
		{
			name:       "a file reference stands in for the url",
			body:       `{"model":"m","input":[{"type":"message","role":"user","content":[{"type":"input_image","file_id":"file_123"}]}]}`,
			wantURL:    "file_123",
			wantDetail: "auto",
		},
		{
			name:       "an image part with neither is sent empty rather than dropped",
			body:       `{"model":"m","input":[{"type":"message","role":"user","content":[{"type":"input_image"}]}]}`,
			wantURL:    "",
			wantDetail: "auto",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResponsesToOpenAI(responsesRequest(t, tc.body), "m", false)
			parts := messageParts(t, got.Messages)
			if len(parts) != 1 || parts[0].Type != schema.PartImageURL {
				t.Fatalf("want one image part, got %+v", parts)
			}
			if parts[0].ImageURL == nil {
				t.Fatal("image reference = nil, want a url object")
			}
			if parts[0].ImageURL.URL != tc.wantURL {
				t.Fatalf("url = %q, want %q", parts[0].ImageURL.URL, tc.wantURL)
			}
			if parts[0].ImageURL.Detail != tc.wantDetail {
				t.Fatalf("detail = %q, want %q", parts[0].ImageURL.Detail, tc.wantDetail)
			}
		})
	}
}

// messageParts reads the first message's parts, failing when the content is not
// a part array.
func messageParts(t *testing.T, messages []schema.ChatMessage) []schema.ContentPart {
	t.Helper()
	if len(messages) != 1 {
		t.Fatalf("messages = %d, want one", len(messages))
	}
	if messages[0].Content.Parts == nil {
		t.Fatalf("content = %q, want a part array", messages[0].Content.Text)
	}
	return messages[0].Content.Parts
}

// TestResponsesToOpenAI_UnknownPart pins that a part kind the chat wire has no
// field for keeps its own type name rather than being rewritten into a text
// block: the upstream sees a kind it may understand, and a client's input is not
// silently reinterpreted.
func TestResponsesToOpenAI_UnknownPart(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantType string
		wantText string
	}{
		{
			name:     "an unknown part keeps its type and text",
			body:     `{"model":"m","input":[{"type":"message","role":"user","content":[{"type":"input_audio","text":"AAAA"}]}]}`,
			wantType: "input_audio",
			wantText: "AAAA",
		},
		{
			name:     "a text part is mapped to the chat spelling",
			body:     `{"model":"m","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]}`,
			wantType: schema.PartText,
			wantText: "hi",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResponsesToOpenAI(responsesRequest(t, tc.body), "m", false)
			parts := messageParts(t, got.Messages)
			if len(parts) != 1 {
				t.Fatalf("parts = %d, want one", len(parts))
			}
			if parts[0].Type != tc.wantType {
				t.Fatalf("part type = %q, want %q", parts[0].Type, tc.wantType)
			}
			if parts[0].Text != tc.wantText {
				t.Fatalf("part text = %q, want %q", parts[0].Text, tc.wantText)
			}
		})
	}
}

// TestResponsesToOpenAI_Tools pins the tool-declaration rules.
//
// A flattened declaration is wrapped into OpenAI's `function` shape, one that
// already carries the wrapper is kept, and a declaration with no name at all is
// skipped: the Responses API's hosted tools carry none, and a chat upstream
// rejects a function declaration without a name.
func TestResponsesToOpenAI_Tools(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantTools  int
		wantName   string
		wantSchema string
	}{
		{
			name:      "no tools leaves the field unset",
			body:      `{"model":"m","input":"hi"}`,
			wantTools: 0,
		},
		{
			name:       "a flattened declaration is wrapped",
			body:       `{"model":"m","input":"hi","tools":[{"type":"function","name":"lookup","description":"d","parameters":{"type":"object","properties":{"q":{"type":"string"}}}}]}`,
			wantTools:  1,
			wantName:   "lookup",
			wantSchema: `{"type":"object","properties":{"q":{"type":"string"}}}`,
		},
		{
			name:       "a declaration already carrying a wrapper is kept",
			body:       `{"model":"m","input":"hi","tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object","properties":{}}}}]}`,
			wantTools:  1,
			wantName:   "lookup",
			wantSchema: `{"type":"object","properties":{}}`,
		},
		{
			name:       "a nameless hosted tool is skipped",
			body:       `{"model":"m","input":"hi","tools":[{"type":"request_user_input"},{"type":"function","name":"lookup"}]}`,
			wantTools:  1,
			wantName:   "lookup",
			wantSchema: `{"type":"object","properties":{}}`,
		},
		{
			name:       "an absent parameter schema gains the required properties member",
			body:       `{"model":"m","input":"hi","tools":[{"type":"function","name":"lookup"}]}`,
			wantTools:  1,
			wantName:   "lookup",
			wantSchema: `{"type":"object","properties":{}}`,
		},
		{
			name:       "an object schema without properties gains one",
			body:       `{"model":"m","input":"hi","tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}]}`,
			wantTools:  1,
			wantName:   "lookup",
			wantSchema: `{"type":"object","properties":{}}`,
		},
		{
			name:       "a non-object schema is left alone",
			body:       `{"model":"m","input":"hi","tools":[{"type":"function","name":"lookup","parameters":{"type":"string"}}]}`,
			wantTools:  1,
			wantName:   "lookup",
			wantSchema: `{"type":"string"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResponsesToOpenAI(responsesRequest(t, tc.body), "m", false)
			if len(got.Tools) != tc.wantTools {
				t.Fatalf("tools = %d, want %d (%+v)", len(got.Tools), tc.wantTools, got.Tools)
			}
			if tc.wantTools == 0 {
				return
			}
			tool := got.Tools[0]
			if tool.Type != schema.BlockFunction {
				t.Fatalf("tool type = %q, want function", tool.Type)
			}
			if tool.Function.Name != tc.wantName {
				t.Fatalf("tool name = %q, want %q", tool.Function.Name, tc.wantName)
			}
			if got, want := string(tool.Function.InputSchema()), tc.wantSchema; !sameJSON(t, got, want) {
				t.Fatalf("parameters = %s, want %s", got, want)
			}
		})
	}
}

// sameJSON reports whether two JSON documents carry the same value, which is
// what a parameter schema's equality means: re-encoding a decoded object orders
// its members differently without changing what they say.
func sameJSON(t *testing.T, left, right string) bool {
	t.Helper()
	leftObject, leftOK := decodeObject([]byte(left))
	rightObject, rightOK := decodeObject([]byte(right))
	if !leftOK || !rightOK {
		return left == right
	}
	leftEncoded, leftErr := json.Marshal(leftObject)
	rightEncoded, rightErr := json.Marshal(rightObject)
	if leftErr != nil || rightErr != nil {
		return left == right
	}
	return bytes.Equal(leftEncoded, rightEncoded)
}
