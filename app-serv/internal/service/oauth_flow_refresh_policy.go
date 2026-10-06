// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_refresh_policy.go
// @for       The rules a refresh and a connect decision are made of: which flow the panel is offered, how a token is renewed, and the small accessors the refresh state is read through.
// @uses      context, strings, time, internal/domain, internal/registry.
// @reason    These are the judgements, separated from the two routes that act on them, so "what does this provider's oauth block mean" is answered in one place: `flowKind` for the panel's offer, `refreshGrant` for the shape a renewal takes. Grouping them here also keeps oauth_flow_refresh.go inside the AGENTS.md §1.1 budget that its own Status/Refresh routes would otherwise push past.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-29
package service

import (
	"context"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/oauthhttp"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// flowKind names the flow the panel should offer for a provider's oauth block.
func flowKind(oauth *registry.OAuth) string {
	switch {
	case oauth == nil:
		return "none"
	case oauth.RequiresCustomExchange():
		return "connector"
	case oauth.AuthorizeURL == "":
		return "device"
	default:
		return "code"
	}
}

// refreshGrant renews one credential through the shape its provider declared.
//
// A state round carries its refresh token in a header against a separate refresh
// endpoint and posts an empty body, which no encoding of the standard grant
// reaches; every other provider answers the ordinary form or JSON grant the
// reference sends it.
func (s *OAuthFlowService) refreshGrant(ctx context.Context, oauth *registry.OAuth, refreshToken string) (oauthhttp.TokenResponse, error) {
	if oauth.StateExchangeFlow() {
		client, ok := s.tokens.(oauthhttp.StateRoundClient)
		if !ok {
			return oauthhttp.TokenResponse{}, domain.NewInternalError("the state refresh needs a token client that speaks it")
		}
		return client.StateRefresh(ctx, oauth, refreshToken)
	}
	grant := oauthhttp.TokenGrant{
		GrantType:    "refresh_token",
		RefreshToken: refreshToken,
		ClientID:     oauth.ClientID,
		ClientSecret: oauth.ClientSecret,
	}
	if oauth.Refresh != nil {
		grant.Scope = oauth.Refresh.Scope
	}
	return s.tokens.Grant(ctx, oauth.TokenURL, tokenEncoding(oauth), grant)
}

// refreshLead converts the registry's millisecond lead to a duration.
func refreshLead(oauth *registry.OAuth) time.Duration {
	if oauth == nil {
		return 0
	}
	return time.Duration(oauth.RefreshLeadMS) * time.Millisecond
}

// tokenEncoding selects the token endpoint's body encoding: JSON when the
// provider's refresh block declares it, form otherwise (§8.1).
func tokenEncoding(oauth *registry.OAuth) string {
	if oauth != nil && oauth.Refresh != nil && strings.EqualFold(oauth.Refresh.Encoding, "json") {
		return "json"
	}
	return ""
}

// credentialExpiry and lastRefresh keep nil-safety in one place.
func credentialExpiry(credential *domain.OAuthCredential) *time.Time {
	if credential == nil {
		return nil
	}
	return credential.ExpiresAt()
}

func lastRefresh(credential *domain.OAuthCredential) *time.Time {
	if credential == nil {
		return nil
	}
	return credential.LastRefreshAt()
}
