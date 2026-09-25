// Package registry holds the embedded provider registry.
//
// @file      internal/registry/credential_free_test.go
// @for       The credential-free predicate both routing and the catalog read.
// @uses      testing, internal/registry.
// @reason    Draft 029 §4.8 F8 made routing synthesize an endpoint for a
//
//	credential-free provider, and the catalog's `?active=true` filter
//	has to give the same answer or the panel hides a lane the router
//	serves. The predicate therefore has more than one caller, and these
//	cases pin each spelling the document uses rather than the one the
//	live registry happened to carry.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-24
package registry

import "testing"

// TestProvider_NeedsNoCredential pins every spelling that makes a provider
// credential-free, and the keyed cases it must not claim: a predicate that
// answered true for a keyed provider would make the catalog offer a lane whose
// first request answers NO_PROVIDER_AVAILABLE.
func TestProvider_NeedsNoCredential(t *testing.T) {
	cases := []struct {
		name  string
		entry Provider
		want  bool
	}{
		{
			name:  "no_auth declared on the provider",
			entry: Provider{ID: "a", Category: "free", NoAuth: true},
			want:  true,
		},
		{
			name:  "no_auth declared on the transport",
			entry: Provider{ID: "a", Category: "free", Transport: Transport{NoAuth: true}},
			want:  true,
		},
		{
			name:  "auth_type says no_auth",
			entry: Provider{ID: "a", Category: "free", AuthType: AuthNone},
			want:  true,
		},
		{
			name:  "a keyed provider is not credential-free",
			entry: Provider{ID: "a", Category: "apikey", AuthType: AuthAPIKey},
			want:  false,
		},
		{
			name:  "an oauth provider is not credential-free",
			entry: Provider{ID: "a", Category: "oauth", AuthType: AuthOAuth},
			want:  false,
		},
		{
			name:  "an entry declaring nothing is keyed by the loader's default",
			entry: Provider{ID: "a", Category: "apikey"},
			want:  false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.entry.NeedsNoCredential(); got != tc.want {
				t.Fatalf("NeedsNoCredential() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestProvider_NeedsNoCredentialAfterBuildAuthType pins the predicate against a
// loaded entry rather than a hand-built one: BuildAuthType derives AuthType from
// the same two flags, and the predicate has to keep answering true after that
// derivation runs, which is the state every entry in the index is in.
func TestProvider_NeedsNoCredentialAfterBuildAuthType(t *testing.T) {
	entry := Provider{ID: "a", Category: "free", Transport: Transport{NoAuth: true}}
	entry.BuildAuthType()
	if entry.AuthType != AuthNone {
		t.Fatalf("AuthType = %q, want %q after BuildAuthType", entry.AuthType, AuthNone)
	}
	if !entry.NeedsNoCredential() {
		t.Fatal("NeedsNoCredential() = false after BuildAuthType, want the entry still credential-free")
	}
}
