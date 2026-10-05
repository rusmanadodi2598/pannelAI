// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_endpoint_keys_test.go
// @for       The credential-set mutations an UpstreamEndpoint refuses.
// @uses      testing.
// @reason    "An api_key endpoint must keep at least one active key" is a rule
//
//	about the collection, so removal, status change and the duplicate
//	label check are pinned together rather than spread over the entity
//	table.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-04

package domain

import (
	"testing"
)

// setKeyStatus changes one key's status through UpdateKey, the only path the
// panel has to a key's state.
func setKeyStatus(t *testing.T, endpoint *UpstreamEndpoint, key UpstreamKey, status string) {
	t.Helper()
	if _, err := endpoint.UpdateKey(key.ID(), key.Label(), "", "", key.Priority(), status, keyNow); err != nil {
		t.Fatalf("UpdateKey(%s) error = %v", status, err)
	}
}

// keyAtPriority finds the fixture key carrying a priority.
func keyAtPriority(t *testing.T, endpoint UpstreamEndpoint, priority int) UpstreamKey {
	t.Helper()
	for _, key := range endpoint.Keys() {
		if key.Priority() == priority {
			return key
		}
	}
	t.Fatalf("no fixture key at priority %d", priority)
	return UpstreamKey{}
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

	setKeyStatus(t, &endpoint, disabled, "disabled")

	if err := endpoint.RemoveKey(active.ID(), keyNow); err == nil {
		t.Fatal("removing the last active key must be refused while a disabled key remains")
	}

	// Re-enabling the second key makes the removal legal again.
	setKeyStatus(t, &endpoint, disabled, "active")
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
