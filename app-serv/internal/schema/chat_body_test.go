// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/chat_body_test.go
// @for       Table-driven coverage of the bounded data-plane body reader.
// @uses      net/http, net/http/httptest, strings, testing.
// @reason    F1 of docs/DRAFT/009-PLAYGROUND-CHAT-ENDPOINT-READINESS.md found the
// semantic rules untested at the boundary. The reader is the first rule every
// chat route applies, and an unbounded read is a denial-of-service surface, so
// the limit is pinned at the byte rather than trusted (AGENTS.md §1.4, §1.6).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-21
package schema

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestReadBody_Table pins the three rules the reader owns: a body is present, it
// fits the bound, and it carries something other than whitespace.
func TestReadBody_Table(t *testing.T) {
	cases := []struct {
		name    string
		body    *string
		valid   bool
		message string
	}{
		{name: "a JSON object", body: bodyOf(`{"model":"gpt-4o"}`), valid: true},
		{name: "whitespace only", body: bodyOf("   \n\t "), message: "request body is required"},
		{name: "empty", body: bodyOf(""), message: "request body is required"},
		{name: "exactly the bound", body: bodyOf(strings.Repeat("a", MaxBodyBytes)), valid: true},
		{name: "one byte over the bound", body: bodyOf(strings.Repeat("a", MaxBodyBytes+1)), message: "request body is too large"},
		{name: "far over the bound", body: bodyOf(strings.Repeat("a", MaxBodyBytes*2)), message: "request body is too large"},
		{name: "no body at all", body: nil, message: "request body is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", nil)
			if tc.body != nil {
				request.Body = http.NoBody
				request = httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(*tc.body))
			}
			raw, err := ReadBody(request)
			if (err == nil) != tc.valid {
				t.Fatalf("ReadBody() error = %v, valid = %v", err, tc.valid)
			}
			if tc.message != "" && !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("ReadBody() error = %v, want %q", err, tc.message)
			}
			if tc.valid && string(raw) != *tc.body {
				t.Fatalf("ReadBody() returned %d bytes, want the body verbatim", len(raw))
			}
		})
	}
}

// bodyOf boxes a body so a nil pointer means "no body", which is distinct from
// an empty one.
func bodyOf(value string) *string { return &value }
