// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_endpoint_service_account_test.go
// @for       The service_account auth type: the key-backed rule it shares with api_key, and the project and location the aggregate carries.
// @uses      testing.
// @reason    A service-account endpoint stores its credential like a key but authenticates unlike one, so the boundary between the two rules is where a mistake would hide: the key invariant must apply to it, and the project and location must be storable on no other auth type. The load path's carry-through is pinned too, because a column the rehydrate forgets reads back as "no project" rather than failing.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-12
package domain

import "testing"

// TestUpstreamAuthType_UsesStoredKey pins which auth types hold their credential
// in the keys table, the rule the router, the removal guard, and the create
// validation all read.
func TestUpstreamAuthType_UsesStoredKey(t *testing.T) {
	cases := []struct {
		authType UpstreamAuthType
		want     bool
	}{
		{authType: UpstreamAuthAPIKey, want: true},
		{authType: UpstreamAuthServiceAccount, want: true},
		{authType: UpstreamAuthOAuth, want: false},
		{authType: UpstreamAuthNone, want: false},
	}
	for _, tc := range cases {
		t.Run(string(tc.authType), func(t *testing.T) {
			if got := tc.authType.UsesStoredKey(); got != tc.want {
				t.Fatalf("UsesStoredKey() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestUpstreamEndpoint_SetServiceAccount pins the attach rule: trimmed values
// land on the aggregate, and both refusals hold, an empty field and a wrong auth
// type.
func TestUpstreamEndpoint_SetServiceAccount(t *testing.T) {
	t.Run("stores the trimmed values", func(t *testing.T) {
		endpoint := newTestEndpoint(t, "service_account")
		err := endpoint.SetServiceAccount(ServiceAccountRef{
			ProjectID: "  pannelai-prod  ", Location: " us-central1 ",
		}, keyNow)
		if err != nil {
			t.Fatalf("SetServiceAccount() error = %v", err)
		}
		got := endpoint.ServiceAccount()
		if got.ProjectID != "pannelai-prod" || got.Location != "us-central1" {
			t.Fatalf("ServiceAccount() = %+v, want the trimmed pair", got)
		}
	})

	cases := []struct {
		name     string
		authType string
		ref      ServiceAccountRef
	}{
		{name: "empty project", authType: "service_account", ref: ServiceAccountRef{Location: "us-central1"}},
		{name: "blank location", authType: "service_account", ref: ServiceAccountRef{ProjectID: "p", Location: "  "}},
		{name: "wrong auth type", authType: "api_key", ref: ServiceAccountRef{ProjectID: "p", Location: "us-central1"}},
		// The pair is spliced into a URL path, so a value that is not one
		// plain segment is refused where it is typed rather than built into a
		// request that addresses something else.
		{name: "a project carrying a slash", authType: "service_account", ref: ServiceAccountRef{ProjectID: "a/b", Location: "us-central1"}},
		{name: "a project walking the path", authType: "service_account", ref: ServiceAccountRef{ProjectID: "..", Location: "us-central1"}},
		{name: "a location walking the path", authType: "service_account", ref: ServiceAccountRef{ProjectID: "p", Location: ".."}},
		{name: "a location with a space", authType: "service_account", ref: ServiceAccountRef{ProjectID: "p", Location: "us central1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := newTestEndpoint(t, tc.authType, newTestKey(t, 1))
			if err := endpoint.SetServiceAccount(tc.ref, keyNow); err == nil {
				t.Fatal("SetServiceAccount() = nil error, want a refusal")
			}
		})
	}
}

// TestServiceAccountEndpoint_KeepsOneActiveKey pins that the collection
// invariant follows UsesStoredKey: an endpoint whose credential is an SA JSON
// cannot lose its last usable key any more than an api_key endpoint can.
func TestServiceAccountEndpoint_KeepsOneActiveKey(t *testing.T) {
	endpoint := newTestEndpoint(t, "service_account", newTestKey(t, 1))
	key := endpoint.Keys()[0]

	if err := endpoint.RemoveKey(key.ID(), keyNow); err == nil {
		t.Fatal("RemoveKey() = nil error, want the last-key refusal")
	}
	if _, err := endpoint.UpdateKey(key.ID(), key.Label(), "", "", key.Priority(), "disabled", keyNow); err == nil {
		t.Fatal("UpdateKey(disabled) = nil error, want the last-key refusal")
	}
}

// TestRehydrateUpstreamEndpoint_CarriesTheServiceAccountFields pins the load
// path: the two columns reach the aggregate, so a stored project survives a
// read rather than reading back as unset.
func TestRehydrateUpstreamEndpoint_CarriesTheServiceAccountFields(t *testing.T) {
	endpoint := RehydrateUpstreamEndpoint(
		"ep_01TEST", "vertex", "Primary", UpstreamAuthServiceAccount, 1, UpstreamEndpointActive,
		nil, EndpointAccount{}, EndpointTestStatus{},
		nil, nil, keyNow, keyNow, nil,
		ServiceAccountRef{ProjectID: "pannelai-prod", Location: "us-central1"},
		EndpointParity{},
	)
	got := endpoint.ServiceAccount()
	if got.ProjectID != "pannelai-prod" || got.Location != "us-central1" {
		t.Fatalf("ServiceAccount() = %+v, want the stored pair", got)
	}
}
