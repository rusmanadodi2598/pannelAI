// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_choice_test.go
// @for       The tool_choice rule the OpenCode Responses wire enforces.
// @uses      testing, internal/registry.
// @reason    The Responses wire answers 400 to any tool_choice other than auto, so
//
//	the connector normalises whatever a client sent rather than forwarding
//	it to be rejected. The chat wire accepts `none`, so the rule is
//	per-wire and pinned on its own.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-21
package provider

import (
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestOpenCode_TransformForcesAutoToolChoiceOnResponses pins the rule the
// Responses wire enforces: any tool_choice other than `auto` is a 400, so the
// connector normalises whatever the client sent rather than forwarding it to be
// rejected.
func TestOpenCode_TransformForcesAutoToolChoiceOnResponses(t *testing.T) {
	connector := NewOpenCode(opencodeEntry("https://opencode.ai", "openai"))

	cases := []struct {
		name     string
		choice   string
		wantAuto bool
	}{
		{name: "absent", choice: "", wantAuto: true},
		{name: "already auto", choice: `"auto"`, wantAuto: true},
		{name: "none", choice: `"none"`, wantAuto: true},
		{name: "required", choice: `"required"`, wantAuto: true},
		{name: "a forced function object", choice: `{"type":"function","name":"my_tool"}`, wantAuto: true},
		{name: "a number", choice: `1`, wantAuto: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"model":"muse-spark-1.3-contributor-free","input":[]`
			if tc.choice != "" {
				body += `,"tool_choice":` + tc.choice
			}
			body += `}`
			request := Request{
				Model: registry.Model{ID: "muse-spark-1.3-contributor-free", TargetFormat: "openai-responses"},
				Body:  []byte(body),
			}
			if err := connector.TransformRequest(&request); err != nil {
				t.Fatalf("TransformRequest() error = %v", err)
			}
			got, ok := decodeBody(t, request.Body)["tool_choice"]
			if tc.wantAuto {
				if !ok || got != "auto" {
					t.Fatalf("tool_choice = %v (present=%v), want auto", got, ok)
				}
			}
		})
	}

	// The chat wire accepts `none`, so the connector must not rewrite it there:
	// forcing auto on a client that asked for no tool call would change its
	// request.
	t.Run("the chat wire keeps a client's own choice", func(t *testing.T) {
		body := `{"model":"big-pickle","messages":[{"role":"user","content":"q"}],"tool_choice":"none"}`
		request := Request{Model: registry.Model{ID: "big-pickle"}, Body: []byte(body)}
		if err := connector.TransformRequest(&request); err != nil {
			t.Fatalf("TransformRequest() error = %v", err)
		}
		if got := decodeBody(t, request.Body)["tool_choice"]; got != "none" {
			t.Fatalf("tool_choice = %v, want the client's none kept on the chat wire", got)
		}
	})
}
