// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/gateway_key_masking_test.go
// @for       Table-driven tests for the key_hint transform.
// @uses      testing (standard library only).
// @reason    SPEC-API-001 §4 fixes the hint shape as sk-…abcd: the hint must
//
//	reveal the credential family and the trailing characters, and
//	nothing of the secret body. Table cases cover typical, boundary,
//	and short inputs so no single fixture can mask a leak
//	(docs/RULLES/TDD.md §2.4).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtechstore.com>
// @layer     service
// @stability experimental
// @since     2026-09-16
package service

import "testing"

// TestMaskGatewayKey_Table verifies the hint never reveals the secret body.
func TestMaskGatewayKey_Table(t *testing.T) {
	const secret = "abcdEfghIJKLmnopQRSTuvwx1234567890ABCDEFGH"

	cases := []struct {
		name   string
		prefix string
		in     string
		want   string
	}{
		{"standard sk- prefix", "sk-", "sk-" + secret, "sk-…EFGH"},
		{"configurable prefix", "pannelai_", "pannelai_" + secret, "pannelai_…EFGH"},
		{"empty prefix", "", secret, "…EFGH"},
		{"exactly prefix plus tail, nothing to hide", "sk-", "sk-abcd", "sk-abcd"},
		{"shorter than the visible window", "sk-", "abc", "abc"},
		{"empty string", "sk-", "", ""},
		{"prefix, no body to hide", "sk-", "abcde", "abcde"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MaskGatewayKey(tc.prefix, tc.in)
			if got != tc.want {
				t.Fatalf("MaskGatewayKey(%q, %q) = %q, want %q", tc.prefix, tc.in, got, tc.want)
			}
		})
	}
}

// TestMaskGatewayKey_LeaksNoBody asserts the transformed hint does not contain
// any interior run of the secret, which is the invariant the hint exists for.
func TestMaskGatewayKey_LeaksNoBody(t *testing.T) {
	const secret = "abcdEfghIJKLmnopQRSTuvwx1234567890ABCDEFGH"
	const prefix = "sk-"

	hint := MaskGatewayKey(prefix, "sk-"+secret)
	body := secret[1 : len(secret)-4]
	if len(body) > 4 {
		for i := 0; i+4 <= len(body); i++ {
			if chunk := body[i : i+4]; len(chunk) >= 4 && contains(hint, chunk) {
				t.Fatalf("hint %q leaks secret body chunk %q", hint, chunk)
			}
		}
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
