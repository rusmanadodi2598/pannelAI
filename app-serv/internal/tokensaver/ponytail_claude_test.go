// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/ponytail_claude_test.go
// @for       The Anthropic cases: the wire whose system slot is a top-level
//
//	member, with a cache breakpoint rule.
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

// TestInjectPonytail_Claude drives the Anthropic wire, whose system slot is a
// top-level member rather than a message.
func TestInjectPonytail_Claude(t *testing.T) {
	prompt := ponytailPrompts[PonytailFull]
	cases := []struct {
		name  string
		body  string
		check func(t *testing.T, in string, out []byte)
	}{
		{
			name: "an absent system member",
			body: `{"model":"m","messages":[{"role":"user","content":"hi"}]}`,
			check: func(t *testing.T, in string, out []byte) {
				envelope := envelopeOf(t, out)
				if got, ok := envelope.memberString("system"); !ok || got != prompt {
					t.Fatalf("system = %q (present %v), want the instruction", got, ok)
				}
				messages := arrayMember(t, envelope, "messages")
				if len(messages) != 1 {
					t.Fatalf("messages = %d, want the one that was there", len(messages))
				}
			},
		},
		{
			name: "a null system member",
			body: `{"model":"m","system":null,"messages":[]}`,
			check: func(t *testing.T, in string, out []byte) {
				if got, ok := envelopeOf(t, out).memberString("system"); !ok || got != prompt {
					t.Fatalf("system = %q (present %v), want the instruction", got, ok)
				}
			},
		},
		{
			name: "a system string",
			body: `{"system":"be terse","messages":[]}`,
			check: func(t *testing.T, in string, out []byte) {
				if got, _ := envelopeOf(t, out).memberString("system"); got != "be terse"+ponytailSeparator+prompt {
					t.Fatalf("system = %q", got)
				}
			},
		},
		{
			name: "a system string already carrying the instruction",
			body: fmt.Sprintf(`{"system":%q,"messages":[]}`, prompt),
			check: func(t *testing.T, in string, out []byte) {
				if string(out) != in {
					t.Fatalf("an injected body must survive byte for byte:\n in: %s\nout: %s", in, out)
				}
			},
		},
		{
			name: "system blocks",
			body: `{"system":[{"type":"text","text":"be terse"}],"messages":[]}`,
			check: func(t *testing.T, in string, out []byte) {
				blocks := arrayMember(t, envelopeOf(t, out), "system")
				if len(blocks) != 2 {
					t.Fatalf("blocks = %d, want the original plus the instruction", len(blocks))
				}
				if got := stringMember(t, blocks[1], "type"); got != "text" {
					t.Fatalf("block type = %q, want text", got)
				}
				if got := stringMember(t, blocks[1], "text"); got != prompt {
					t.Fatalf("block text = %q, want the instruction", got)
				}
			},
		},
		{
			name: "the instruction joins the cached prefix",
			body: `{"system":[{"type":"text","text":"stable","cache_control":{"type":"ephemeral"}},{"type":"text","text":"tail"}],"messages":[]}`,
			check: func(t *testing.T, in string, out []byte) {
				blocks := arrayMember(t, envelopeOf(t, out), "system")
				if len(blocks) != 3 {
					t.Fatalf("blocks = %d, want three", len(blocks))
				}
				if got := stringMember(t, blocks[0], "text"); got != prompt {
					t.Fatalf("block 0 = %q, want the instruction before the cache breakpoint", got)
				}
				if got := stringMember(t, blocks[1], "text"); got != "stable" {
					t.Fatalf("block 1 = %q, want the cached block", got)
				}
				if _, cached := envelopeOf(t, blocks[1])["cache_control"]; !cached {
					t.Fatalf("the cached block lost its marker: %s", blocks[1])
				}
				if got := stringMember(t, blocks[2], "text"); got != "tail" {
					t.Fatalf("block 2 = %q, want the tail block last", got)
				}
			},
		},
		{
			name: "system blocks already carrying the instruction",
			body: fmt.Sprintf(`{"system":[{"type":"text","text":%q}],"messages":[]}`, prompt),
			check: func(t *testing.T, in string, out []byte) {
				if string(out) != in {
					t.Fatalf("an injected body must survive byte for byte:\n in: %s\nout: %s", in, out)
				}
			},
		},
		{
			name: "a system member of another type",
			body: `{"system":42,"messages":[]}`,
			check: func(t *testing.T, in string, out []byte) {
				if string(out) != in {
					t.Fatalf("a system member the wire cannot hold must be left alone:\n in: %s\nout: %s", in, out)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, tc.body, InjectPonytail([]byte(tc.body), WireClaude, PonytailFull))
		})
	}
}
