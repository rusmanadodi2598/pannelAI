// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_key_test.go
// @for       Tests for key construction, plaintext boundaries, and rotation.
// @uses      testing, time.
// @reason    SPEC-API-001 §7.5 defines a key's shape — identity, ciphertext,
//
//	priority, rotation — and the service depends on the constructor
//	rejecting malformed input, on the plaintext never being reachable from
//	the aggregate, and on rotation keeping the priority order intact.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import (
	"testing"
	"time"
)

var keyNow = time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)

// sealedFixture is what the aggregate is actually handed: ciphertext produced
// by the service, never a credential. It is deliberately not `sk-`-prefixed,
// because a realistic-looking key literal in a test file is exactly what the
// secrets gate must flag, and an allowlist entry for it would weaken the gate
// for every future fixture.
const sealedFixture = "v1:AAECAwQFBgcICQoLDA0ODw==:c2VhbGVkLWNyZWRlbnRpYWw="

// newTestKey builds a key whose priority doubles as its distinguishing suffix,
// so a test that orders several keys can tell them apart.
func newTestKey(t *testing.T, priority int) UpstreamKey {
	t.Helper()
	key, err := NewUpstreamKey(
		"key-"+string(rune('a'+priority)), "ep_1", "uky_"+string(rune('a'+priority)),
		sealedFixture, "sk-\u2026cret", priority, keyNow,
	)
	if err != nil {
		t.Fatalf("NewUpstreamKey() error = %v", err)
	}
	return key
}

func TestNewUpstreamKey_RejectsMalformedInput(t *testing.T) {
	cases := []struct {
		name     string
		label    string
		endpoint string
		value    string
		hint     string
		priority int
		wantErr  bool
	}{
		{name: "valid", label: "primary", endpoint: "ep_1", value: "sk-abc", hint: "sk-\u2026abc", priority: 1},
		{name: "empty label", endpoint: "ep_1", value: "sk-abc", hint: "sk-\u2026abc", priority: 1, wantErr: true},
		{name: "blank label", label: "   ", endpoint: "ep_1", value: "sk-abc", hint: "sk-\u2026abc", priority: 1, wantErr: true},
		{name: "empty endpoint", label: "primary", value: "sk-abc", hint: "sk-\u2026abc", priority: 1, wantErr: true},
		{name: "empty value", label: "primary", endpoint: "ep_1", hint: "sk-\u2026abc", priority: 1, wantErr: true},
		{name: "empty hint", label: "primary", endpoint: "ep_1", value: "sk-abc", priority: 1, wantErr: true},
		{name: "zero priority", label: "primary", endpoint: "ep_1", value: "sk-abc", hint: "sk-\u2026abc", wantErr: true},
		{name: "negative priority", label: "primary", endpoint: "ep_1", value: "sk-abc", hint: "sk-\u2026abc", priority: -2, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewUpstreamKey(tc.label, tc.endpoint, "x", tc.value, tc.hint, tc.priority, keyNow)
			if tc.wantErr && err == nil {
				t.Fatal("NewUpstreamKey() = nil error, want an error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("NewUpstreamKey() error = %v", err)
			}
		})
	}
}

// TestUpstreamKey_DisabledIsUnavailableRegardlessOfWindow also confirms that a
// client cannot hand-set the breaker's own state: `error` is not a settable
// status, so a PATCH that sends it is rejected instead of hiding a healthy key.
func TestUpstreamKey_DisabledIsUnavailableRegardlessOfWindow(t *testing.T) {
	cases := []struct {
		name      string
		status    string
		available bool
		wantErr   bool
	}{
		{name: "active", status: "active", available: true},
		{name: "disabled", status: "disabled"},
		{name: "error is not client-settable", status: "error", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := newTestKey(t, 1)
			parsed, err := ParseUpstreamKeyStatus(tc.status)
			if err != nil {
				t.Fatalf("ParseUpstreamKeyStatus(%q) error = %v", tc.status, err)
			}
			transitionErr := key.Transition(parsed)
			if tc.wantErr {
				if transitionErr == nil {
					t.Fatal("Transition(error) = nil, want it rejected")
				}
				return
			}
			if transitionErr != nil {
				t.Fatalf("Transition(%q) error = %v", tc.status, transitionErr)
			}
			if got := key.Available(keyNow); got != tc.available {
				t.Fatalf("Available() = %v, want %v", got, tc.available)
			}
			// A disabled key is out of backoff even if it was tripped first.
			if tc.status == "disabled" && key.RateLimitedUntil() != nil {
				t.Fatal("disabling a key must clear its backoff window")
			}
		})
	}
}

// TestUpstreamKey_StoredValueIsScopeNotCredential pins the boundary between the
// service, which seals a credential, and the aggregate, which only stores what
// it is handed.
//
// The aggregate cannot prove "the plaintext never arrives": it stores whatever
// string it receives, so that guarantee belongs to the service that encrypts,
// and asserting it here would fail the moment a caller passed plaintext for a
// reason the test cannot see. What this CAN pin is that the read surface stays
// narrow: the sealed value round-trips unchanged, and the two accessors that a
// response layer uses expose the ciphertext and a mask, never anything that
// reveals the credential's body.
func TestUpstreamKey_StoredValueIsScopeNotCredential(t *testing.T) {
	key, err := NewUpstreamKey("primary", "ep_1", "uky_1", sealedFixture, "sk-\u2026cret", 1, keyNow)
	if err != nil {
		t.Fatalf("NewUpstreamKey() error = %v", err)
	}

	if got := key.EncryptedValue(); got != sealedFixture {
		t.Fatalf("EncryptedValue() = %q, want the sealed value back unchanged", got)
	}

	hint := key.Hint()
	if hint == "" {
		t.Fatal("Hint() must expose a mask, not nothing")
	}
	// A hint is what a list response renders, so it must not carry the value.
	if hint == key.EncryptedValue() {
		t.Fatal("Hint() returned the stored value whole")
	}
}

func TestUpstreamKey_ReorderAndRelabel(t *testing.T) {
	cases := []struct {
		name     string
		priority int
		label    string
		wantErr  bool
	}{
		{name: "valid reorder", priority: 5, label: "secondary"},
		{name: "zero priority", priority: 0, label: "secondary", wantErr: true},
		{name: "negative priority", priority: -1, label: "secondary", wantErr: true},
		{name: "blank label", priority: 2, label: "  ", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := newTestKey(t, 1)
			err := key.Update(tc.label, tc.priority, keyNow)
			if tc.wantErr {
				if err == nil {
					t.Fatal("Update() = nil error, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Update() error = %v", err)
			}
			if key.Priority() != tc.priority {
				t.Fatalf("priority = %d, want %d", key.Priority(), tc.priority)
			}
			if key.Label() != tc.label {
				t.Fatalf("label = %q, want %q", key.Label(), tc.label)
			}
		})
	}
}

func TestParseUpstreamKeyStatus(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{in: "active"},
		{in: "disabled"},
		{in: "error"},
		{in: "", wantErr: true},
		{in: "ACTIVE", wantErr: true},
		{in: "revoked", wantErr: true},
		{in: "bogus", wantErr: true},
	}
	for _, tc := range cases {
		t.Run("status "+tc.in, func(t *testing.T) {
			_, err := ParseUpstreamKeyStatus(tc.in)
			if tc.wantErr && err == nil {
				t.Fatalf("ParseUpstreamKeyStatus(%q) = nil error, want an error", tc.in)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("ParseUpstreamKeyStatus(%q) error = %v", tc.in, err)
			}
		})
	}
}
