// Package registry loads the provider registry and answers lookups over it.
//
// @file      internal/registry/oauth_startable_test.go
// @for       The derived OAuth flag: which loaded entries offer a round the gateway can start.
// @uses      testing.
// @reason    The reference marks hasOAuth on some entries and not on others, and for the Google family it never marks it at all while shipping a complete flow: its own panel decides by category instead. The port reads the flag as the panel's gate, so an entry with a startable round and no flag is a provider an operator cannot connect. Pinning the derivation here is what keeps the gate and the flow data from disagreeing again.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-17
package registry

import "testing"

// TestSupportsFlow_AdmitsOnlyBlocksWithARoundToStart pins the predicate the
// derivation reads, including the shape that must answer false: a block naming
// only a client id, which no shared client can start a round from.
func TestSupportsFlow_AdmitsOnlyBlocksWithARoundToStart(t *testing.T) {
	cases := []struct {
		name  string
		oauth *OAuth
		want  bool
	}{
		{name: "nil block", oauth: nil, want: false},
		{name: "empty block", oauth: &OAuth{}, want: false},
		{name: "bare client id", oauth: &OAuth{ClientID: "cid"}, want: false},
		{name: "authorization endpoint", oauth: &OAuth{AuthorizeURL: "https://example.com/auth"}, want: true},
		{name: "device code endpoint", oauth: &OAuth{DeviceCodeURL: "https://example.com/device"}, want: true},
		{name: "device auth endpoint", oauth: &OAuth{DeviceAuthURL: "https://example.com/authorize"}, want: true},
		{name: "state round", oauth: &OAuth{StateURL: "https://example.com/state"}, want: true},
		{name: "a token url alone", oauth: &OAuth{TokenURL: "https://example.com/token"}, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.oauth.SupportsFlow(); got != tc.want {
				t.Fatalf("SupportsFlow() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestLoadedOAuth_FlagsTheEntriesThePortCanConnect reads the embedded document,
// not a fixture: an entry is either connectable in the binary that ships or it
// is not, and a fixture would prove only the predicate.
func TestLoadedOAuth_FlagsTheEntriesThePortCanConnect(t *testing.T) {
	index, err := Load()
	if err != nil {
		t.Fatalf("loading embedded registry: %v", err)
	}

	cases := []struct {
		provider string
		want     bool
		why      string
	}{
		{provider: "antigravity", want: true, why: "declares the full Google code round"},
		{provider: "gemini-cli", want: true, why: "declares the full Google code round"},
		{
			provider: "gemini", want: false,
			why: "its oauth block names a client id and no round to start; it is an API-key provider",
		},
		{
			provider: "vertex", want: false,
			why: "authenticates with a service account, not a round",
		},
		{
			provider: "kilocode", want: false,
			why: "its exchange needs a connector that does not exist yet",
		},
		{provider: "claude", want: true, why: "declares an authorization endpoint"},
		{provider: "github", want: true, why: "declares an authorization endpoint"},
	}

	for _, tc := range cases {
		t.Run(tc.provider, func(t *testing.T) {
			entry, ok := index.Provider(tc.provider)
			if !ok {
				t.Fatalf("%s is missing from the embedded registry", tc.provider)
			}
			if entry.HasOAuth != tc.want {
				t.Fatalf("%s HasOAuth = %v, want %v: %s", tc.provider, entry.HasOAuth, tc.want, tc.why)
			}
		})
	}
}

// TestLoadedOAuth_NeverErasesADeclaredFlag holds the derivation additive. A
// provider whose flow the predicate cannot describe (kimchi and xai declare the
// flag with no endpoint the shared client reads) is connected today, and a rule
// that recomputed the flag from scratch would hide their OAuth section.
func TestLoadedOAuth_NeverErasesADeclaredFlag(t *testing.T) {
	raw, err := embedded.ReadFile(registryFile)
	if err != nil {
		t.Fatalf("reading embedded registry: %v", err)
	}
	doc, err := decode(raw)
	if err != nil {
		t.Fatalf("decoding embedded registry: %v", err)
	}
	index, err := NewIndex(doc)
	if err != nil {
		t.Fatalf("building index: %v", err)
	}

	declared := 0
	for _, provider := range doc.Providers {
		if !provider.HasOAuth {
			continue
		}
		declared++
		entry, ok := index.Provider(provider.ID)
		if !ok {
			t.Fatalf("%s vanished from the index", provider.ID)
		}
		if !entry.HasOAuth {
			t.Fatalf("%s declared has_oauth and the derived flag erased it", provider.ID)
		}
	}
	if declared == 0 {
		t.Fatal("no provider declares has_oauth: the document changed shape and this test is now vacuous")
	}
}
