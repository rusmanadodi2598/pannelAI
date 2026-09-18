// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_openai_claude_test.go
// @for       Table-driven tests for OpenAI ↔ Anthropic request translation.
// @uses      testing, internal/schema.
// @reason    SPEC-API-001 §7.15 makes translation the data plane's core job, so
//
//	every rule it implements — system hoisting, tool ordering, image
//	mapping, and usage folding — is pinned here. Translation is a pure
//	function, so these run with no network and no clock (AGENTS.md §2.1).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// chatRequest decodes an OpenAI body the way the handler does, so a test drives
// the same decode boundary a request does.
func chatRequest(t *testing.T, body string) schema.ChatRequest {
	t.Helper()
	req, err := schema.DecodeChatRequest([]byte(body))
	if err != nil {
		t.Fatalf("decoding chat request: %v", err)
	}
	return req
}

// TestOpenAIToClaude_SystemPromptPlacement pins where a system prompt lands.
//
// Anthropic's messages array cannot carry a system role, so a request that leaves
// one there is rejected outright rather than degraded. This asserts the hoist, and
// the instruction a json_schema response_format adds, because a client that asked
// for structured output must still get one through the Claude route.
func TestOpenAIToClaude_SystemPromptPlacement(t *testing.T) {
	cases := []struct {
		name           string
		body           string
		wantSystemText string
		wantNoSystem   bool
		wantMessages   int
	}{
		{
			name:           "a lone system message is hoisted with only the user turn left",
			body:           `{"model":"m","messages":[{"role":"system","content":"be terse"},{"role":"user","content":"hi"}]}`,
			wantSystemText: "be terse",
			wantMessages:   1,
		},
		{
			name:           "several system messages are joined into one system block",
			body:           `{"model":"m","messages":[{"role":"system","content":"one"},{"role":"user","content":"q"},{"role":"system","content":"two"}]}`,
			wantSystemText: "one\ntwo",
			wantMessages:   1,
		},
		{
			name:           "a system message with array content keeps only its text blocks",
			body:           `{"model":"m","messages":[{"role":"system","content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]},{"role":"user","content":"q"}]}`,
			wantSystemText: "a\nb",
			wantMessages:   1,
		},
		{
			name:           "a request with no system message emits no system field",
			body:           `{"model":"m","messages":[{"role":"user","content":"q"}]}`,
			wantSystemText: "",
			wantNoSystem:   true,
			wantMessages:   1,
		},
		{
			name:           "a json_schema response format becomes a system instruction",
			body:           `{"model":"m","messages":[{"role":"user","content":"q"}],"response_format":{"type":"json_schema","json_schema":{"name":"r","schema":{"type":"object","properties":{"n":{"type":"number"}}}}}}`,
			wantSystemText: "You must respond with valid JSON",
			wantMessages:   1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := OpenAIToClaude(chatRequest(t, tc.body), "upstream-model", false)
			if tc.wantNoSystem {
				if len(got.System) != 0 {
					t.Fatalf("System = %+v, want none", got.System)
				}
			} else {
				if len(got.System) == 0 {
					t.Fatalf("System = none, want a block carrying %q", tc.wantSystemText)
				}
				if text := got.System[0].Text; !strings.Contains(text, tc.wantSystemText) {
					t.Fatalf("system text = %q, want it to contain %q", text, tc.wantSystemText)
				}
			}
			if len(got.Messages) != tc.wantMessages {
				t.Fatalf("len(Messages) = %d, want %d (%+v)", len(got.Messages), tc.wantMessages, got.Messages)
			}
			if got.Model != "upstream-model" {
				t.Fatalf("Model = %q, want the resolved upstream id", got.Model)
			}
			if got.MaxTokens != DefaultClaudeMaxTokens {
				t.Fatalf("MaxTokens = %d, want the default %d when the client asked for none", got.MaxTokens, DefaultClaudeMaxTokens)
			}
		})
	}
}

// TestOpenAIToClaude_MaxTokensCeiling pins the ceiling rule: Claude requires
// max_tokens, and a client asking above the documented ceiling is clamped rather
// than refused.
func TestOpenAIToClaude_MaxTokensCeiling(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"absent becomes the default", `{"model":"m","messages":[{"role":"user","content":"q"}]}`, DefaultClaudeMaxTokens},
		{"max_tokens is honoured below the ceiling", `{"model":"m","messages":[{"role":"user","content":"q"}],"max_tokens":512}`, 512},
		{"max_completion_tokens is honoured too", `{"model":"m","messages":[{"role":"user","content":"q"}],"max_completion_tokens":64}`, 64},
		{"a value above the ceiling is clamped", `{"model":"m","messages":[{"role":"user","content":"q"}],"max_tokens":999999}`, DefaultClaudeMaxTokens},
		{"exactly the ceiling is kept", `{"model":"m","messages":[{"role":"user","content":"q"}],"max_tokens":64000}`, 64000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := OpenAIToClaude(chatRequest(t, tc.body), "m", false).MaxTokens; got != tc.want {
				t.Fatalf("MaxTokens = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestOpenAIToClaude_ToolOrdering pins Claude's two ordering rules.
//
// A tool_result must stand in its own user turn immediately after the tool_use it
// answers, and a turn carrying tool_use ends there. Both are rejections upstream,
// not preferences, so a translation that gets them wrong fails every tool-using
// client.
func TestOpenAIToClaude_ToolOrdering(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantRoles []string
		wantBlock []string
	}{
		{
			name: "a tool call and its result become two turns",
			body: `{"model":"m","messages":[
				{"role":"user","content":"read it"},
				{"role":"assistant","content":null,"tool_calls":[{"id":"t1","type":"function","function":{"name":"Read","arguments":"{\"path\":\"a\"}"}}]},
				{"role":"tool","tool_call_id":"t1","content":"contents"}]}`,
			wantRoles: []string{RoleUser, RoleAssistant, RoleUser},
			// content:null carries no text, so the assistant turn holds only the
			// tool_use; the four-block reading would invent an empty text block.
			wantBlock: []string{schema.BlockText, schema.BlockToolUse, schema.BlockToolResult},
		},
		{
			name:      "text after a tool_use closes the turn instead of trailing it",
			body:      `{"model":"m","messages":[{"role":"assistant","content":[{"type":"text","text":"here"},{"type":"text","text":"and here"}],"tool_calls":[{"id":"t1","type":"function","function":{"name":"Read","arguments":"{}"}}]}]}`,
			wantRoles: []string{RoleAssistant},
			wantBlock: []string{schema.BlockText, schema.BlockToolUse},
		},
		{
			name:      "consecutive user messages merge into one turn",
			body:      `{"model":"m","messages":[{"role":"user","content":"one"},{"role":"user","content":"two"}]}`,
			wantRoles: []string{RoleUser},
			wantBlock: []string{schema.BlockText, schema.BlockText},
		},
		{
			name:      "an empty assistant turn produces no message at all",
			body:      `{"model":"m","messages":[{"role":"user","content":"q"},{"role":"assistant","content":"   "},{"role":"user","content":"q2"}]}`,
			wantRoles: []string{RoleUser},
			wantBlock: []string{schema.BlockText, schema.BlockText},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := OpenAIToClaude(chatRequest(t, tc.body), "m", false)
			roles := make([]string, 0, len(got.Messages))
			blocks := make([]string, 0, len(tc.wantBlock))
			for _, message := range got.Messages {
				roles = append(roles, message.Role)
				for _, block := range message.Content {
					blocks = append(blocks, block.Type)
				}
			}
			if strings.Join(roles, ",") != strings.Join(tc.wantRoles, ",") {
				t.Fatalf("roles = %v, want %v", roles, tc.wantRoles)
			}
			if strings.Join(blocks, ",") != strings.Join(tc.wantBlock, ",") {
				t.Fatalf("blocks = %v, want %v", blocks, tc.wantBlock)
			}
		})
	}
}

// TestOpenAIToClaude_ImageBlocks pins how an image reference is placed: a data URI
// becomes inline base64 and an http(s) URL stays a URL the upstream fetches.
func TestOpenAIToClaude_ImageBlocks(t *testing.T) {
	const dataURI = "data:image/png;base64,iVBORw0KGgo="
	cases := []struct {
		name      string
		url       string
		wantType  string
		wantMedia string
		wantURL   string
		wantData  string
	}{
		{name: "a png data URI becomes inline base64", url: dataURI, wantType: "base64", wantMedia: "image/png", wantData: "iVBORw0KGgo="},
		{name: "a jpeg data URI keeps its media type", url: "data:image/jpeg;base64,AAAA", wantType: "base64", wantMedia: "image/jpeg", wantData: "AAAA"},
		{name: "a remote https URL stays a URL", url: "https://example.test/a.png", wantType: "url", wantURL: "https://example.test/a.png"},
		{name: "a remote http URL stays a URL", url: "http://example.test/a.png", wantType: "url", wantURL: "http://example.test/a.png"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"model":"m","messages":[{"role":"user","content":[{"type":"text","text":"look"},{"type":"image_url","image_url":{"url":"` + tc.url + `"}}]}]}`
			got := OpenAIToClaude(chatRequest(t, body), "m", false)
			var image *schema.Block
			for i := range got.Messages[0].Content {
				if got.Messages[0].Content[i].Type == schema.BlockImage {
					image = &got.Messages[0].Content[i]
				}
			}
			if image == nil || image.Source == nil {
				t.Fatalf("no image block produced for %q: %+v", tc.url, got.Messages)
			}
			if image.Source.Type != tc.wantType {
				t.Fatalf("source type = %q, want %q", image.Source.Type, tc.wantType)
			}
			if image.Source.MediaType != tc.wantMedia {
				t.Fatalf("media_type = %q, want %q", image.Source.MediaType, tc.wantMedia)
			}
			if image.Source.URL != tc.wantURL {
				t.Fatalf("url = %q, want %q", image.Source.URL, tc.wantURL)
			}
			if image.Source.Data != tc.wantData {
				t.Fatalf("data = %q, want %q", image.Source.Data, tc.wantData)
			}
		})
	}
}

// TestParseToolChoice pins the tool_choice vocabulary, including the shapes that
// must be refused: Anthropic rejects an unknown type, so passing one through is a
// 400 naming nothing.
func TestParseToolChoice(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		wantMode string
		wantName string
		wantErr  bool
	}{
		{name: "absent tool_choice yields no mode", raw: `null`},
		{name: "auto passes through", raw: `"auto"`, wantMode: "auto"},
		{name: "none passes through", raw: `"none"`, wantMode: "none"},
		{name: "required passes through", raw: `"required"`, wantMode: "required"},
		{name: "an unknown string form is refused", raw: `"sometimes"`, wantErr: true},
		{name: "a forced function becomes the tool mode", raw: `{"type":"function","function":{"name":"Read"}}`, wantMode: "tool", wantName: "Read"},
		{name: "a forced function with no name is refused", raw: `{"type":"function","function":{}}`, wantErr: true},
		{name: "an Anthropic any becomes required", raw: `{"type":"any"}`, wantMode: "required"},
		{name: "an Anthropic tool keeps its name", raw: `{"type":"tool","name":"Read"}`, wantMode: "tool", wantName: "Read"},
		{name: "an unknown object type is refused", raw: `{"type":"program"}`, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := schema.ParseToolChoice(json.RawMessage(tc.raw))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("err = nil, want a validation error for %s", tc.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v, want none", err)
			}
			if got.Mode != tc.wantMode {
				t.Fatalf("Mode = %q, want %q", got.Mode, tc.wantMode)
			}
			if got.Name != tc.wantName {
				t.Fatalf("Name = %q, want %q", got.Name, tc.wantName)
			}
		})
	}
}

// TestClaudeToolChoice pins the mapping onto Anthropic's closed set.
func TestClaudeToolChoice(t *testing.T) {
	cases := []struct {
		mode     string
		wantType string
	}{
		{mode: "", wantType: "auto"},
		{mode: "auto", wantType: "auto"},
		{mode: "none", wantType: "none"},
		{mode: "required", wantType: "any"},
		{mode: "tool", wantType: "tool"},
	}
	for _, tc := range cases {
		t.Run("mode="+tc.mode, func(t *testing.T) {
			got := claudeToolChoice(schema.ToolChoice{Mode: tc.mode, Name: "Read"})
			if got.Type != tc.wantType {
				t.Fatalf("Type = %q, want %q", got.Type, tc.wantType)
			}
		})
	}
}

// TestClaudeToOpenAI_SystemAndTools pins the reverse direction: Anthropic's
// top-level system field becomes a leading system message, and its tool
// declarations become OpenAI function tools.
func TestClaudeToOpenAI_SystemAndTools(t *testing.T) {
	cases := []struct {
		name         string
		body         string
		wantSystem   string
		wantRoles    []string
		wantTools    int
		wantToolName string
	}{
		{
			name:       "a string system field becomes a leading system message",
			body:       `{"model":"m","max_tokens":100,"system":"be terse","messages":[{"role":"user","content":"hi"}]}`,
			wantSystem: "be terse",
			wantRoles:  []string{RoleSystem, RoleUser},
		},
		{
			name:       "an array system field is joined",
			body:       `{"model":"m","max_tokens":100,"system":[{"type":"text","text":"a"},{"type":"text","text":"b"}],"messages":[{"role":"user","content":"hi"}]}`,
			wantSystem: "a\nb",
			wantRoles:  []string{RoleSystem, RoleUser},
		},
		{
			name:      "no system field produces no system message",
			body:      `{"model":"m","max_tokens":100,"messages":[{"role":"user","content":"hi"}]}`,
			wantRoles: []string{RoleUser},
		},
		{
			name:         "tool declarations become function tools",
			body:         `{"model":"m","max_tokens":100,"messages":[{"role":"user","content":"hi"}],"tools":[{"name":"Read","description":"read","input_schema":{"type":"object"}}]}`,
			wantRoles:    []string{RoleUser},
			wantTools:    1,
			wantToolName: "Read",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := schema.DecodeMessagesRequest([]byte(tc.body))
			if err != nil {
				t.Fatalf("decoding messages request: %v", err)
			}
			got := ClaudeToOpenAI(req, "upstream", false)
			roles := make([]string, 0, len(got.Messages))
			for _, message := range got.Messages {
				roles = append(roles, message.Role)
			}
			if strings.Join(roles, ",") != strings.Join(tc.wantRoles, ",") {
				t.Fatalf("roles = %v, want %v", roles, tc.wantRoles)
			}
			if tc.wantSystem != "" {
				if got.Messages[0].Content.Text != tc.wantSystem {
					t.Fatalf("system content = %q, want %q", got.Messages[0].Content.Text, tc.wantSystem)
				}
			}
			if len(got.Tools) != tc.wantTools {
				t.Fatalf("len(Tools) = %d, want %d", len(got.Tools), tc.wantTools)
			}
			if tc.wantTools > 0 && got.Tools[0].Function.Name != tc.wantToolName {
				t.Fatalf("tool name = %q, want %q", got.Tools[0].Function.Name, tc.wantToolName)
			}
			if got.Stream {
				t.Fatal("Stream = true, want the caller's value")
			}
		})
	}
}

// TestClaudeToOpenAI_ToolResultsSplit pins the split OpenAI requires: a
// tool_result block becomes its own `tool` message paired by id.
func TestClaudeToOpenAI_ToolResultsSplit(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantRoles []string
		wantID    string
	}{
		{
			name:      "one tool result becomes one tool message",
			body:      `{"model":"m","max_tokens":10,"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"done"}]}]}`,
			wantRoles: []string{RoleTool},
			wantID:    "t1",
		},
		{
			name:      "a result block array is joined into text",
			body:      `{"model":"m","max_tokens":10,"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"t2","content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]}]}]}`,
			wantRoles: []string{RoleTool},
			wantID:    "t2",
		},
		{
			name:      "a tool result beside text yields the result first",
			body:      `{"model":"m","max_tokens":10,"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"t3","content":"x"},{"type":"text","text":"and now"}]}]}`,
			wantRoles: []string{RoleTool, RoleUser},
			wantID:    "t3",
		},
		{
			name:      "a tool_use becomes a tool_calls entry",
			body:      `{"model":"m","max_tokens":10,"messages":[{"role":"assistant","content":[{"type":"tool_use","id":"t4","name":"Read","input":{"path":"a"}}]}]}`,
			wantRoles: []string{RoleAssistant},
			wantID:    "t4",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := schema.DecodeMessagesRequest([]byte(tc.body))
			if err != nil {
				t.Fatalf("decoding messages request: %v", err)
			}
			got := ClaudeToOpenAI(req, "upstream", false)
			roles := make([]string, 0, len(got.Messages))
			for _, message := range got.Messages {
				roles = append(roles, message.Role)
			}
			if strings.Join(roles, ",") != strings.Join(tc.wantRoles, ",") {
				t.Fatalf("roles = %v, want %v", roles, tc.wantRoles)
			}
			// The result block is emitted first, so its id lives on the tool
			// message rather than on whichever message happens to be last.
			matched := false
			for _, message := range got.Messages {
				if message.ToolCallID == tc.wantID {
					matched = true
				}
				for _, call := range message.ToolCalls {
					if call.ID == tc.wantID {
						matched = true
					}
				}
			}
			if !matched {
				t.Fatalf("no block carried id %q: %+v", tc.wantID, got.Messages)
			}
		})
	}
}

// TestUsageFoldRoundTrip pins the token arithmetic both directions use, which is
// where an off-by-a-cache-count error would silently understate every prompt.
func TestUsageFoldRoundTrip(t *testing.T) {
	cases := []struct {
		name       string
		claude     schema.MessagesUsage
		wantPrompt int
		wantCompl  int
		wantTotal  int
	}{
		{
			name:       "a plain response folds with no cache traffic",
			claude:     schema.MessagesUsage{InputTokens: 100, OutputTokens: 20},
			wantPrompt: 100, wantCompl: 20, wantTotal: 120,
		},
		{
			name:       "cache read is part of the prompt",
			claude:     schema.MessagesUsage{InputTokens: 100, OutputTokens: 20, CacheReadInputTokens: 400},
			wantPrompt: 500, wantCompl: 20, wantTotal: 520,
		},
		{
			name:       "cache creation is part of the prompt too",
			claude:     schema.MessagesUsage{InputTokens: 10, OutputTokens: 5, CacheCreationInputTokens: 90},
			wantPrompt: 100, wantCompl: 5, wantTotal: 105,
		},
		{
			name:       "both cache sides fold in",
			claude:     schema.MessagesUsage{InputTokens: 1, OutputTokens: 2, CacheReadInputTokens: 3, CacheCreationInputTokens: 4},
			wantPrompt: 8, wantCompl: 2, wantTotal: 10,
		},
		{
			name:       "a zero usage block stays zero rather than negative",
			claude:     schema.MessagesUsage{},
			wantPrompt: 0, wantCompl: 0, wantTotal: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClaudeUsageToOpenAI(tc.claude)
			if got.PromptTokens != tc.wantPrompt || got.CompletionTokens != tc.wantCompl || got.TotalTokens != tc.wantTotal {
				t.Fatalf("usage = %+v, want prompt=%d completion=%d total=%d", got, tc.wantPrompt, tc.wantCompl, tc.wantTotal)
			}
			back := OpenAIToClaudeUsage(got)
			if back.InputTokens != tc.claude.InputTokens {
				t.Fatalf("input_tokens round trip = %d, want %d", back.InputTokens, tc.claude.InputTokens)
			}
			if back.OutputTokens != tc.claude.OutputTokens {
				t.Fatalf("output_tokens round trip = %d, want %d", back.OutputTokens, tc.claude.OutputTokens)
			}
		})
	}
}
