// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_response_openai_claude_test.go
// @for       A non-streaming OpenAI completion re-framed as an Anthropic message.
// @uses      encoding/json, testing, internal/schema.
// @reason    This is the path a non-streaming Claude Code client is served on when the upstream answers with a stream the gateway folds, and it carried no test at all: a tool call's `input`, the block order around it, and the argument-name correction all landed on the client unobserved.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-14
package dataplane

import (
	"encoding/json"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

func TestOpenAIToClaudeResponse_ToolInput(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantBlocks []string
		wantID     string
		wantName   string
		wantJSON   string
	}{
		{
			name: "a call that used the real parameter is passed through",
			body: `{"id":"chatcmpl-1","choices":[{"index":0,"message":{"role":"assistant",` +
				`"tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":1}"}}]},` +
				`"finish_reason":"tool_calls"}]}`,
			wantBlocks: []string{schema.BlockToolUse},
			wantID:     "call_1", wantName: "lookup", wantJSON: `{"q":1}`,
		},
		{
			name: "a Bash call that invented cmd is renamed onto command",
			body: `{"id":"chatcmpl-2","choices":[{"index":0,"message":{"role":"assistant",` +
				`"tool_calls":[{"id":"call_2","type":"function","function":{"name":"Bash","arguments":"{\"cmd\":\"ls -l\"}"}}]},` +
				`"finish_reason":"tool_calls"}]}`,
			wantBlocks: []string{schema.BlockToolUse},
			wantID:     "call_2", wantName: "Bash", wantJSON: `{"command":"ls -l"}`,
		},
		{
			name: "a Read call that invented path is renamed onto file_path",
			body: `{"id":"chatcmpl-3","choices":[{"index":0,"message":{"role":"assistant",` +
				`"tool_calls":[{"id":"call_3","type":"function","function":{"name":"Read","arguments":"{\"path\":\"/x\"}"}}]},` +
				`"finish_reason":"tool_calls"}]}`,
			wantBlocks: []string{schema.BlockToolUse},
			wantID:     "call_3", wantName: "Read", wantJSON: `{"file_path":"/x","path":"/x"}`,
		},
		{
			name: "text is kept and ordered before the call",
			body: `{"id":"chatcmpl-4","choices":[{"index":0,"message":{"role":"assistant","content":"thinking",` +
				`"tool_calls":[{"id":"call_4","type":"function","function":{"name":"Bash","arguments":"{}"}}]},` +
				`"finish_reason":"tool_calls"}]}`,
			wantBlocks: []string{schema.BlockText, schema.BlockToolUse},
			wantID:     "call_4", wantName: "Bash", wantJSON: `{}`,
		},
		{
			name:       "an answer with no call carries only its text",
			body:       `{"id":"chatcmpl-5","choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}]}`,
			wantBlocks: []string{schema.BlockText},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			messages, err := OpenAIToClaudeResponse([]byte(tc.body), "claude-x")
			if err != nil {
				t.Fatalf("OpenAIToClaudeResponse() error = %v", err)
			}
			if len(messages.Content) != len(tc.wantBlocks) {
				t.Fatalf("blocks = %s, want one per %v", describeBlocks(messages.Content), tc.wantBlocks)
			}
			for index, want := range tc.wantBlocks {
				if got := messages.Content[index].Type; got != want {
					t.Fatalf("block %d is %q, want %q", index, got, want)
				}
			}
			if tc.wantID == "" {
				return
			}
			call := messages.Content[len(messages.Content)-1]
			if call.ID != tc.wantID || call.Name != tc.wantName {
				t.Fatalf("tool call is %s/%s, want %s/%s", call.ID, call.Name, tc.wantID, tc.wantName)
			}
			if got := string(call.Input); got != tc.wantJSON {
				t.Fatalf("tool input = %s, want %s", got, tc.wantJSON)
			}
			var object map[string]json.RawMessage
			if err := json.Unmarshal(call.Input, &object); err != nil {
				t.Fatalf("tool input %s is not the object Anthropic requires: %v", call.Input, err)
			}
		})
	}
}

func TestOpenAIToClaudeResponse_RefusesAnUnreadableBody(t *testing.T) {
	if _, err := OpenAIToClaudeResponse([]byte(`[1,2,3]`), "claude-x"); err == nil {
		t.Fatal("an array body was accepted, want the upstream error the client reads")
	}
}

// describeBlocks renders a block list for a failure message.
func describeBlocks(blocks []schema.Block) string {
	encoded, err := json.Marshal(blocks)
	if err != nil {
		return "(unreadable)"
	}
	return string(encoded)
}
