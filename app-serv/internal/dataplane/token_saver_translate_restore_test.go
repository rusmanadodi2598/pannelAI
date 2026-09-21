// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/token_saver_translate_restore_test.go
// @for       Table-driven tests for member-preserving Headroom restoration.
// @uses      bytes, encoding/json, testing.
// @reason    SPEC-API-002 §3.4 keeps every unmodelled field byte-identical, and
// §8.2 puts compressed messages back into the original wire. Both claims are
// structural, so each case decodes the result and asserts the members it keeps.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-20
package dataplane

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestTokenSaverTranslator_Restore covers member-preserving restoration for
// OpenAI, Claude, and Responses plus malformed output and unknown wires.
func TestTokenSaverTranslator_Restore(t *testing.T) {
	compressed := []byte(`[{"role":"system","content":"compressed instructions"},{"role":"user","content":"compressed message"}]`)
	cases := []struct {
		name       string
		body       string
		wire       string
		messages   []byte
		wantMember string
		wantKeep   string
		wantError  bool
	}{
		{
			name: "OpenAI replaces messages and preserves sibling",
			body: `{"model":"m","messages":[{"role":"user","content":"old"}],"keep":"openai"}`,
			wire: TargetOpenAI, messages: compressed, wantMember: "messages", wantKeep: "openai",
		},
		{
			name: "Claude restores system and messages",
			body: `{"model":"m","max_tokens":100,"system":"old instructions","messages":[{"role":"user","content":"old"}],"keep":"claude"}`,
			wire: TargetClaude, messages: compressed, wantMember: "messages", wantKeep: "claude",
		},
		{
			name: "Responses restores input and instructions",
			body: `{"model":"m","instructions":"old instructions","input":[{"type":"message","role":"user","content":"old"}],"keep":"responses"}`,
			wire: TargetResponses, messages: compressed, wantMember: "input", wantKeep: "responses",
		},
		{
			name: "malformed compressed array is rejected",
			body: `{"model":"m","messages":[],"keep":"safe"}`,
			wire: TargetOpenAI, messages: []byte(`{"role":"user"}`), wantError: true,
		},
		{
			name: "unknown wire remains unchanged",
			body: `{"model":"m","messages":[],"keep":"unknown"}`,
			wire: "wire-not-supported", messages: compressed, wantKeep: "unknown",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original := []byte(tc.body)
			got, err := (TokenSaverTranslator{}).Restore(original, tc.wire, "upstream", tc.messages)
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v, want error=%v", err, tc.wantError)
			}
			if tc.wantError {
				if !bytes.Equal(got, original) {
					t.Fatalf("failed restore changed body = %s, want %s", got, original)
				}
				return
			}
			if tc.wantMember == "" {
				if !bytes.Equal(got, original) {
					t.Fatalf("unknown wire changed body = %s, want %s", got, original)
				}
				return
			}
			object := translateObject(t, got)
			if keep := translateString(t, object, "keep"); keep != tc.wantKeep {
				t.Fatalf("keep = %q, want %q", keep, tc.wantKeep)
			}
			assertRestoredWire(t, tc.wire, object, tc.wantMember)
		})
	}
}

// assertRestoredWire checks the member the wire's own restore step owns, and
// the wire-specific shape a compressed message array becomes.
func assertRestoredWire(t *testing.T, wire string, object map[string]json.RawMessage, member string) {
	t.Helper()
	if _, exists := object[member]; !exists {
		t.Fatalf("restored body carries no %s member: %v", member, object)
	}
	switch wire {
	case TargetOpenAI:
		messages := translateArray(t, object, member)
		if len(messages) == 0 {
			t.Fatalf("messages = %s, want the compressed array", object[member])
		}
		if role := translateString(t, translateObject(t, messages[0]), "role"); role != "system" {
			t.Fatalf("first message role = %q, want system", role)
		}
	case TargetClaude:
		if system := translateContentText(t, object["system"]); system == "" {
			t.Fatalf("system = %s, want the compressed instruction", object["system"])
		}
		messages := translateArray(t, object, member)
		if len(messages) == 0 {
			t.Fatalf("messages = %s, want the compressed array", object[member])
		}
	case TargetResponses:
		if instructions := translateString(t, object, "instructions"); instructions == "" {
			t.Fatalf("instructions = %s, want the compressed instruction", object["instructions"])
		}
		items := translateArray(t, object, member)
		if len(items) == 0 {
			t.Fatalf("input = %s, want the compressed items", object[member])
		}
		if role := translateString(t, translateObject(t, items[0]), "role"); role != "user" {
			t.Fatalf("first item role = %q, want user", role)
		}
	default:
		t.Fatalf("unhandled wire %q", wire)
	}
}
