// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/secret_test.go
// @for       Table-driven tests for sealing and opening a stored credential.
// @uses      testing, strings.
// @reason    Upstream keys and OAuth tokens are stored as ciphertext
//
//	(SPEC-API-001 §6), so this is the one place a credential crosses
//	between plaintext and storage. A silent failure here either leaks
//	a credential or makes a stored one unreadable, and neither shows
//	up until a request fails.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import (
	"strings"
	"testing"
)

// testKey is 32 bytes, matching what config.Config validates ENCRYPTION_KEY to.
var testKey = []byte("0123456789abcdef0123456789abcdef")

func TestNewSealer_RejectsAWrongKeyLength(t *testing.T) {
	cases := []struct {
		name    string
		key     []byte
		wantErr bool
	}{
		{name: "exactly 32 bytes", key: testKey},
		{name: "empty", key: nil, wantErr: true},
		{name: "31 bytes", key: testKey[:31], wantErr: true},
		{name: "33 bytes", key: append(append([]byte(nil), testKey...), 'x'), wantErr: true},
		{name: "16 bytes is AES-128, not AES-256", key: testKey[:16], wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewSealer(tc.key)
			if tc.wantErr && err == nil {
				t.Fatal("NewSealer() = nil error, want a refusal")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("NewSealer() error = %v", err)
			}
		})
	}
}

func TestSealer_RoundTrip(t *testing.T) {
	sealer, err := NewSealer(testKey)
	if err != nil {
		t.Fatalf("NewSealer() error = %v", err)
	}

	cases := []struct {
		name      string
		plaintext string
	}{
		{name: "a typical api key", plaintext: "sk-proj-abcdefghijklmnopqrstuvwxyz0123456789"},
		{name: "short", plaintext: "x"},
		{name: "a long credential", plaintext: strings.Repeat("a", 4096)},
		{name: "a refresh token with padding", plaintext: "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.abc-_.~+/=="},
		{name: "a unicode credential", plaintext: "rahasia-\u4e2d\u6587-\U0001F510"},
		{name: "whitespace inside", plaintext: "key with spaces"},
		{name: "a newline inside", plaintext: "line1\nline2"},
		{name: "an empty credential", plaintext: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sealed, err := sealer.Seal(tc.plaintext)
			if err != nil {
				t.Fatalf("Seal() error = %v", err)
			}
			opened, err := sealer.Open(sealed)
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			if opened != tc.plaintext {
				t.Fatalf("round trip = %q, want %q", opened, tc.plaintext)
			}
		})
	}
}

// TestSealer_SealIsNonDeterministic pins the property that makes the nonce
// discipline observable: sealing the same plaintext twice must not produce the
// same stored value, or the ciphertext itself becomes a lookup key an attacker
// can compare against a known credential.
func TestSealer_SealIsNonDeterministic(t *testing.T) {
	sealer, err := NewSealer(testKey)
	if err != nil {
		t.Fatalf("NewSealer() error = %v", err)
	}
	const plaintext = "sk-same-value-both-times"

	first, err := sealer.Seal(plaintext)
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	second, err := sealer.Seal(plaintext)
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	if first == second {
		t.Fatal("sealing the same value twice produced identical ciphertext, so the nonce was reused")
	}
	// Both must still open, which is what proves the nonce is stored, not derived.
	for _, sealed := range []string{first, second} {
		if opened, err := sealer.Open(sealed); err != nil || opened != plaintext {
			t.Fatalf("Open(%q) = (%q, %v), want the original plaintext", sealed, opened, err)
		}
	}
}

func TestSealer_OpenRejectsMalformedInput(t *testing.T) {
	sealer, err := NewSealer(testKey)
	if err != nil {
		t.Fatalf("NewSealer() error = %v", err)
	}
	valid, err := sealer.Seal("sk-round-trip")
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}

	cases := []struct {
		name    string
		sealed  string
		wantErr bool
	}{
		{name: "a sealed value opens", sealed: valid},
		{name: "empty", sealed: "", wantErr: true},
		{name: "no version prefix", sealed: "just-text", wantErr: true},
		{name: "an unknown version", sealed: "v2:AAAA:BBBB", wantErr: true},
		{name: "a missing ciphertext part", sealed: "v1:AAAA", wantErr: true},
		{name: "an extra part", sealed: "v1:AAAA:BBBB:CCCC", wantErr: true},
		{name: "a non-base64 nonce", sealed: "v1:!!!!:BBBB", wantErr: true},
		{name: "a non-base64 ciphertext", sealed: "v1:AAAA:!!!!", wantErr: true},
		{name: "a truncated ciphertext", sealed: "v1:" + strings.SplitN(valid, ":", 3)[1] + ":AAAA", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := sealer.Open(tc.sealed)
			if tc.wantErr && err == nil {
				t.Fatal("Open() = nil error, want a refusal")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Open() error = %v", err)
			}
		})
	}
}

// TestSealer_OpenRejectsAWrongKey is the property the whole scheme rests on: a
// stored credential is unreadable without the key that sealed it, and the
// failure is an error rather than a plausible-looking wrong plaintext.
func TestSealer_OpenRejectsAWrongKey(t *testing.T) {
	cases := []struct {
		name     string
		otherKey []byte
	}{
		{name: "a different key", otherKey: []byte("abcdef0123456789abcdef0123456789")},
		{name: "the same bytes reversed", otherKey: []byte("fedcba9876543210fedcba9876543210")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sealer, err := NewSealer(testKey)
			if err != nil {
				t.Fatalf("NewSealer() error = %v", err)
			}
			sealed, err := sealer.Seal("sk-secret-material")
			if err != nil {
				t.Fatalf("Seal() error = %v", err)
			}

			other, err := NewSealer(tc.otherKey)
			if err != nil {
				t.Fatalf("NewSealer(other) error = %v", err)
			}
			opened, err := other.Open(sealed)
			if err == nil {
				t.Fatalf("Open() with the wrong key = %q, want an error", opened)
			}
			if opened != "" {
				t.Fatalf("Open() with the wrong key returned %q, want nothing", opened)
			}
		})
	}
}

// TestSealer_OpenRejectsTamperedCiphertext covers integrity: GCM authenticates
// the ciphertext, so flipping any byte must be detected rather than decrypted
// into garbage.
func TestSealer_OpenRejectsTamperedCiphertext(t *testing.T) {
	sealer, err := NewSealer(testKey)
	if err != nil {
		t.Fatalf("NewSealer() error = %v", err)
	}
	sealed, err := sealer.Seal("sk-integrity-check")
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	parts := strings.Split(sealed, ":")
	if len(parts) != 3 {
		t.Fatalf("sealed value %q does not have the documented three parts", sealed)
	}

	cases := []struct {
		name   string
		tamper func(ciphertext string) string
	}{
		{name: "a flipped first character", tamper: flipFirst},
		{name: "a truncated value", tamper: func(c string) string { return c[:len(c)-1] }},
		{name: "an appended byte", tamper: func(c string) string { return c + "A" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tampered := "v1:" + parts[1] + ":" + tc.tamper(parts[2])
			if tampered == sealed {
				t.Fatal("the fixture failed to tamper with the ciphertext")
			}
			if opened, err := sealer.Open(tampered); err == nil {
				t.Fatalf("Open(tampered) = %q, want an integrity error", opened)
			}
		})
	}
}

// flipFirst swaps a base64 character for a different one, so the tamper is
// guaranteed to change the decoded bytes rather than possibly landing on the
// same value.
func flipFirst(value string) string {
	if value == "" {
		return value
	}
	if value[0] == 'A' {
		return "B" + value[1:]
	}
	return "A" + value[1:]
}
