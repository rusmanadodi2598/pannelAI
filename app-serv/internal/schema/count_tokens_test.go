// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/count_tokens_test.go
// @for       Table-driven tests for the count_tokens contract: what the
//
//	estimate measures and what the route refuses.
//
// @uses      strings, testing.
// @reason    TDD.md §2.5 requires the table to cover the whole input surface,
//
//	not one happy sample: both content shapes, every block kind that must
//	not count, the empty and absent cases, and the fields the route
//	tolerates because a CLI tool sends them.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-19
package schema

import (
	"strings"
	"testing"
)

// TestCountTokensRequest_InputChars pins what the estimate is measured over.
// Every case decodes first, because a bare-string body and an array body are
// two wire shapes that must measure the same.
func TestCountTokensRequest_InputChars(t *testing.T) {
	longText := strings.Repeat("a", 4096)
	cases := []struct {
		name string
		body string
		want int
	}{
		{
			name: "a bare string body",
			body: `{"model":"claude-sonnet-4","messages":[{"role":"user","content":"hello"}]}`,
			want: 5,
		},
		{
			name: "text parts are summed across parts and messages",
			body: `{"model":"claude-sonnet-4","messages":[` +
				`{"role":"user","content":[{"type":"text","text":"ab"},{"type":"text","text":"cde"}]},` +
				`{"role":"assistant","content":[{"type":"text","text":"fg"}]}]}`,
			want: 7,
		},
		{
			name: "non-text blocks contribute nothing",
			body: `{"model":"claude-sonnet-4","messages":[{"role":"user","content":[` +
				`{"type":"text","text":"hi"},` +
				`{"type":"image","source":{"type":"base64","media_type":"image/png","data":"aGk="}},` +
				`{"type":"tool_use","id":"tu_1","name":"lookup","input":{"q":"x"}},` +
				`{"type":"tool_result","tool_use_id":"tu_1","content":"a result"},` +
				`{"type":"thinking","thinking":"a thought"}]}]}`,
			want: 2,
		},
		{
			name: "an empty message list",
			body: `{"model":"claude-sonnet-4","messages":[]}`,
			want: 0,
		},
		{
			name: "an absent message list",
			body: `{"model":"claude-sonnet-4"}`,
			want: 0,
		},
		{
			name: "empty and null content",
			body: `{"model":"claude-sonnet-4","messages":[{"role":"user","content":""},{"role":"user","content":null}]}`,
			want: 0,
		},
		{
			name: "a large body scales with its text",
			body: `{"model":"claude-sonnet-4","messages":[{"role":"user","content":[{"type":"text","text":"` + longText + `"}]}]}`,
			want: len(longText),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := DecodeCountTokensRequest([]byte(tc.body))
			if err != nil {
				t.Fatalf("DecodeCountTokensRequest() error = %v", err)
			}
			if got := req.InputChars(); got != tc.want {
				t.Fatalf("InputChars() = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestCountTokensRequest_Validation drives the body through the same decode and
// validate calls the handler makes. The tolerated cases are the shapes a CLI
// tool actually sends: a system field, tools, and the Anthropic members the
// gateway does not model.
func TestCountTokensRequest_Validation(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name:    "a minimal body",
			body:    `{"model":"claude-sonnet-4","messages":[{"role":"user","content":"hello"}]}`,
			wantErr: false,
		},
		{
			name: "a full body with system, tools, and thinking",
			body: `{"model":"claude-sonnet-4",` +
				`"system":[{"type":"text","text":"be brief"}],` +
				`"messages":[{"role":"user","content":"hello"}],` +
				`"tools":[{"name":"lookup","description":"looks up","input_schema":{"type":"object"}}],` +
				`"tool_choice":{"type":"auto"},` +
				`"thinking":{"type":"enabled","budget_tokens":2048}}`,
			wantErr: false,
		},
		{
			name:    "a system field as a bare string",
			body:    `{"model":"claude-sonnet-4","system":"be brief","messages":[{"role":"user","content":"hello"}]}`,
			wantErr: false,
		},
		{
			name:    "the Anthropic members the gateway does not model",
			body:    `{"model":"claude-sonnet-4","messages":[{"role":"user","content":"hi"}],"betas":["prompt-caching"],"metadata":{"user_id":"u-1"}}`,
			wantErr: false,
		},
		{
			name:    "a missing model",
			body:    `{"messages":[{"role":"user","content":"hello"}]}`,
			wantErr: true,
		},
		{
			name:    "an empty model",
			body:    `{"model":"","messages":[{"role":"user","content":"hello"}]}`,
			wantErr: true,
		},
		{
			name:    "a role outside the Anthropic pair",
			body:    `{"model":"claude-sonnet-4","messages":[{"role":"system","content":"hello"}]}`,
			wantErr: true,
		},
		{
			name:    "a message with an empty content array",
			body:    `{"model":"claude-sonnet-4","messages":[{"role":"user","content":[]}]}`,
			wantErr: false,
		},
		{
			name:    "content that is neither a string nor blocks",
			body:    `{"model":"claude-sonnet-4","messages":[{"role":"user","content":42}]}`,
			wantErr: true,
		},
		{
			name:    "a tool without a name",
			body:    `{"model":"claude-sonnet-4","messages":[{"role":"user","content":"hi"}],"tools":[{"description":"nameless"}]}`,
			wantErr: true,
		},
		{
			name:    "malformed JSON",
			body:    `{"model":`,
			wantErr: true,
		},
		{
			name:    "an empty object",
			body:    `{}`,
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := DecodeCountTokensRequest([]byte(tc.body))
			if err == nil {
				err = ValidateStruct(req)
			}
			if tc.wantErr != (err != nil) {
				t.Fatalf("decode+validate error = %v, wantErr = %t", err, tc.wantErr)
			}
		})
	}
}
