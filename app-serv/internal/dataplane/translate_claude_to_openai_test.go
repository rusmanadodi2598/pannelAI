// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_claude_to_openai_test.go
// @for       Anthropic-to-OpenAI request translation and the usage fold.
// @uses      encoding/json, testing, internal/schema.
// @reason    The reverse direction has its own rules, a top-level system field
//
//	becoming a leading message and each tool_use turn standing alone, and
//	they are pinned separately from the OpenAI-to-Anthropic half.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04
package dataplane

import (
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

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
