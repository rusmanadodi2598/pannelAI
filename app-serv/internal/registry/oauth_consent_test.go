// Package registry loads the provider registry and answers lookups over it.
//
// @file      internal/registry/oauth_consent_test.go
// @for       The offline consent a Google authorization must ask for, read from the embedded document.
// @uses      testing.
// @reason    Google hands back a refresh token only when the authorization asked for offline access, and a stored account without one stops working the moment its access token expires: the flow reads as a successful login that has to be redone hourly. The reference states this in its authorize builder rather than in its entry data, so nothing a regeneration copies would keep it, and the rule is declared by this port instead. Pinning it against the shipped document is what keeps the pair from quietly disappearing.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-18
package registry

import "testing"

func TestEmbeddedOAuth_GoogleEntriesAskForOfflineConsent(t *testing.T) {
	index, err := Load()
	if err != nil {
		t.Fatalf("loading embedded registry: %v", err)
	}

	for _, id := range []string{"antigravity", "gemini-cli"} {
		t.Run(id, func(t *testing.T) {
			entry, ok := index.Provider(id)
			if !ok {
				t.Fatalf("%s is missing from the embedded registry", id)
			}
			if entry.OAuth == nil {
				t.Fatalf("%s declares no oauth block", id)
			}
			// Both halves matter: access_type decides whether a refresh token is
			// issued, and prompt=consent is what makes Google show the grant screen
			// again rather than silently reusing an earlier approval.
			for name, want := range map[string]string{"access_type": "offline", "prompt": "consent"} {
				if got := entry.OAuth.ExtraParams[name]; got != want {
					t.Fatalf("%s extra_params[%s] = %q, want %q", id, name, got, want)
				}
			}
		})
	}
}

// TestEmbeddedOAuth_ConsentIsDeclaredOnlyWhereItIsNeeded pins the scope of the
// rule: it is entry data under `oauth.extra_params`, the one place the authorize
// builder already merges, and it sticks to the two providers that need it. A
// blanket default would change every other provider's consent screen, and an
// entry with no oauth block at all must stay that way rather than gain one.
func TestEmbeddedOAuth_ConsentIsDeclaredOnlyWhereItIsNeeded(t *testing.T) {
	index, err := Load()
	if err != nil {
		t.Fatalf("loading embedded registry: %v", err)
	}

	want := map[string]bool{"antigravity": true, "gemini-cli": true}
	for _, entry := range index.All() {
		asks := entry.OAuth != nil && entry.OAuth.ExtraParams["access_type"] == "offline"
		if asks != want[entry.ID] {
			t.Fatalf("%s asks for offline consent = %v, want %v", entry.ID, asks, want[entry.ID])
		}
	}
}
