// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_connect.go
// @for       Landing a provider token set on an account: sealing, the identity match, and the update-or-create write both connect paths share.
// @uses      context, errors, fmt, strings, time, internal/domain, internal/repository.
// @reason    A code callback and a device poll produce the same thing, one account's credentials, and the rule that keeps them agreeing is non-obvious: match on email and workspace, seal both tokens, name a fresh endpoint after its identity. Written twice those rules drift apart, so this file owns them once.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-27
package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/oauthhttp"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// connectTokens is the credential material an account is connected with, in
// whichever shape the flow that produced it reported: a code exchange answers a
// relative expires_in, a device poll an absolute instant.
type connectTokens struct {
	AccessToken  string
	RefreshToken string
	Scopes       []string
	ExpiresAt    *time.Time
}

// connectAccount writes one account's tokens: seal, find the endpoint its
// identity already owns, and update that endpoint or create a new one. Every
// connect path lands here, so an account cannot split in two under one
// provider because one path matched and another did not.
func (s *OAuthFlowService) connectAccount(ctx context.Context, providerID string, tokens connectTokens, account domain.EndpointAccount, origin string, now time.Time) (OAuthConnect, error) {
	credential, err := sealConnectTokens(s.sealer, tokens, account, now)
	if err != nil {
		return OAuthConnect{}, err
	}
	hint := domain.MaskSecret(tokens.AccessToken)

	existingID, err := s.store.FindOAuthEndpoint(ctx, providerID, account.Email().String(), account.WorkspaceID())
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
		return OAuthConnect{Endpoint: endpoint, Created: false, TokenHint: hint, RedirectBase: origin}, nil
	}

	label, err := s.accountLabel(ctx, providerID, account)
	if err != nil {
		return OAuthConnect{}, err
	}

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
	return OAuthConnect{Endpoint: endpoint, Created: true, TokenHint: hint, RedirectBase: origin}, nil
}

// accountLabel names a freshly created OAuth account. A vendor that stated an
// identity gets that identity's own name; one that stated nothing, a state round
// returning only tokens (how CodeBuddy answers), gets the smallest `Account N`
// the provider does not already use: the store enforces UNIQUE (provider_id,
// label), so a taken name turns a granted login into an insert error. Names are
// read, never counted from the row total, because deleting `Account 1` would make
// the next login recompute a name a surviving account already holds. The scan
// window is dataplane.MaxEndpointsPerProvider, so a free name always exists in it.
func (s *OAuthFlowService) accountLabel(ctx context.Context, providerID string, account domain.EndpointAccount) (string, error) {
	if account.HasIdentity() {
		return defaultOAuthLabel(account), nil
	}
	endpoints, _, err := s.store.List(ctx, repository.EndpointFilter{ProviderID: providerID},
		repository.PageQuery{Page: 1, PerPage: accountLabelScanLimit})
	if err != nil {
		return "", err
	}
	taken := make(map[string]bool, len(endpoints))
	for _, endpoint := range endpoints {
		taken[endpoint.Label()] = true
	}
	for candidate := 1; candidate <= accountLabelScanLimit+1; candidate++ {
		name := fmt.Sprintf("Account %d", candidate)
		if !taken[name] {
			return name, nil
		}
	}
	return "", domain.NewInternalError("no free account label is left for this provider")
}

// accountLabelScanLimit is how many of a provider's rows are read to decide which
// `Account N` names are already used.
const accountLabelScanLimit = 100

// sealConnectTokens seals both tokens and stamps the expiry and refresh instant,
// so the aggregate receives ciphertext only (SPEC-API-001 §6). A nil ExpiresAt
// leaves the lifetime unknown rather than inventing one.
func sealConnectTokens(sealer SecretSealer, tokens connectTokens, account domain.EndpointAccount, now time.Time) (*domain.OAuthCredential, error) {
	accessSealed, err := sealer.Seal(strings.TrimSpace(tokens.AccessToken))
	if err != nil {
		return nil, domain.NewInternalError("the access token could not be stored")
	}
	refreshSealed := ""
	if refresh := strings.TrimSpace(tokens.RefreshToken); refresh != "" {
		if refreshSealed, err = sealer.Seal(refresh); err != nil {
			return nil, domain.NewInternalError("the refresh token could not be stored")
		}
	}
	return domain.NewOAuthCredential(domain.OAuthCredentialInput{
		AccessTokenEncrypted:  accessSealed,
		RefreshTokenEncrypted: refreshSealed,
		ExpiresAt:             tokens.ExpiresAt,
		Scopes:                tokens.Scopes,
		AccountEmail:          account.Email().String(),
		AccountID:             account.WorkspaceID(),
		LastRefreshAt:         &now,
	})
}

// tokenExpiry turns a grant's relative lifetime into an absolute instant. An
// expires_in of zero or less means the upstream said nothing about expiry.
func tokenExpiry(token oauthhttp.TokenResponse, now time.Time) *time.Time {
	if token.ExpiresIn <= 0 {
		return nil
	}
	expires := now.Add(time.Duration(token.ExpiresIn) * time.Second)
	return &expires
}
