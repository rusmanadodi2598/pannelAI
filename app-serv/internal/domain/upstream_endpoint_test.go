// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_endpoint_test.go
// @for       Table-driven tests for the endpoint aggregate and its key rules.
// @uses      testing.
// @reason    SPEC-API-001 §7.5 makes the endpoint the mutation boundary for its
//
//	keys and requires it to keep at least one usable credential; these tests
//	pin that invariant and the CRUD rules around it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import (
	"testing"
)

func newTestEndpoint(t *testing.T, authType string, keys ...UpstreamKey) UpstreamEndpoint {
	t.Helper()
	parsed, err := ParseUpstreamAuthType(authType)
	if err != nil {
		t.Fatalf("ParseUpstreamAuthType(%q) error = %v", authType, err)
	}
	endpoint, err := NewUpstreamEndpoint("ep_1", "deepseek", "primary", parsed, 1, keyNow)
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint() error = %v", err)
	}
	for _, key := range keys {
		endpoint.AttachKey(key)
	}
	return endpoint
}

func TestNewUpstreamEndpoint_RejectsMalformedInput(t *testing.T) {
	cases := []struct {
		name       string
		id         string
		providerID string
		label      string
		authType   UpstreamAuthType
		priority   int
		wantErr    bool
	}{
		{name: "valid", id: "ep_1", providerID: "deepseek", label: "primary", authType: UpstreamAuthAPIKey, priority: 1},
		{name: "empty id", providerID: "deepseek", label: "primary", authType: UpstreamAuthAPIKey, priority: 1, wantErr: true},
		{name: "empty provider", id: "ep_1", label: "primary", authType: UpstreamAuthAPIKey, priority: 1, wantErr: true},
		{name: "blank label", id: "ep_1", providerID: "deepseek", label: "  ", authType: UpstreamAuthAPIKey, priority: 1, wantErr: true},
		{name: "zero priority", id: "ep_1", providerID: "deepseek", label: "primary", authType: UpstreamAuthAPIKey, wantErr: true},
		{name: "negative priority", id: "ep_1", providerID: "deepseek", label: "primary", authType: UpstreamAuthAPIKey, priority: -3, wantErr: true},
		{name: "invalid auth type", id: "ep_1", providerID: "deepseek", label: "primary", authType: UpstreamAuthType("cookie"), priority: 1, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewUpstreamEndpoint(tc.id, tc.providerID, tc.label, tc.authType, tc.priority, keyNow)
			if tc.wantErr && err == nil {
				t.Fatal("NewUpstreamEndpoint() = nil error, want an error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("NewUpstreamEndpoint() error = %v", err)
			}
		})
	}
}

// TestUpstreamEndpoint_RemoveKeyKeepsOneUsableCredential pins the invariant from
// SPEC-API-001 §7.5: an api_key endpoint must always retain at least one key,
// because routing it with none is a guaranteed failure. An oauth endpoint is
// exempt: its credential belongs to the flow, not to a key row.
func TestUpstreamEndpoint_RemoveKeyKeepsOneUsableCredential(t *testing.T) {
	cases := []struct {
		name     string
		authType string
		keys     int
		remove   int
		wantErr  bool
	}{
		{name: "api key endpoint keeps its second key", authType: "api_key", keys: 2, remove: 1},
		{name: "api key endpoint refuses to drop the last key", authType: "api_key", keys: 1, remove: 1, wantErr: true},
		{name: "api key endpoint refuses to drop below one", authType: "api_key", keys: 3, remove: 3, wantErr: true},
		{name: "oauth endpoint may hold no keys", authType: "oauth", keys: 0, remove: 0},
		{name: "oauth endpoint may drop its only key", authType: "oauth", keys: 1, remove: 1},
		{name: "no auth endpoint needs no key", authType: "no_auth", keys: 0, remove: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			keys := make([]UpstreamKey, 0, tc.keys)
			for i := 1; i <= tc.keys; i++ {
				keys = append(keys, newTestKey(t, i))
			}
			endpoint := newTestEndpoint(t, tc.authType, keys...)

			// Remove repeatedly until refused or the requested count is reached,
			// so a case removing more keys than may be kept still exercises the
			// refusal rather than looping on a shrinking list.
			var err error
			removed := 0
			for range tc.remove {
				err = endpoint.RemoveKey(keys[removed].ID(), keyNow)
				if err != nil {
					break
				}
				removed++
			}
			if tc.wantErr {
				if err == nil {
					t.Fatal("RemoveKey() = nil error, want it refused")
				}
				// The refusal must leave the endpoint usable: at least one key.
				if len(endpoint.Keys()) < 1 {
					t.Fatal("a refused removal must not empty the endpoint")
				}
				return
			}
			if err != nil {
				t.Fatalf("RemoveKey() error = %v", err)
			}
			if got := len(endpoint.Keys()); got != tc.keys-tc.remove {
				t.Fatalf("keys = %d, want %d", got, tc.keys-tc.remove)
			}
		})
	}
}

// TestUpstreamEndpoint_RemovingTheLastActiveKeyIsRefused covers the narrower
// case the panel slows down for: disabled keys are still rows, but only an
// *active* one lets the endpoint route.
func TestUpstreamEndpoint_RemovingTheLastActiveKeyIsRefused(t *testing.T) {
	active := newTestKey(t, 1)
	disabled := newTestKey(t, 2)
	endpoint := newTestEndpoint(t, "api_key", active, disabled)

	if _, err := endpoint.SetKeyStatus(disabled.ID(), "disabled", keyNow); err != nil {
		t.Fatalf("Transition(disabled) error = %v", err)
	}

	if err := endpoint.RemoveKey(active.ID(), keyNow); err == nil {
		t.Fatal("removing the last active key must be refused while a disabled key remains")
	}

	// Re-enabling the second key makes the removal legal again.
	if _, err := endpoint.SetKeyStatus(disabled.ID(), "active", keyNow); err != nil {
		t.Fatalf("Transition(active) error = %v", err)
	}
	if err := endpoint.RemoveKey(active.ID(), keyNow); err != nil {
		t.Fatalf("RemoveKey() error = %v, want it allowed once another active key exists", err)
	}
}

func TestUpstreamEndpoint_AttachKeyRejectsDuplicates(t *testing.T) {
	endpoint := newTestEndpoint(t, "api_key", newTestKey(t, 1))
	duplicate := newTestKey(t, 1)

	_, err := endpoint.AddKey(duplicate.Label(), duplicate.EncryptedValue(), duplicate.Hint(), 2, keyNow)
	if err == nil {
		t.Fatal("a duplicate label must be refused")
	}

	added, err := endpoint.AddKey("secondary", "ciphertext", "sk-\u2026ondary", 2, keyNow)
	if err != nil {
		t.Fatalf("AddKey() error = %v", err)
	}
	if got := len(endpoint.Keys()); got != 2 {
		t.Fatalf("keys = %d, want 2", got)
	}
	if added.EndpointID() != endpoint.ID() {
		t.Fatalf("new key endpoint = %q, want %q", added.EndpointID(), endpoint.ID())
	}
	if added.Status() != UpstreamKeyActive {
		t.Fatalf("new key status = %q, want active", added.Status())
	}
}

func TestUpstreamEndpoint_TransitionAndRename(t *testing.T) {
	cases := []struct {
		name     string
		status   string
		label    string
		priority int
		wantErr  bool
	}{
		{name: "valid update", status: "active", label: "renamed", priority: 4},
		{name: "disable", status: "disabled", label: "primary", priority: 1},
		{name: "blank label", status: "active", label: "  ", priority: 1, wantErr: true},
		{name: "zero priority", status: "active", label: "primary", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := newTestEndpoint(t, "api_key", newTestKey(t, 1))
			err := endpoint.Update(tc.label, tc.priority, tc.status, keyNow)
			if tc.wantErr {
				if err == nil {
					t.Fatal("Update() = nil error, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Update() error = %v", err)
			}
			if endpoint.Label() != tc.label {
				t.Fatalf("label = %q, want %q", endpoint.Label(), tc.label)
			}
			if endpoint.Priority() != tc.priority {
				t.Fatalf("priority = %d, want %d", endpoint.Priority(), tc.priority)
			}
		})
	}
}

func TestParseUpstreamAuthType(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{in: "api_key"},
		{in: "oauth"},
		{in: "no_auth"},
		{in: "", wantErr: true},
		{in: "API_KEY", wantErr: true},
		{in: "cookie", wantErr: true},
		{in: "bearer", wantErr: true},
	}
	for _, tc := range cases {
		t.Run("auth "+tc.in, func(t *testing.T) {
			_, err := ParseUpstreamAuthType(tc.in)
			if tc.wantErr && err == nil {
				t.Fatalf("ParseUpstreamAuthType(%q) = nil error, want an error", tc.in)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("ParseUpstreamAuthType(%q) error = %v", tc.in, err)
			}
		})
	}
}
