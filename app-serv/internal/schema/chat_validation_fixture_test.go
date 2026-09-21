// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/chat_validation_fixture_test.go
// @for       The shared body builders and the decode-then-validate assertion
// used by the chat contract tests.
// @uses      testing.
// @reason    Two test files walk the same two steps (decode, validate), so the
// helper lives once beside them rather than being copied. It is separate
// because a fixture is not a rule, and AGENTS.md §1.1 asks for the split
// before the limit forces it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-21
package schema

import "testing"

// assertChatValidity decodes and validates one body, failing when the outcome
// differs from the expectation.
func assertChatValidity(t *testing.T, body string, valid bool) {
	t.Helper()
	req, err := DecodeChatRequest([]byte(body))
	if err == nil {
		err = ValidateStruct(req)
	}
	if (err == nil) != valid {
		t.Fatalf("validation error = %v, valid = %v; body = %s", err, valid, body)
	}
}

// chatMessage wraps one message in an otherwise valid request body.
func chatMessage(message string) string {
	return `{"model":"gpt-4o","messages":[` + message + `]}`
}

// chatWith appends one member to an otherwise valid request body.
func chatWith(member string) string {
	return `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],` + member + `}`
}

// repeat builds a string of n copies, so a bound can be tested at and over its
// limit without a literal.
func repeat(value string, n int) string {
	out := make([]byte, 0, n*len(value))
	for i := 0; i < n; i++ {
		out = append(out, value...)
	}
	return string(out)
}
