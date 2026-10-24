// Package config reads process settings into a typed, validated struct.
//
// @file      internal/config/oauth_clients.go
// @for       The OAuth client material a deployment supplies for a provider's authorization round.
// @uses      strings, config.getenv.
// @reason    The reference publishes installed-app client material inside its registry document, and this repository commits generated registry data, so either half read from that document would be a credential in git history. The secret therefore exists only in the process environment, and for the two Google families so does the id: a provider that states nothing here is refused before its round starts rather than sent with a blank client.
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

// providerOAuthClients names the flows whose client belongs to the deployment.
// The two Google installed-app families are the ones that need it: their client
// material lives in the reference's registry document, and none of it is copied
// into a committed file here. A provider absent from this table has no env slot,
// so its flow can only start on a client id the entry itself carries.
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
