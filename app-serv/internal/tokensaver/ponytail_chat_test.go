// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/ponytail_chat_test.go
// @for       The OpenAI chat cases: the wire whose system slot is a message.
// @uses      fmt, strings, testing.
// @reason    SPEC-API-002 §7 injects into the first system or developer message,
//
//	and creates one when the body has neither. Each case reads the slot
//	back, so a change in position or part type fails rather than passes.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import (
	"fmt"
	"strings"
	"testing"
)

// TestInjectPonytail_Chat drives the OpenAI chat wire, whose system slot is a
// message inside the list.
func TestInjectPonytail_Chat(t *testing.T) {
	prompt := ponytailPrompts[PonytailFull]
	cases := []struct {
		name  string
		body  string
		check func(t *testing.T, in string, out []byte)
	}{
		{
			name: "a system message as a string",
			body: `{"model":"m","messages":[{"role":"system","content":"be terse"},{"role":"user","content":"hi"}]}`,
			check: func(t *testing.T, in string, out []byte) {
				messages := arrayMember(t, envelopeOf(t, out), "messages")
				if len(messages) != 2 {
					t.Fatalf("messages = %d, want the two that were there", len(messages))
				}
				if got, want := stringMember(t, messages[0], "content"), "be terse"+ponytailSeparator+prompt; got != want {
					t.Fatalf("content = %q, want %q", got, want)
				}
				if got := stringMember(t, messages[1], "content"); got != "hi" {
					t.Fatalf("the user message changed: %q", got)
				}
			},
		},
		{
			name: "a developer message is the system slot",
			body: `{"messages":[{"role":"developer","content":"be terse"}]}`,
			check: func(t *testing.T, in string, out []byte) {
				messages := arrayMember(t, envelopeOf(t, out), "messages")
				if got, want := stringMember(t, messages[0], "content"), "be terse"+ponytailSeparator+prompt; got != want {
					t.Fatalf("content = %q, want %q", got, want)
				}
			},
		},
		{
			name: "a system message with content parts",
			body: `{"messages":[{"role":"system","content":[{"type":"text","text":"be terse"}]}]}`,
			check: func(t *testing.T, in string, out []byte) {
				messages := arrayMember(t, envelopeOf(t, out), "messages")
				parts := arrayMember(t, envelopeOf(t, messages[0]), "content")
				if len(parts) != 2 {
					t.Fatalf("parts = %d, want the original plus the instruction", len(parts))
				}
				if got := stringMember(t, parts[1], "type"); got != "text" {
					t.Fatalf("part type = %q, want text", got)
				}
				if got := stringMember(t, parts[1], "text"); got != prompt {
					t.Fatalf("part text = %q, want the instruction", got)
				}
			},
		},
		{
			name: "a body without a system message",
			body: `{"model":"m","messages":[{"role":"user","content":"hi"}]}`,
			check: func(t *testing.T, in string, out []byte) {
				messages := arrayMember(t, envelopeOf(t, out), "messages")
				if len(messages) != 2 {
					t.Fatalf("messages = %d, want a created one in front", len(messages))
				}
				if got := stringMember(t, messages[0], "role"); got != "system" {
					t.Fatalf("role = %q, want system", got)
				}
				if got := stringMember(t, messages[0], "content"); got != prompt {
					t.Fatalf("content = %q, want the instruction", got)
				}
				if got := stringMember(t, messages[1], "content"); got != "hi" {
					t.Fatalf("the user message changed: %q", got)
				}
			},
		},
		{
			name: "a system message with null content",
			body: `{"messages":[{"role":"system","content":null}]}`,
			check: func(t *testing.T, in string, out []byte) {
				messages := arrayMember(t, envelopeOf(t, out), "messages")
				if got := stringMember(t, messages[0], "content"); got != prompt {
					t.Fatalf("content = %q, want the instruction as a string", got)
				}
			},
		},
		{
			name: "a system message with no content member",
			body: `{"messages":[{"role":"system"}]}`,
			check: func(t *testing.T, in string, out []byte) {
				messages := arrayMember(t, envelopeOf(t, out), "messages")
				if got := stringMember(t, messages[0], "content"); got != prompt {
					t.Fatalf("content = %q, want the instruction", got)
				}
			},
		},
		{
			name: "two system messages: the first one carries it",
			body: `{"messages":[{"role":"system","content":"first"},{"role":"system","content":"second"}]}`,
			check: func(t *testing.T, in string, out []byte) {
				messages := arrayMember(t, envelopeOf(t, out), "messages")
				if got := stringMember(t, messages[0], "content"); got != "first"+ponytailSeparator+prompt {
					t.Fatalf("the first system message = %q", got)
				}
				if got := stringMember(t, messages[1], "content"); got != "second" {
					t.Fatalf("the second system message changed: %q", got)
				}
			},
		},
		{
			name: "an instruction quoted inside other words is not the instruction",
			body: fmt.Sprintf(`{"messages":[{"role":"system","content":%q}]}`,
				"the skill says: "+prompt+" and that is all"),
			check: func(t *testing.T, in string, out []byte) {
				messages := arrayMember(t, envelopeOf(t, out), "messages")
				got := stringMember(t, messages[0], "content")
				if !strings.HasSuffix(got, ponytailSeparator+prompt) {
					t.Fatalf("the instruction must still be appended: %q", got)
				}
			},
		},
		{
			name: "an already injected segment is left alone",
			body: fmt.Sprintf(`{"messages":[{"role":"system","content":%q}]}`,
				"be terse"+ponytailSeparator+prompt),
			check: func(t *testing.T, in string, out []byte) {
				if string(out) != in {
					t.Fatalf("an injected body must survive byte for byte:\n in: %s\nout: %s", in, out)
				}
			},
		},
		{
			name: "an already injected part is left alone",
			body: fmt.Sprintf(`{"messages":[{"role":"system","content":[{"type":"text","text":%q}]}]}`, prompt),
			check: func(t *testing.T, in string, out []byte) {
				if string(out) != in {
					t.Fatalf("an injected body must survive byte for byte:\n in: %s\nout: %s", in, out)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, tc.body, InjectPonytail([]byte(tc.body), WireOpenAI, PonytailFull))
		})
	}
}
