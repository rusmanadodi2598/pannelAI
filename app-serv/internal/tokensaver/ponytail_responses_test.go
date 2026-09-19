// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/ponytail_responses_test.go
// @for       The Responses cases: the wire whose system slot is an instructions
//
//	string or a system message item.
//
// @uses      encoding/json, fmt, strings, testing.
// @reason    SPEC-API-002 §7 gives each wire its own slot and its own part type,
//
//	and the Anthropic case carries a rule the others do not: the
//	instruction lands before the last cache-control block, so a cached
//	prefix stays byte-identical. That is asserted by position, not by
//	presence.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import (
	"fmt"
	"testing"
)

// TestInjectPonytail_Responses drives the Responses wire: an instructions
// string when the body carries one, a system message item otherwise.
func TestInjectPonytail_Responses(t *testing.T) {
	prompt := ponytailPrompts[PonytailFull]
	cases := []struct {
		name  string
		body  string
		check func(t *testing.T, in string, out []byte)
	}{
		{
			name: "an instructions string",
			body: `{"model":"m","instructions":"be terse","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]}`,
			check: func(t *testing.T, in string, out []byte) {
				envelope := envelopeOf(t, out)
				instructions, ok := envelope.memberString("instructions")
				if !ok {
					t.Fatalf("instructions is not a string: %s", out)
				}
				if want := "be terse" + ponytailSeparator + prompt; instructions != want {
					t.Fatalf("instructions = %q, want %q", instructions, want)
				}
				items := arrayMember(t, envelope, "input")
				if len(items) != 1 {
					t.Fatalf("input = %d items, want the one that was there", len(items))
				}
				if got := stringMember(t, items[0], "role"); got != "user" {
					t.Fatalf("the input item changed: %q", got)
				}
			},
		},
		{
			name: "instructions already carrying the instruction",
			body: fmt.Sprintf(`{"instructions":%q,"input":[]}`, "be terse"+ponytailSeparator+prompt),
			check: func(t *testing.T, in string, out []byte) {
				if string(out) != in {
					t.Fatalf("an injected body must survive byte for byte:\n in: %s\nout: %s", in, out)
				}
			},
		},
		{
			name: "a system message item with parts",
			body: `{"input":[{"type":"message","role":"system","content":[{"type":"input_text","text":"be terse"}]}]}`,
			check: func(t *testing.T, in string, out []byte) {
				items := arrayMember(t, envelopeOf(t, out), "input")
				parts := arrayMember(t, envelopeOf(t, items[0]), "content")
				if len(parts) != 2 {
					t.Fatalf("parts = %d, want the original plus the instruction", len(parts))
				}
				if got := stringMember(t, parts[1], "type"); got != "input_text" {
					t.Fatalf("part type = %q, want input_text", got)
				}
				if got := stringMember(t, parts[1], "text"); got != prompt {
					t.Fatalf("part text = %q, want the instruction", got)
				}
			},
		},
		{
			name: "a system message item with string content",
			body: `{"input":[{"type":"message","role":"system","content":"be terse"}]}`,
			check: func(t *testing.T, in string, out []byte) {
				items := arrayMember(t, envelopeOf(t, out), "input")
				if got := stringMember(t, items[0], "content"); got != "be terse"+ponytailSeparator+prompt {
					t.Fatalf("content = %q", got)
				}
			},
		},
		{
			name: "a body without a system item",
			body: `{"model":"m","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]}`,
			check: func(t *testing.T, in string, out []byte) {
				items := arrayMember(t, envelopeOf(t, out), "input")
				if len(items) != 2 {
					t.Fatalf("input = %d items, want a created one in front", len(items))
				}
				created := envelopeOf(t, items[0])
				if kind, _ := created.memberString("type"); kind != "message" {
					t.Fatalf("created item type = %q, want message", kind)
				}
				if role, _ := created.memberString("role"); role != "system" {
					t.Fatalf("created item role = %q, want system", role)
				}
				parts := arrayMember(t, created, "content")
				if len(parts) != 1 {
					t.Fatalf("created content = %d parts, want one", len(parts))
				}
				if got := stringMember(t, parts[0], "type"); got != "input_text" {
					t.Fatalf("created part type = %q, want input_text", got)
				}
				if got := stringMember(t, parts[0], "text"); got != prompt {
					t.Fatalf("created part text = %q, want the instruction", got)
				}
				if got := stringMember(t, items[1], "role"); got != "user" {
					t.Fatalf("the user item changed: %q", got)
				}
			},
		},
		{
			name: "a function_call item is not a system slot",
			body: `{"input":[{"type":"function_call","name":"f"},{"type":"message","role":"system","content":[{"type":"input_text","text":"be terse"}]}]}`,
			check: func(t *testing.T, in string, out []byte) {
				items := arrayMember(t, envelopeOf(t, out), "input")
				if got := stringMember(t, items[0], "type"); got != "function_call" {
					t.Fatalf("the first item changed: %q", got)
				}
				parts := arrayMember(t, envelopeOf(t, items[1]), "content")
				if len(parts) != 2 {
					t.Fatalf("the system item's parts = %d, want the instruction appended", len(parts))
				}
			},
		},
		{
			name: "a string input stays a string",
			body: `{"model":"m","input":"hello"}`,
			check: func(t *testing.T, in string, out []byte) {
				if string(out) != in {
					t.Fatalf("a string input must be left alone:\n in: %s\nout: %s", in, out)
				}
			},
		},
		{
			name: "a null input stays null",
			body: `{"model":"m","input":null}`,
			check: func(t *testing.T, in string, out []byte) {
				if string(out) != in {
					t.Fatalf("a null input must be left alone:\n in: %s\nout: %s", in, out)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, tc.body, InjectPonytail([]byte(tc.body), WireResponses, PonytailFull))
		})
	}
}
