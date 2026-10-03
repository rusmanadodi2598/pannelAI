// Package domain holds the business objects and the rules that guard them.
//
// @file      internal/domain/upstream_endpoint_key_patch_test.go
// @for       The PATCH path's key invariant: an api_key endpoint keeps at least
//
//	one active key through UpdateKey as it does through RemoveKey.
//
// @uses      testing, internal/domain.
// @reason    The invariant lived only in SetKeyStatus, which the PATCH path
//
//	does not call, so a status PATCH could disable the last active key
//	and leave the endpoint unroutable while DELETE of the same key was
//	refused (draft 042 R03).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-10-03
package domain

import "testing"

// TestUpstreamEndpoint_PatchingTheLastActiveKeyToDisabledIsRefused pins the
// invariant on the path the PATCH handler actually calls.
func TestUpstreamEndpoint_PatchingTheLastActiveKeyToDisabledIsRefused(t *testing.T) {
	endpoint := newTestEndpoint(t, "api_key", newTestKey(t, 1))
	key := endpoint.Keys()[0]

	if _, err := endpoint.UpdateKey(key.ID(), key.Label(), "", "", key.Priority(), "disabled", keyNow); err == nil {
		t.Fatal("a PATCH that would leave an api_key endpoint with no active key must be refused")
	}

	// The same PATCH is legal once another key stays active.
	endpoint = newTestEndpoint(t, "api_key", newTestKey(t, 1), newTestKey(t, 2))
	first := endpoint.Keys()[0]
	updated, err := endpoint.UpdateKey(first.ID(), first.Label(), "", "", first.Priority(), "disabled", keyNow)
	if err != nil {
		t.Fatalf("UpdateKey(disabled) error = %v, want it allowed while another key stays active", err)
	}
	if string(updated.Status()) != string(UpstreamKeyDisabled) {
		t.Fatalf("updated status = %q, want disabled", updated.Status())
	}
}
