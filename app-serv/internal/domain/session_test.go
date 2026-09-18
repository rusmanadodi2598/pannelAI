// Package domain tests session token transformations.
//
// @file      internal/domain/session_test.go
// @for       Table-driven verification of session token signing and parsing.
// @uses      testing, internal/domain.
// @reason    Strict TDD requires generalized coverage of valid and malformed
//
//	opaque session credentials before they guard management routes.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-09-17
package domain

import (
	"strings"
	"testing"
)

func TestSessionTokenRoundTrip(t *testing.T) {
	cases := []struct {
		name   string
		secret string
	}{
		{name: "short test secret", secret: "test-secret"},
		{name: "minimum production secret", secret: strings.Repeat("a", 32)},
		{name: "unicode secret bytes", secret: "秘密-session-secret-0123456789"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token, digest, err := NewSessionToken([]byte(tc.secret))
			if err != nil {
				t.Fatalf("NewSessionToken: %v", err)
			}
			parsed, err := ParseSessionToken(token, []byte(tc.secret))
			if err != nil {
				t.Fatalf("ParseSessionToken: %v", err)
			}
			if parsed != digest || digest == "" {
				t.Fatalf("digest mismatch: got %q want %q", parsed, digest)
			}
		})
	}
}

// tamperFirstChar replaces the first character of a token's nonce with a
// character it is guaranteed not to already have.
//
// Substituting a fixed character is not enough: the nonce is random, so roughly
// one token in sixty-four already begins with that character, leaving the
// "tampered" value byte-identical to the original. The parse then succeeds and
// the test fails for a reason that has nothing to do with rejection, which is a
// nondeterministic test rather than a defect in the code under test.
func tamperFirstChar(token string) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	replacement := byte('x')
	if token[0] == replacement {
		replacement = alphabet[0]
	}
	return string(replacement) + token[1:]
}

func TestSessionTokenRejectsMalformedOrTamperedValues(t *testing.T) {
	secret := []byte(strings.Repeat("s", 32))
	token, _, err := NewSessionToken(secret)
	if err != nil {
		t.Fatal(err)
	}
	tampered := tamperFirstChar(token)
	if tampered == token {
		t.Fatal("the fixture failed to tamper with the token")
	}
	cases := []struct {
		name  string
		value string
		key   []byte
	}{
		{name: "empty", value: "", key: secret},
		{name: "missing separator", value: "abc", key: secret},
		{name: "bad encoding", value: "!invalid.!signature", key: secret},
		{name: "tampered nonce", value: tampered, key: secret},
		{name: "wrong secret", value: token, key: []byte(strings.Repeat("x", 32))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseSessionToken(tc.value, tc.key); err == nil {
				t.Fatal("expected malformed session token to be rejected")
			}
		})
	}
}

func TestSessionTokensUseDistinctNonces(t *testing.T) {
	secret := []byte(strings.Repeat("s", 32))
	seen := make(map[string]struct{}, 5)
	for i := 0; i < 5; i++ {
		token, _, err := NewSessionToken(secret)
		if err != nil {
			t.Fatal(err)
		}
		if _, exists := seen[token]; exists {
			t.Fatal("session nonce repeated")
		}
		seen[token] = struct{}{}
	}
}
