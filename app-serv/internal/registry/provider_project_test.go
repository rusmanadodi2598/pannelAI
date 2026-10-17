// Package registry loads the provider registry and answers lookups over it.
//
// @file      internal/registry/provider_project_test.go
// @for       Which registry declaration names the endpoint that resolves an account's Cloud project.
// @uses      testing.
// @reason    The reference carries that endpoint in three places across its Google family, and a reader that checks only one of them silently skips the bootstrap for the providers that declared it elsewhere. The accessor exists so the flow has one question to ask, and these tests hold the precedence and the two entries that answer it today.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-17
package registry

import "testing"

func TestProjectBootstrap_ReadsEachDeclaredSpelling(t *testing.T) {
	const assist = "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist"

	cases := []struct {
		name    string
		entries Provider
		wantURL string
		wantUA  string
	}{
		{
			name:    "oauth block wins when it declares one",
			entries: Provider{OAuth: &OAuth{LoadCodeAssistURL: assist, LoadCodeAssistUA: "antigravity/ide/1.0"}},
			wantURL: assist, wantUA: "antigravity/ide/1.0",
		},
		{
			name:    "transport usage block is read next",
			entries: Provider{Transport: Transport{Usage: UsageConfig{LoadCodeAssistURL: assist}}},
			wantURL: assist,
		},
		{
			// Antigravity carries this spelling and not the one above, on its usage block.
			name:    "the usage project api url is the third spelling",
			entries: Provider{Transport: Transport{Usage: UsageConfig{LoadProjectAPIURL: assist}}},
			wantURL: assist,
		},
		{
			name:    "an entry that declares nothing answers empty",
			entries: Provider{OAuth: &OAuth{ClientID: "cid"}},
		},
		{
			name:    "a bare entry is safe to ask",
			entries: Provider{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			url, userAgent := tc.entries.ProjectBootstrap()
			if url != tc.wantURL {
				t.Fatalf("bootstrap url = %q, want %q", url, tc.wantURL)
			}
			if userAgent != tc.wantUA {
				t.Fatalf("bootstrap user agent = %q, want %q", userAgent, tc.wantUA)
			}
		})
	}
}

// TestProjectBootstrap_EmbeddedEntries pins what the shipped document actually
// says, so a regeneration that drops a declaration is caught here rather than
// becoming a login that silently stores no project.
func TestProjectBootstrap_EmbeddedEntries(t *testing.T) {
	index, err := Load()
	if err != nil {
		t.Fatalf("loading embedded registry: %v", err)
	}

	cases := []struct {
		provider string
		wantURL  string
		wantUA   string
	}{
		{
			provider: "antigravity",
			wantURL:  "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist",
			wantUA:   "antigravity/ide/2.11.0 darwin/arm64",
		},
		{
			provider: "gemini-cli",
			wantURL:  "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist",
		},
		{provider: "gemini", wantURL: ""},
		{provider: "vertex", wantURL: ""},
	}

	for _, tc := range cases {
		t.Run(tc.provider, func(t *testing.T) {
			entry, ok := index.Provider(tc.provider)
			if !ok {
				t.Fatalf("%s is missing from the embedded registry", tc.provider)
			}
			url, userAgent := entry.ProjectBootstrap()
			if url != tc.wantURL {
				t.Fatalf("%s bootstrap url = %q, want %q", tc.provider, url, tc.wantURL)
			}
			if userAgent != tc.wantUA {
				t.Fatalf("%s bootstrap user agent = %q, want %q", tc.provider, userAgent, tc.wantUA)
			}
		})
	}
}
