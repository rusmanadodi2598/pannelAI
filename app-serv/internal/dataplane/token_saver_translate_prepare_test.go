// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/token_saver_translate_prepare_test.go
// @for       Table-driven tests for safe Headroom preparation across provider wires.
// @uses      encoding/json, testing.
// @reason    SPEC-API-002 §8.2 permits only message-shaped bodies into Headroom;
// these cases pin supported wires, malformed input, refusal, and unknown targets.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-20
package dataplane

import (
	"encoding/json"
	"testing"
)

// TestTokenSaverTranslator_Prepare covers every supported wire, safe refusal,
// malformed input, and unknown wire handling.
func TestTokenSaverTranslator_Prepare(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wire      string
		wantOK    bool
		wantError bool
		wantModel string
	}{
		{
			name: "OpenAI messages are already the pivot",
			body: `{"model":"client","messages":[{"role":"user","content":"hello"}],"keep":{"id":7}}`,
			wire: TargetOpenAI, wantOK: true, wantModel: "upstream",
		},
		{
			name: "Claude system and messages become the pivot",
			body: `{"model":"client","max_tokens":100,"system":"be brief","messages":[{"role":"user","content":"hello"}],"keep":{"id":7}}`,
			wire: TargetClaude, wantOK: true, wantModel: "upstream",
		},
		{
			name: "Responses message input becomes the pivot",
			body: `{"model":"client","instructions":"be brief","input":[{"type":"message","role":"user","content":"hello"}],"keep":{"id":7}}`,
			wire: TargetResponses, wantOK: true, wantModel: "upstream",
		},
		{
			name: "Responses reasoning is refused",
			body: `{"model":"client","input":[{"type":"reasoning","summary":[]} ]}`,
			wire: TargetResponses,
		},
		{
			name: "Responses function output is refused",
			body: `{"model":"client","input":[{"type":"function_call_output","call_id":"c1","output":"done"}]}`,
			wire: TargetResponses,
		},
		{
			name: "malformed Claude body reports a decode error",
			body: `{"model":`, wire: TargetClaude, wantError: true,
		},
		{
			name: "unknown wire is never guessed",
			body: `{"messages":[{"role":"user","content":"hello"}]}`,
			wire: TargetOpenAI + "-unknown",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request, ok, err := (TokenSaverTranslator{}).Prepare([]byte(tc.body), tc.wire, "upstream")
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v; request=%+v", ok, tc.wantOK, request)
			}
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v, want error=%v", err, tc.wantError)
			}
			if !tc.wantOK {
				return
			}
			if request.Model != tc.wantModel {
				t.Fatalf("model = %q, want %q", request.Model, tc.wantModel)
			}
			var pivot []json.RawMessage
			if err := json.Unmarshal(request.Messages, &pivot); err != nil {
				t.Fatalf("messages are not an array: %v; raw=%s", err, request.Messages)
			}
			if len(pivot) == 0 {
				t.Fatalf("messages = %s, want at least one message", request.Messages)
			}
			first := translateObject(t, pivot[0])
			if _, exists := first["role"]; !exists {
				t.Fatalf("first pivot message = %s, want a role member", pivot[0])
			}
		})
	}
}
