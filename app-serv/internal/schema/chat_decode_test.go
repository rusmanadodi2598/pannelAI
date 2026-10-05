// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/chat_decode_test.go
// @for       What a client is told when its chat body cannot be decoded.
// @uses      testing, internal/schema ChatRequest.
// @reason    The decode error is the one place a Go type reached a client: the
//
//	standard message names the destination struct and field type
//	(`[]schema.ChatMessage`), which is a fact about this process rather
//	than about the caller's body, not re-sendable, and nothing the caller
//	can act on. The member at fault is the actionable half, so the test
//	pins that half and forbids the other.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability stable
// @since     2026-09-30
package schema

import (
	"strings"
	"testing"
)

func TestDecodeChatRequest_NamesTheFaultingMemberNotAGoType(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantField string
	}{
		{name: "messages given as a string", body: `{"model":"m","messages":"hi"}`, wantField: "messages"},
		{name: "model given as a number", body: `{"model":7,"messages":[]}`, wantField: "model"},
		{name: "temperature given as a string", body: `{"model":"m","messages":[],"temperature":"hot"}`, wantField: "temperature"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeChatRequest([]byte(tc.body))
			if err == nil {
				t.Fatal("DecodeChatRequest() error = nil, want a refusal")
			}
			message := err.Error()
			if !strings.Contains(message, tc.wantField) {
				t.Fatalf("message = %q, want it to name %q", message, tc.wantField)
			}
			// The leak this guards against is a Go identifier the caller never
			// wrote: the package path, the struct name, or a slice type.
			for _, leaked := range []string{"schema.", "ChatRequest", "[]", "struct field", "unmarshal string into Go"} {
				if strings.Contains(message, leaked) {
					t.Fatalf("message %q leaks the Go type %q", message, leaked)
				}
			}
			if !strings.Contains(message, "invalid request body") {
				t.Fatalf("message = %q, want the client to read which part failed", message)
			}
		})
	}
}

func TestDecodeChatRequest_ReportsMalformedJSONInEnglish(t *testing.T) {
	_, err := DecodeChatRequest([]byte(`{"model":`))
	if err == nil {
		t.Fatal("DecodeChatRequest() error = nil, want a refusal for truncated JSON")
	}
	if strings.Contains(err.Error(), "schema.") || strings.Contains(err.Error(), "ChatRequest") {
		t.Fatalf("message %q leaks a Go type", err.Error())
	}
}

func TestDecodeChatRequest_AcceptsAWellFormedBody(t *testing.T) {
	req, err := DecodeChatRequest([]byte(`{"model":"m","messages":[{"role":"user","content":"hi"}],"stop":"x"}`))
	if err != nil {
		t.Fatalf("DecodeChatRequest() error = %v", err)
	}
	if got := req.StopSequences(); len(got) != 1 || got[0] != "x" {
		t.Fatalf("StopSequences() = %v, want the single string the caller sent", got)
	}
}
