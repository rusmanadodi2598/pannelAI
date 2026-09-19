// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_responses_parts_test.go
// @for       Table-driven tests for how a Responses item's content is rendered.
// @uses      testing, internal/schema.
// @reason    SPEC-API-001 §10 makes the Responses API a P3 deliverable, and its
//
//	content vocabulary differs from OpenAI's in both directions: an
//	image part carries a detail default, an unknown part has no field
//	to live in, and a tool result must arrive as a plain string. Each
//	is a rejection upstream when it is wrong, so each is pinned here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import "testing"

// TestOpenAIToResponses_ContentParts pins the part mapping, including the two
// cases where a part cannot be carried as itself: an image reference with no
// detail takes the API default, and a part type the Responses vocabulary has no
// field for travels as its JSON text so the model still sees it.
func TestOpenAIToResponses_ContentParts(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantType   string
		wantText   string
		wantImage  string
		wantDetail string
	}{
		{
			name:       "an image part carries its url and the default detail",
			body:       `{"model":"m","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://example.test/i.png"}}]}]}`,
			wantType:   ItemInputImage,
			wantImage:  "https://example.test/i.png",
			wantDetail: "auto",
		},
		{
			name:       "an explicit detail is kept",
			body:       `{"model":"m","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://example.test/i.png","detail":"high"}}]}]}`,
			wantType:   ItemInputImage,
			wantImage:  "https://example.test/i.png",
			wantDetail: "high",
		},
		{
			name:     "an image part with no reference degrades to empty text",
			body:     `{"model":"m","messages":[{"role":"user","content":[{"type":"image_url"}]}]}`,
			wantType: ItemInputText,
			wantText: "",
		},
		{
			name:     "an unknown part type is carried as its json text",
			body:     `{"model":"m","messages":[{"role":"user","content":[{"type":"input_audio","input_audio":{"data":"AAAA"}}]}]}`,
			wantType: ItemInputText,
			wantText: `{"type":"input_audio","input_audio":{"data":"AAAA"}}`,
		},
		{
			name:     "a text part keeps its text verbatim",
			body:     `{"model":"m","messages":[{"role":"user","content":[{"type":"text","text":"line one\nline two"}]}]}`,
			wantType: ItemInputText,
			wantText: "line one\nline two",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := OpenAIToResponses(chatRequest(t, tc.body), "m", false)
			if len(got.Input) != 1 || len(got.Input[0].Content) != 1 {
				t.Fatalf("want one item with one part, got %+v", got.Input)
			}
			part := got.Input[0].Content[0]
			if part.Type != tc.wantType {
				t.Fatalf("part type = %q, want %q", part.Type, tc.wantType)
			}
			if part.Text != tc.wantText {
				t.Fatalf("part text = %q, want %q", part.Text, tc.wantText)
			}
			if part.ImageURL != tc.wantImage {
				t.Fatalf("image url = %q, want %q", part.ImageURL, tc.wantImage)
			}
			if tc.wantImage != "" && part.Detail != tc.wantDetail {
				t.Fatalf("detail = %q, want %q", part.Detail, tc.wantDetail)
			}
		})
	}
}

// TestOpenAIToResponses_ToolOutputShapes pins how a tool result is rendered: the
// API requires a string, so parts are concatenated with nothing between them
// (a separator would corrupt a structured payload) and a non-text part is
// carried as its JSON text rather than dropped.
func TestOpenAIToResponses_ToolOutputShapes(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "a bare string result is forwarded as is",
			body: `{"model":"m","messages":[{"role":"tool","tool_call_id":"c1","content":"42"}]}`,
			want: "42",
		},
		{
			name: "text parts are concatenated with no separator",
			body: `{"model":"m","messages":[{"role":"tool","tool_call_id":"c1","content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]}]}`,
			want: "ab",
		},
		{
			name: "an empty result is forwarded as an empty string",
			body: `{"model":"m","messages":[{"role":"tool","tool_call_id":"c1","content":""}]}`,
			want: "",
		},
		{
			name: "a json payload survives the round trip intact",
			body: `{"model":"m","messages":[{"role":"tool","tool_call_id":"c1","content":"{\"ok\":true,\"rows\":[1,2]}"}]}`,
			want: `{"ok":true,"rows":[1,2]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := OpenAIToResponses(chatRequest(t, tc.body), "m", false)
			if len(got.Input) != 1 || got.Input[0].Type != ItemFunctionCallOut {
				t.Fatalf("want one function_call_output item, got %+v", got.Input)
			}
			if got.Input[0].Output != tc.want {
				t.Fatalf("output = %q, want %q", got.Input[0].Output, tc.want)
			}
			if got.Input[0].CallID != "c1" {
				t.Fatalf("call_id = %q, want the tool_call_id it answers", got.Input[0].CallID)
			}
		})
	}
}
