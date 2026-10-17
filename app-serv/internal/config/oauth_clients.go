// Package config reads process settings into a typed, validated struct.
//
// @file      internal/config/oauth_clients.go
// @for       The OAuth client material a deployment supplies for a provider's authorization round.
// @uses      strings, config.getenv.
// @reason    The reference publishes installed-app client secrets inside its registry document, and this repository commits generated registry data, so a secret read from the document would be a secret in git. Reading it per provider here keeps one source of truth for each half: the client id comes from the registry unless the operator registered their own, and the secret exists only in the process environment.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-10-17
package config

import "strings"

// OAuthClient is one provider's OAuth client material. Both halves are
// optional: an empty value means "the registry entry decides".
type OAuthClient struct {
	ClientID     string
	ClientSecret string
}

func (c OAuthClient) isEmpty() bool { return c.ClientID == "" && c.ClientSecret == "" }

// providerOAuthClients names the flows whose client may belong to the
// deployment. The Google family is first because its rounds are started by the
// shared client against a client id the registry already carries.
var providerOAuthClients = []struct {
	provider  string
	idEnv     string
	secretEnv string
}{
	{provider: "gemini-cli", idEnv: "GEMINI_CLI_OAUTH_CLIENT_ID", secretEnv: "GEMINI_CLI_OAUTH_CLIENT_SECRET"},
	{provider: "antigravity", idEnv: "ANTIGRAVITY_OAUTH_CLIENT_ID", secretEnv: "ANTIGRAVITY_OAUTH_CLIENT_SECRET"},
}

// loadOAuthClients reads the per-provider client material. A provider that
// states nothing is left out of the map rather than entered empty, so a reader
// can tell "not configured" from "configured with an empty secret".
func loadOAuthClients() map[string]OAuthClient {
	clients := make(map[string]OAuthClient, len(providerOAuthClients))
	for _, entry := range providerOAuthClients {
		client := OAuthClient{
			ClientID:     strings.TrimSpace(getenv(entry.idEnv, "")),
			ClientSecret: strings.TrimSpace(getenv(entry.secretEnv, "")),
		}
		if client.isEmpty() {
			continue
		}
		clients[entry.provider] = client
	}
	return clients
}
