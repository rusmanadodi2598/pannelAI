// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/ponytail_test.go
// @for       The shared test helpers, the chat-wire cases, and the inputs that
//
//	must leave a body untouched.
//
// @uses      encoding/json, fmt, strings, testing.
// @reason    SPEC-API-002 §7 makes injection idempotent and leaves the wire
//
//	shapes to disagree about where a system slot lives. Both are claims
//	about bytes, so each case reads the body back rather than trusting
//	the injector's own report.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import (
	"encoding/json"
	"strings"
	"testing"
)

// envelopeOf decodes a body the injector returned.
func envelopeOf(t *testing.T, body []byte) object {
	t.Helper()
	envelope, ok := decodeObject(body)
	if !ok {
		t.Fatalf("the body is not a JSON object: %s", body)
	}
	return envelope
}

// arrayMember reads one array member of an envelope.
func arrayMember(t *testing.T, envelope object, member string) []json.RawMessage {
	t.Helper()
	items, ok := decodeArray(envelope[member])
	if !ok {
		t.Fatalf("the %s member is not an array: %s", member, envelope[member])
	}
	return items
}

// stringMember reads one string member of a raw object.
func stringMember(t *testing.T, raw json.RawMessage, member string) string {
	t.Helper()
	value, ok := envelopeOf(t, raw).memberString(member)
	if !ok {
		t.Fatalf("the %s member is not a string: %s", member, raw)
	}
	return value
}

// TestInjectPonytail_LeavesUntouchedInputs drives the inputs the injector must
// return byte for byte: an unknown level, an unknown wire, and a body it cannot
// read. A saver is an optimization, so none of them may fail a request.
func TestInjectPonytail_LeavesUntouchedInputs(t *testing.T) {
	chat := `{"messages":[{"role":"system","content":"be terse"}]}`
	cases := []struct {
		name  string
		body  string
		wire  string
		level string
	}{
		{name: "an unknown level", body: chat, wire: WireOpenAI, level: "off"},
		{name: "an upper-case level", body: chat, wire: WireOpenAI, level: "FULL"},
		{name: "an empty level", body: chat, wire: WireOpenAI, level: ""},
		{name: "an unknown wire", body: chat, wire: "gemini", level: PonytailFull},
		{name: "an empty wire", body: chat, wire: "", level: PonytailFull},
		{name: "a body that is not JSON", body: "not json at all", wire: WireOpenAI, level: PonytailFull},
		{name: "a body that is an array", body: `[{"role":"system","content":"hi"}]`, wire: WireOpenAI, level: PonytailFull},
		{name: "a body that is the null literal", body: `null`, wire: WireOpenAI, level: PonytailFull},
		{name: "an empty body", body: ``, wire: WireOpenAI, level: PonytailFull},
		{name: "a chat body with no messages member", body: `{"model":"m"}`, wire: WireOpenAI, level: PonytailFull},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := InjectPonytail([]byte(tc.body), tc.wire, tc.level)
			if string(out) != tc.body {
				t.Fatalf("the body changed:\n in: %s\nout: %s", tc.body, out)
			}
		})
	}
}

// TestInjectPonytail_KeepsSiblingMembers asserts the bytes the injector must not
// touch: a body's other members, including one carrying characters json.Marshal
// would escape by default.
func TestInjectPonytail_KeepsSiblingMembers(t *testing.T) {
	body := []byte(`{"model":"m","temperature":0.5,"tools":[{"type":"function","function":{"name":"f"}}],"metadata":{"note":"a <b> & c"},"messages":[{"role":"system","content":"be terse"}]}`)
	before := envelopeOf(t, body)
	after := envelopeOf(t, InjectPonytail(body, WireOpenAI, PonytailFull))
	for _, member := range []string{"model", "temperature", "tools", "metadata"} {
		if string(before[member]) != string(after[member]) {
			t.Fatalf("%s changed:\n in: %s\nout: %s", member, before[member], after[member])
		}
	}
	if !strings.Contains(string(after["metadata"]), "a <b> & c") {
		t.Fatalf("the encoder escaped HTML: %s", after["metadata"])
	}
}
