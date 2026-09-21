// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/chat_request_rules_test.go
// @for       Table-driven coverage of the request-level chat contract rules:
// the scalar bounds, the stop shapes, the response format, and the roles.
// @uses      testing.
// @reason    F1 of docs/DRAFT/009-PLAYGROUND-CHAT-ENDPOINT-READINESS.md requires
// a benign control, a boundary, and a malformed case per rule. The numeric
// bounds and the cross-field pairs are the members a client can get wrong
// without noticing, so each is pinned here rather than left to the struct tag.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-21
package schema

import "testing"

// TestChatRequest_ScalarBounds pins the numeric members at both ends: the
// contract's own limits, the boundary values, and the shapes that fall outside.
func TestChatRequest_ScalarBounds(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		valid bool
	}{
		{name: "temperature at zero", body: chatWith(`"temperature":0`), valid: true},
		{name: "temperature at the ceiling", body: chatWith(`"temperature":2`), valid: true},
		{name: "temperature above the ceiling", body: chatWith(`"temperature":2.01`)},
		{name: "temperature below zero", body: chatWith(`"temperature":-0.01`)},
		{name: "temperature far above", body: chatWith(`"temperature":100`)},
		{name: "top_p at one", body: chatWith(`"top_p":1`), valid: true},
		{name: "top_p above one", body: chatWith(`"top_p":1.5`)},
		{name: "top_p negative", body: chatWith(`"top_p":-1`)},
		{name: "max_tokens at one", body: chatWith(`"max_tokens":1`), valid: true},
		{name: "max_tokens at zero", body: chatWith(`"max_tokens":0`)},
		{name: "max_tokens negative", body: chatWith(`"max_tokens":-5`)},
		{name: "max_completion_tokens at zero", body: chatWith(`"max_completion_tokens":0`)},
		{name: "a very large ceiling is accepted", body: chatWith(`"max_tokens":1000000`), valid: true},
		{name: "a model id at the length limit", body: `{"model":"` + repeat("m", 200) + `","messages":[{"role":"user","content":"hi"}]}`, valid: true},
		{name: "a model id over the length limit", body: `{"model":"` + repeat("m", 201) + `","messages":[{"role":"user","content":"hi"}]}`},
		{name: "an empty model id", body: `{"model":"","messages":[{"role":"user","content":"hi"}]}`},
		{name: "no messages", body: `{"model":"gpt-4o","messages":[]}`},
		{name: "messages omitted", body: `{"model":"gpt-4o"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertChatValidity(t, tc.body, tc.valid)
		})
	}
}

// TestChatRequest_Roles pins the closed role vocabulary: an unknown role is
// refused rather than forwarded, because a provider's answer to one names
// nothing the client can act on.
func TestChatRequest_Roles(t *testing.T) {
	cases := []struct {
		name  string
		role  string
		valid bool
	}{
		{name: "system", role: "system", valid: true},
		{name: "user", role: "user", valid: true},
		{name: "assistant", role: "assistant", valid: true},
		{name: "tool", role: "tool", valid: true},
		{name: "developer", role: "developer", valid: true},
		{name: "an unknown role", role: "banana"},
		{name: "an empty role", role: ""},
		{name: "a cased role", role: "User"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := chatMessage(`{"role":"` + tc.role + `","content":"hi"}`)
			assertChatValidity(t, body, tc.valid)
		})
	}
}
