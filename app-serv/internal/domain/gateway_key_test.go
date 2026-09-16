// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/gateway_key_test.go
// @for       Tests for the GatewayKey aggregate transitions and key masking.
// @uses      testing, time (standard library only).
// @reason    AGENTS.md §2.2 requires the aggregate to enforce its own state
//
//	transitions, and SPEC-API-001 §4 requires plaintext to be masked
//	after creation; these are the invariants a caller cannot be
//	allowed to break (docs/RULLES/TDD.md §2.4).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtechstore.com>
// @layer     domain
// @stability experimental
// @since     2026-09-16
package domain

import (
	"strings"
	"testing"
	"time"
)

var fixedTime = time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)

// TestGatewayKey_New masks and digests the plaintext exactly once.
func TestGatewayKey_New(t *testing.T) {
	const plaintext = "sk-abcdEfghIJKLmnopQRSTuvwx1234567890ABCDEFGH"
	k := NewGatewayKey("ci-runner", plaintext, "sk-…wxyz", fixedTime)

	if !strings.HasPrefix(k.ID(), IDPrefixGatewayKey) {
		t.Fatalf("id %q lacks prefix %q", k.ID(), IDPrefixGatewayKey)
	}
	if k.Status() != GatewayKeyActive {
		t.Fatalf("status = %q, want active", k.Status())
	}
	if k.ValueHash() != HashKey(plaintext) {
		t.Fatal("value_hash must be the digest of the plaintext")
	}
	if k.KeyHint() != "sk-…wxyz" {
		t.Fatal("key_hint must be the masked form of the plaintext")
	}
	if k.CreatedAt() != fixedTime {
		t.Fatalf("created_at = %v, want %v", k.CreatedAt(), fixedTime)
	}
	if k.RevokedAt() != nil {
		t.Fatal("a fresh key must not be revoked")
	}
}

// TestGatewayKey_StatusTransitions covers the allowed and rejected transitions.
func TestGatewayKey_StatusTransitions(t *testing.T) {
	t.Run("disable then re-enable", func(t *testing.T) {
		k := NewGatewayKey("k", "sk-plain1", "sk-…inin", fixedTime)
		if err := k.Transition(GatewayKeyDisabled); err != nil {
			t.Fatalf("disable: unexpected error: %v", err)
		}
		if k.Status() != GatewayKeyDisabled {
			t.Fatalf("status = %q, want disabled", k.Status())
		}
		if err := k.Transition(GatewayKeyActive); err != nil {
			t.Fatalf("re-enable: unexpected error: %v", err)
		}
	})

	t.Run("revocation is terminal", func(t *testing.T) {
		k := NewGatewayKey("k", "sk-plain1", "sk-…inin", fixedTime)
		if err := k.Revoke(fixedTime); err != nil {
			t.Fatalf("revoke: unexpected error: %v", err)
		}
		if k.Status() != GatewayKeyRevoked {
			t.Fatalf("status = %q, want revoked", k.Status())
		}
		if k.RevokedAt() == nil {
			t.Fatal("revoked_at must be set after revocation")
		}
		if err := k.Transition(GatewayKeyActive); err != ErrGatewayKeyRevoked {
			t.Fatalf("revive: err = %v, want %v", err, ErrGatewayKeyRevoked)
		}
		if err := k.Revoke(fixedTime); err != ErrGatewayKeyRevoked {
			t.Fatalf("double revoke: err = %v, want %v", err, ErrGatewayKeyRevoked)
		}
	})

	t.Run("transition via status is rejected", func(t *testing.T) {
		k := NewGatewayKey("k", "sk-plain1", "sk-…inin", fixedTime)
		if err := k.Transition(GatewayKeyRevoked); err == nil {
			t.Fatal("revoking via Transition must be rejected")
		}
	})
}

// TestParseGatewayKeyStatus covers accepted and rejected wire values.
func TestParseGatewayKeyStatus(t *testing.T) {
	cases := []struct {
		in      string
		want    GatewayKeyStatus
		wantErr bool
	}{
		{"active", GatewayKeyActive, false},
		{"disabled", GatewayKeyDisabled, false},
		{"revoked", GatewayKeyRevoked, false},
		{"", "", true},
		{"ACTIVE", "", true},
		{"paused", "", true},
		{"deleted", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseGatewayKeyStatus(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestGatewayKey_Rename covers name validation.
func TestGatewayKey_Rename(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"short", "ci", false},
		{"boundary length", strings.Repeat("a", 120), false},
		{"empty", "", true},
		{"too long", strings.Repeat("a", 121), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := NewGatewayKey("old", "sk-plain1", "sk-…inin", fixedTime)
			err := k.Rename(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if k.Name() != tc.in {
				t.Fatalf("name = %q, want %q", k.Name(), tc.in)
			}
		})
	}
}
