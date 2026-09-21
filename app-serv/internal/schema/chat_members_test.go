// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/chat_members_test.go
// @for       Table-driven coverage of the nested chat members: the content
// union, content parts, tool declarations, and the tool_choice control.
// @uses      testing.
// @reason    F1 of docs/DRAFT/009-PLAYGROUND-CHAT-ENDPOINT-READINESS.md requires
// a benign control, a boundary, and a malformed case per rule. These are the
// nested members a struct tag cannot reach, so each closed set and each
// discriminator is pinned here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-21
package schema

import "testing"

// TestChatRequest_ContentUnion pins the two permitted content shapes and the
// one legitimate absence: an assistant turn that carries tool calls is what a
// tool-calling conversation sends, and refusing it would break the flow.
func TestChatRequest_ContentUnion(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		valid bool
	}{
		{name: "a bare string", body: chatMessage(`{"role":"user","content":"hi"}`), valid: true},
		{name: "a text part", body: chatMessage(`{"role":"user","content":[{"type":"text","text":"hi"}]}`), valid: true},
		{name: "assistant with tool calls and no content", body: chatMessage(`{"role":"assistant","tool_calls":[{"id":"c1","type":"function","function":{"name":"lookup","arguments":"{}"}}]}`), valid: true},
		{name: "a user turn with no content", body: chatMessage(`{"role":"user"}`)},
		{name: "an assistant turn with no content and no calls", body: chatMessage(`{"role":"assistant"}`)},
		{name: "a tool turn with no content", body: chatMessage(`{"role":"tool","tool_call_id":"c1"}`)},
		{name: "an empty string", body: chatMessage(`{"role":"user","content":""}`)},
		{name: "an empty part array", body: chatMessage(`{"role":"user","content":[]}`)},
		{name: "a null content", body: chatMessage(`{"role":"user","content":null}`)},
		{name: "a numeric content", body: chatMessage(`{"role":"user","content":42}`)},
		{name: "a text part with empty text", body: chatMessage(`{"role":"user","content":[{"type":"text","text":""}]}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertChatValidity(t, tc.body, tc.valid)
		})
	}
}

// TestChatRequest_ContentParts pins each part against its declared type, so a
// part whose payload does not match its discriminator never reaches a
// translator that would render it as an empty block.
func TestChatRequest_ContentParts(t *testing.T) {
	cases := []struct {
		name  string
		part  string
		valid bool
	}{
		{name: "image with a url", part: `{"type":"image_url","image_url":{"url":"https://example.test/a.png"}}`, valid: true},
		{name: "image with detail auto", part: `{"type":"image_url","image_url":{"url":"https://example.test/a.png","detail":"auto"}}`, valid: true},
		{name: "image with detail low", part: `{"type":"image_url","image_url":{"url":"https://example.test/a.png","detail":"low"}}`, valid: true},
		{name: "image with an unknown detail", part: `{"type":"image_url","image_url":{"url":"https://example.test/a.png","detail":"banana"}}`},
		{name: "image without a url", part: `{"type":"image_url","image_url":{}}`},
		{name: "image with a blank url", part: `{"type":"image_url","image_url":{"url":"   "}}`},
		{name: "image with no image_url member", part: `{"type":"image_url"}`},
		{name: "input_audio as an object", part: `{"type":"input_audio","input_audio":{"data":"aGk=","format":"wav"}}`, valid: true},
		{name: "input_audio as a string", part: `{"type":"input_audio","input_audio":"aGk="}`},
		{name: "input_audio absent", part: `{"type":"input_audio"}`},
		{name: "file as an object", part: `{"type":"file","file":{"file_id":"f1"}}`, valid: true},
		{name: "file as an array", part: `{"type":"file","file":[]}`},
		{name: "file absent", part: `{"type":"file"}`},
		{name: "an unknown part type", part: `{"type":"banana","text":"hi"}`},
		{name: "a part with no type", part: `{"text":"hi"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := chatMessage(`{"role":"user","content":[` + tc.part + `]}`)
			assertChatValidity(t, body, tc.valid)
		})
	}
}

// TestChatRequest_Tools pins the declaration a provider reads: the wrapper type,
// the name bounds, and the parameter schema, which must be an object because a
// provider rejects anything else.
func TestChatRequest_Tools(t *testing.T) {
	cases := []struct {
		name  string
		tool  string
		valid bool
	}{
		{name: "a declared function", tool: `{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}`, valid: true},
		{name: "a function without parameters", tool: `{"type":"function","function":{"name":"lookup"}}`, valid: true},
		{name: "a function with an empty parameter object", tool: `{"type":"function","function":{"name":"lookup","parameters":{}}}`, valid: true},
		{name: "a function with a scalar parameter schema", tool: `{"type":"function","function":{"name":"lookup","parameters":"text"}}`},
		{name: "a function with an array parameter schema", tool: `{"type":"function","function":{"name":"lookup","parameters":[]}}`},
		{name: "an unknown tool type", tool: `{"type":"banana","function":{"name":"lookup"}}`},
		{name: "a tool with no type", tool: `{"function":{"name":"lookup"}}`},
		{name: "a function with no name", tool: `{"type":"function","function":{}}`},
		{name: "a function name at the length limit", tool: `{"type":"function","function":{"name":"` + repeat("n", 128) + `"}}`, valid: true},
		{name: "a function name over the length limit", tool: `{"type":"function","function":{"name":"` + repeat("n", 129) + `"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := chatWith(`"tools":[` + tc.tool + `]`)
			assertChatValidity(t, body, tc.valid)
		})
	}
}

// TestChatRequest_ToolChoice pins the control against the vocabulary the
// translator can map, so an unrecognised value is refused here rather than
// forwarded to a provider that answers with an opaque 400.
func TestChatRequest_ToolChoice(t *testing.T) {
	cases := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "auto", value: `"auto"`, valid: true},
		{name: "none", value: `"none"`, valid: true},
		{name: "required", value: `"required"`, valid: true},
		{name: "an unknown string", value: `"banana"`},
		{name: "a forced function", value: `{"type":"function","function":{"name":"lookup"}}`, valid: true},
		{name: "a forced function with no name", value: `{"type":"function","function":{}}`},
		{name: "an unknown object type", value: `{"type":"banana"}`},
		{name: "an anthropic tool block", value: `{"type":"tool","name":"lookup"}`, valid: true},
		{name: "a number", value: `42`},
		{name: "an array", value: `["auto"]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := chatWith(`"tool_choice":` + tc.value)
			assertChatValidity(t, body, tc.valid)
		})
	}
}
