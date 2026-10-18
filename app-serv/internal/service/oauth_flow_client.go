// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_client.go
// @for       The OAuth client a provider's round is performed with, resolved from typed config and refused when the deployment states none.
// @uses      internal/domain, internal/registry, strings.
// @reason    No half of a client material lives in the committed registry document any more, because the reference ships live Google credentials inside it and a credential has no business in git history. One resolution rule keeps the id, the exchange, and every later refresh on the same client, and one refusal replaces what used to be a round that started with an empty client id and failed at the provider.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-17
package service

import (
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// OAuthClient is one provider's client material as the deployment supplied it.
// An empty half means "the registry entry decides that half".
type OAuthClient struct {
	ClientID     string
	ClientSecret string
}

// clientFor resolves the pair a round is performed with. The id may still come from
// the registry entry, where a provider that ships a public client id declares it,
// and the deployment overrides it when it registered its own; the secret has one
// source, because the registry type carries no such member.
//
// A round with no client id is refused rather than sent: an authorize URL carrying
// an empty client_id fails at the provider, after the browser already left.
func (s *OAuthFlowService) clientFor(providerID string, oauth *registry.OAuth) (string, string, error) {
	configured := s.providerClients[providerID]
	clientID := strings.TrimSpace(configured.ClientID)
	if clientID == "" {
		clientID = strings.TrimSpace(oauth.ClientID)
	}
	clientSecret := strings.TrimSpace(configured.ClientSecret)
	if clientID == "" {
		return "", "", domain.NewValidationError(
			"the OAuth client for provider " + providerID + " is not configured on this deployment")
	}
	return clientID, clientSecret, nil
}
