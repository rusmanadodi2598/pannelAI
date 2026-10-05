// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/translate_claude_tool_choice_test.go
// @for       The tool_choice vocabulary on both sides of the translation.
// @uses      testing, internal/schema.
// @reason    Anthropic accepts a closed set of tool_choice shapes and rejects the
//
//	rest with a 400 that names nothing, so the parse and the mapping are
//	pinned apart from the request translation they belong to.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04
package dataplane

import (
	"encoding/json"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

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
