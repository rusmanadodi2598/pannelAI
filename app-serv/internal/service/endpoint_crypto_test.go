// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_crypto_test.go
// @for       The build-time credential-sealing helpers and the AES-GCM round-trip
//
//	the endpoint service depends on.
//
// @uses      strings, testing, internal/domain.
// @reason    SPEC-API-001 §6 requires upstream credentials encrypted at rest, and the
//
//	service's whole guarantee is that it hands the aggregate ciphertext
//	rather than plaintext. The sealer itself is domain-owned and tested
//	there; what this file pins is the composition the service relies on —
//	every stored value is v1 ciphertext, the same plaintext seals to
//	different ciphertext each time, and opening returns exactly what was
//	sealed. That is the property a bug would silently break.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// testEncryptionKey is a fixed 32-byte key, so a test never depends on the
// environment. It is deliberately not a realistic key literal: a value that looked
// like a deployed secret is exactly what the secrets gate must flag.
var testEncryptionKey = []byte(strings.Repeat("k", 32))

// newTestSealer returns the production sealer built on the fixed test key.
func newTestSealer(t *testing.T) *domain.Sealer {
	t.Helper()
	sealer, err := domain.NewSealer(testEncryptionKey)
	if err != nil {
		t.Fatalf("NewSealer() error = %v", err)
	}
	return sealer
}

// isSealed reports whether a stored value is the sealed form this project writes:
// the version prefix, a nonce, and a body.
func isSealed(value string) bool {
	parts := strings.Split(value, ":")
	return len(parts) == 3 && parts[0] == "v1" && parts[1] != "" && parts[2] != ""
}

// TestSealerRoundTripThroughTheServiceGuarantee pins the encryption contract the
// service composes on: seal then open returns the plaintext exactly, the stored form
// is recognisable ciphertext, and the same input seals differently each time because
// the nonce is fresh per call.
func TestSealerRoundTripThroughTheServiceGuarantee(t *testing.T) {
	sealer := newTestSealer(t)

	cases := []struct {
		name      string
		plaintext string
	}{
		{name: "a typical api key", plaintext: "sk-live-abcdef1234567890"},
		{name: "a one-character credential", plaintext: "x"},
		{name: "a value with punctuation and spaces", plaintext: "token with spaces & symbols: \t\n\"'"},
		{name: "a long token", plaintext: strings.Repeat("a", 4096)},
		{name: "a unicode value", plaintext: "kunci-secret-ñ-日本"},
		{name: "an empty value", plaintext: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sealed, err := sealer.Seal(tc.plaintext)
			if err != nil {
				t.Fatalf("Seal() error = %v", err)
			}
			if !isSealed(sealed) {
				t.Fatalf("sealed = %q, want the v1:<nonce>:<body> form", sealed)
			}
			if sealed == tc.plaintext {
				t.Fatal("the sealed value must not be the plaintext")
			}

			opened, err := sealer.Open(sealed)
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			if opened != tc.plaintext {
				t.Fatalf("Open() = %q, want %q", opened, tc.plaintext)
			}

			// A fresh nonce means the same plaintext seals differently, which is
			// what stops two equal credentials from being detectable in storage.
			again, err := sealer.Seal(tc.plaintext)
			if err != nil {
				t.Fatalf("second Seal() error = %v", err)
			}
			if again == sealed {
				t.Fatal("sealing the same plaintext twice must not produce identical ciphertext")
			}
		})
	}
}

// TestSealerRejectsMalformedStoredValues pins that Open never returns a plausible
// wrong plaintext: every malformed shape is an error instead.
func TestSealerRejectsMalformedStoredValues(t *testing.T) {
	sealer := newTestSealer(t)
	valid, err := sealer.Seal("a-credential")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(valid, ":")

	cases := []struct {
		name   string
		sealed string
	}{
		{name: "empty", sealed: ""},
		{name: "not a sealed value", sealed: "plaintext"},
		{name: "an unknown version", sealed: "v9:" + parts[1] + ":" + parts[2]},
		{name: "too few fields", sealed: "v1:" + parts[1]},
		{name: "too many fields", sealed: valid + ":extra"},
		{name: "a bad nonce encoding", sealed: "v1:not-base64!!:" + parts[2]},
		{name: "a bad body encoding", sealed: "v1:" + parts[1] + ":not-base64!!"},
		{name: "a truncated body", sealed: "v1:" + parts[1] + ":" + parts[2][:4]},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opened, err := sealer.Open(tc.sealed)
			if err == nil {
				t.Fatalf("Open(%q) = %q, want an error", tc.sealed, opened)
			}
			if opened != "" {
				t.Fatalf("Open() returned %q alongside an error, want no plaintext", opened)
			}
		})
	}
}

// TestSealerOpenOnTheWrongKeyFails asserts a ciphertext sealed under one key cannot
// be opened under another, which is what makes key rotation detectable rather than
// silently wrong.
func TestSealerOpenOnTheWrongKeyFails(t *testing.T) {
	first := newTestSealer(t)
	other, err := domain.NewSealer([]byte(strings.Repeat("z", 32)))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := first.Seal("a-credential")
	if err != nil {
		t.Fatal(err)
	}
	if opened, err := other.Open(sealed); err == nil {
		t.Fatalf("Open() under the wrong key = %q, want an error", opened)
	}
}
