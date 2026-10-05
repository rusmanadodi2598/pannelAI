// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_openai_claude_test.go
// @for       Table-driven tests for OpenAI ↔ Anthropic request translation.
// @uses      testing, internal/schema.
// @reason    SPEC-API-001 §7.15 makes translation the data plane's core job, so
//
//	every rule it implements, system hoisting, tool ordering, image
//	mapping, and usage folding, is pinned here. Translation is a pure
//	function, so these run with no network and no clock (AGENTS.md §2.1).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-17
package dataplane

import (
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
