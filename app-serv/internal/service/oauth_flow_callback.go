// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_callback.go
// @for       Completing a provider authorization: replay-guarded state
//
//	consumption, code exchange, identity matching, and sealing
//	(SPEC-API-001 §7.4 GET .../oauth/callback).
//
// @uses      encoding/json, errors, net/url, strings, time, internal/domain,
//
//	internal/registry.
//
// @reason    The callback is the only place provider tokens enter the
//
//	system, so it is where the §4 single-use rule is enforced,
//	where a declared userinfo endpoint is consulted fail-closed
//	(so one account can never split in two), and where the
//	aggregate receives ciphertext only.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// oauthStatePayload is the private context staged with a state: which provider
// the flow belongs to, the PKCE verifier only the callback may spend, the
// redirect the code was issued against, and the origin a browser redirect
// should return to.
type oauthStatePayload struct {
	ProviderID  string `json:"provider_id"`
	Verifier    string `json:"code_verifier,omitempty"`
	RedirectURI string `json:"redirect_uri"`
	Origin      string `json:"origin,omitempty"`
}

// OAuthCallbackInput is one callback: the provider from the path, the code and
// state from the query, and the error pair a refusal carries instead of a code.
type OAuthCallbackInput struct {
	ProviderID       string
	Code             string
	State            string
	Error            string
	ErrorDescription string
}

// OAuthConnect is the outcome: the endpoint standing for the account, whether
// the flow created it or updated a known one, the token hint, and the origin a
// browser should land on.
type OAuthConnect struct {
	Endpoint     domain.UpstreamEndpoint
	Created      bool
	TokenHint    string
	RedirectBase string
}

// requireCodeFlow resolves a provider and refuses every entry the shared
// client cannot serve: no oauth block, a connector-required exchange, or a
// device flow with no browser authorize endpoint. Both Start and Callback gate
// through it so the two ends of one flow can never disagree about eligibility.
func (s *OAuthFlowService) requireCodeFlow(providerID string) (registry.Provider, *registry.OAuth, error) {
	name := strings.TrimSpace(providerID)
	entry, ok := s.index.Provider(name)
	if !ok {
		return registry.Provider{}, nil, domain.NewValidationError("unknown provider_id: " + name)
	}
	if entry.OAuth == nil {
		return registry.Provider{}, nil, domain.NewValidationError("provider " + name + " does not declare an oauth flow")
	}
	if entry.OAuth.RequiresCustomExchange() {
		return registry.Provider{}, nil, domain.NewValidationError("provider " + name + " needs a connector for its token exchange")
	}
	if entry.OAuth.AuthorizeURL == "" {
		return registry.Provider{}, nil, domain.NewValidationError("provider " + name + " uses a device authorization flow; connect it through its device endpoint instead")
	}
	return entry, entry.OAuth, nil
}

// callbackURLFor renders the gateway's own callback route under a base URL.
func callbackURLFor(base, providerID string) string {
	return base + "/api/v1/providers/" + providerID + "/oauth/callback"
}

// originFor picks the origin a finished flow should send a browser back to:
// the declared base URL when one exists, otherwise the redirect's own origin.
func originFor(base, redirect string) string {
	if trimmed := strings.TrimSuffix(strings.TrimSpace(base), "/"); trimmed != "" {
		return trimmed
	}
	if parsed, err := url.Parse(redirect); err == nil && parsed.Host != "" {
		return parsed.Scheme + "://" + parsed.Host
	}
	return ""
}

// Callback consumes the staged state exactly once, exchanges the code, reads
// the account identity when the provider declares a userinfo endpoint, and
// stores the sealed token set on the account's endpoint.
func (s *OAuthFlowService) Callback(ctx context.Context, in OAuthCallbackInput) (OAuthConnect, error) {
	_, oauth, err := s.requireCodeFlow(in.ProviderID)
	if err != nil {
		return OAuthConnect{}, err
	}
	providerID := strings.TrimSpace(in.ProviderID)
	if in.Error != "" {
		refusal := in.Error
		if in.ErrorDescription != "" {
			refusal = refusal + ": " + in.ErrorDescription
		}
		return OAuthConnect{}, domain.NewUpstreamError("the provider refused the authorization: " + refusal)
	}
	code := strings.TrimSpace(in.Code)
	if code == "" {
		return OAuthConnect{}, domain.NewValidationError("code is required")
	}
	state := strings.TrimSpace(in.State)
	if state == "" {
		return OAuthConnect{}, domain.NewValidationError("state is required")
	}

	raw, ok, err := s.states.Take(ctx, state)
	if err != nil {
		return OAuthConnect{}, err
	}
	if !ok {
		return OAuthConnect{}, domain.NewValidationError("the state is unknown, expired, or already used")
	}
	var payload oauthStatePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return OAuthConnect{}, domain.NewInternalError("the staged oauth state could not be decoded")
	}
	if payload.ProviderID != providerID {
		return OAuthConnect{}, domain.NewValidationError("the state belongs to another provider")
	}

	token, err := s.tokens.Grant(ctx, oauth.TokenURL, tokenEncoding(oauth), TokenGrant{
		GrantType: "authorization_code", Code: code,
		ClientID: oauth.ClientID, ClientSecret: oauth.ClientSecret,
		RedirectURI: payload.RedirectURI, CodeVerifier: payload.Verifier,
	})
	if err != nil {
		return OAuthConnect{}, err
	}

	account := domain.EndpointAccount{}
	if oauth.UserInfoURL != "" {
		identity, err := s.tokens.UserInfo(ctx, oauth.UserInfoURL, token.AccessToken)
		if err != nil {
			// Fail closed: writing an account without its identity would let
			// the next connect of the same account create a duplicate under a
			// generated label, which is exactly what the identity match exists
			// to prevent.
			return OAuthConnect{}, err
		}
		account = identity.Account()
	}

	now := s.clock()
	credential, err := sealTokenSet(s.sealer, token, oauth.ScopeList(), account, now)
	if err != nil {
		return OAuthConnect{}, err
	}
	hint := domain.MaskSecret(token.AccessToken)

	existingID, err := s.store.FindOAuthEndpoint(ctx, providerID, account.Email, account.WorkspaceID)
	if err != nil && !errors.Is(err, domain.ErrEndpointNotFound) {
		return OAuthConnect{}, err
	}
	if existingID != "" {
		endpoint, err := s.store.GetByID(ctx, existingID)
		if err != nil {
			return OAuthConnect{}, err
		}
		endpoint.SetOAuth(credential, now)
		endpoint.SetAccount(account, now)
		if err := s.store.Update(ctx, endpoint); err != nil {
			return OAuthConnect{}, err
		}
		return OAuthConnect{Endpoint: endpoint, Created: false, TokenHint: hint, RedirectBase: payload.Origin}, nil
	}

	label := connectLabel(providerID, account)
	endpoint, err := domain.NewUpstreamEndpoint(
		domain.IDPrefixUpstreamEndpoint+domain.NewULID(now), providerID, label,
		domain.UpstreamAuthOAuth, 1, now)
	if err != nil {
		return OAuthConnect{}, err
	}
	endpoint.SetOAuth(credential, now)
	endpoint.SetAccount(account, now)
	if err := s.store.Create(ctx, endpoint); err != nil {
		return OAuthConnect{}, err
	}
	return OAuthConnect{Endpoint: endpoint, Created: true, TokenHint: hint, RedirectBase: payload.Origin}, nil
}

// connectLabel names a fresh account after the identity that distinguishes it,
// falling back to the provider plus a fixed marker when the flow learned no
// identity at all (no userinfo endpoint, or one that reports none).
func connectLabel(providerID string, account domain.EndpointAccount) string {
	if account.Email != "" || account.WorkspaceID != "" || account.Name != "" {
		return defaultOAuthLabel(account)
	}
	return providerID + " oauth"
}

// sealTokenSet seals both tokens and stamps the expiry and refresh instant, so
// the aggregate receives ciphertext only (SPEC-API-001 §6). An expires_in of
// zero or less leaves the expiry unknown rather than inventing one.
func sealTokenSet(sealer SecretSealer, token TokenResponse, scopes []string, account domain.EndpointAccount, now time.Time) (*domain.OAuthCredential, error) {
	accessSealed, err := sealer.Seal(token.AccessToken)
	if err != nil {
		return nil, domain.NewInternalError("the access token could not be stored")
	}
	refreshSealed := ""
	if refresh := strings.TrimSpace(token.RefreshToken); refresh != "" {
		if refreshSealed, err = sealer.Seal(refresh); err != nil {
			return nil, domain.NewInternalError("the refresh token could not be stored")
		}
	}
	credential := &domain.OAuthCredential{
		AccessTokenEncrypted:  accessSealed,
		RefreshTokenEncrypted: refreshSealed,
		Scopes:                scopes,
		AccountEmail:          account.Email,
		AccountID:             account.WorkspaceID,
		LastRefreshAt:         &now,
	}
	if token.ExpiresIn > 0 {
		expires := now.Add(time.Duration(token.ExpiresIn) * time.Second)
		credential.ExpiresAt = &expires
	}
	return credential, nil
}
