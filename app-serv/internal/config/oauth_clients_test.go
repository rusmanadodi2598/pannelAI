// Package config reads and validates the process environment.
//
// @file      internal/config/oauth_clients_test.go
// @for       The typed OAuth client material a deployment may supply for a provider's flow.
// @uses      testing.
// @reason    The reference ships public installed-app client secrets in its registry, and this repository commits generated registry data, so a secret read from the document would be a secret in git history. These tests hold the two rules that keep the secret out of the document: it is read from the environment per provider, and an unset one leaves the provider on the registry's client id rather than a broken round.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-10-17
package config

import "testing"

func TestLoadOAuthClients_ReadsEachProviderSeparately(t *testing.T) {
	t.Setenv("GEMINI_CLI_OAUTH_CLIENT_SECRET", "gc-secret")
	t.Setenv("ANTIGRAVITY_OAUTH_CLIENT_ID", "own-client.apps.googleusercontent.com")
	t.Setenv("ANTIGRAVITY_OAUTH_CLIENT_SECRET", "ag-secret")

	clients := loadOAuthClients()

	if got := clients["gemini-cli"].ClientSecret; got != "gc-secret" {
		t.Fatalf("gemini-cli secret = %q, want the value of GEMINI_CLI_OAUTH_CLIENT_SECRET", got)
	}
	// A provider whose client id is not overridden stays empty here: the registry
	// entry is the fallback, and inventing one in config would give one value two
	// sources that can drift apart.
	if got := clients["gemini-cli"].ClientID; got != "" {
		t.Fatalf("gemini-cli client id = %q, want empty so the registry entry is used", got)
	}
	if got := clients["antigravity"].ClientID; got != "own-client.apps.googleusercontent.com" {
		t.Fatalf("antigravity client id = %q, want the configured override", got)
	}
	if got := clients["antigravity"].ClientSecret; got != "ag-secret" {
		t.Fatalf("antigravity secret = %q, want the value of ANTIGRAVITY_OAUTH_CLIENT_SECRET", got)
	}
}

func TestLoadOAuthClients_LeavesAnUnconfiguredProviderAbsent(t *testing.T) {
	clients := loadOAuthClients()

	// A map holding an empty entry would read as "configured with nothing", which
	// the flow cannot tell apart from a provider that declares no client at all.
	if len(clients) != 0 {
		t.Fatalf("no environment set, got %d configured providers: an unset provider must be absent", len(clients))
	}
}
