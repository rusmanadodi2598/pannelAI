// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_client.go
// @for       The OAuth client a provider's round is performed with, resolving typed config over the registry entry.
// @uses      internal/registry, strings.
// @reason    The registry entry is the only place a client id is declared, and its secret deliberately never appears in that committed document. One resolution rule keeps the id, the exchange, and every later refresh on the same client: read the deployment's value first, fall back to what the entry says.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-17
package service

import (
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// OAuthClient is one provider's client material as the deployment supplied it.
// An empty half means "the registry entry decides that half".
type OAuthClient struct {
	ClientID     string
	ClientSecret string
}

// clientFor resolves the pair a round is performed with: the deployment's value
// when it states one, the registry entry otherwise, so an entry that declares a
// secret is not exchanged without it.
func (s *OAuthFlowService) clientFor(providerID string, oauth *registry.OAuth) (clientID, clientSecret string) {
	clientID, clientSecret = oauth.ClientID, oauth.ClientSecret
	if configured, ok := s.providerClients[providerID]; ok {
		if configured.ClientID != "" {
			clientID = configured.ClientID
		}
		if configured.ClientSecret != "" {
			clientSecret = configured.ClientSecret
		}
	}
	return strings.TrimSpace(clientID), strings.TrimSpace(clientSecret)
}
