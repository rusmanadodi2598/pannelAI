// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_refresh.go
// @for       Reporting per-endpoint token state and refreshing tokens, forced
//
//	or due (SPEC-API-001 §7.4 GET .../oauth/status, POST
//
//	.../oauth/refresh).
//
// @uses      context, strings, time, internal/domain, internal/registry,
//
//	internal/repository.
//
// @reason    Status is the panel's view of credential health and refresh is
//
//	the operator's manual override of the worker; both derive
//	"due" from the same domain rule so they can never disagree,
//	and both write through the aggregate so a rotated token set
//	replaces the sealed pair atomically.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// oauthListPerPage bounds one page of the endpoint listing behind Status and
// the due sweep; the loop pages until the store's total is covered.
const oauthListPerPage = 100

// OAuthEndpointState is one OAuth endpoint's report: lifecycle status, token
// expiry, last refresh, and the derived refresh state.
type OAuthEndpointState struct {
	EndpointID    string
	Label         string
	Status        string
	ExpiresAt     *time.Time
	LastRefreshAt *time.Time
	RefreshState  string
}

// OAuthStatus answers the status route: the flow kind the provider declares
// and one row per OAuth endpoint of that provider.
type OAuthStatus struct {
	ProviderID string
	Flow       string
	Endpoints  []OAuthEndpointState
}

// OAuthRefreshOutcome reports what a forced refresh did.
type OAuthRefreshOutcome struct {
	Refreshed   int
	EndpointIDs []string
	ExpiresAt   *time.Time
}

// Status reports every OAuth endpoint of the provider with its derived
// refresh state.
func (s *OAuthFlowService) Status(ctx context.Context, providerID string) (OAuthStatus, error) {
	name := strings.TrimSpace(providerID)
	entry, ok := s.index.Provider(name)
	if !ok {
		return OAuthStatus{}, domain.NewValidationError("unknown provider_id: " + name)
	}

	endpoints, err := s.listOAuthEndpoints(ctx, name)
	if err != nil {
		return OAuthStatus{}, err
	}
	now := s.clock()
	states := make([]OAuthEndpointState, 0, len(endpoints))
	for _, endpoint := range endpoints {
		credential := endpoint.OAuth()
		states = append(states, OAuthEndpointState{
			EndpointID:    endpoint.ID(),
			Label:         endpoint.Label(),
			Status:        string(endpoint.Status()),
			ExpiresAt:     credentialExpiry(credential),
			LastRefreshAt: lastRefresh(credential),
			RefreshState:  domain.OAuthRefreshState(credential, refreshLead(entry.OAuth), now),
		})
	}
	return OAuthStatus{ProviderID: name, Flow: flowKind(entry.OAuth), Endpoints: states}, nil
}

// Refresh forces one endpoint's token refresh, or every due endpoint of the
// provider when no endpoint id is given. It fails fast on the first refusal:
// a forced action should surface the concrete reason, not average over it.
func (s *OAuthFlowService) Refresh(ctx context.Context, providerID, endpointID string) (OAuthRefreshOutcome, error) {
	name := strings.TrimSpace(providerID)
	entry, ok := s.index.Provider(name)
	if !ok {
		return OAuthRefreshOutcome{}, domain.NewValidationError("unknown provider_id: " + name)
	}
	if entry.OAuth == nil || entry.OAuth.TokenURL == "" {
		return OAuthRefreshOutcome{}, domain.NewValidationError("provider " + name + " does not declare a refreshable oauth flow")
	}

	if target := strings.TrimSpace(endpointID); target != "" {
		endpoint, err := s.store.GetByID(ctx, target)
		if err != nil {
			return OAuthRefreshOutcome{}, err
		}
		if endpoint.ProviderID() != name {
			return OAuthRefreshOutcome{}, domain.NewNotFoundError("upstream endpoint not found")
		}
		expires, err := s.refreshEndpoint(ctx, entry.OAuth, endpoint)
		if err != nil {
			return OAuthRefreshOutcome{}, err
		}
		return OAuthRefreshOutcome{Refreshed: 1, EndpointIDs: []string{target}, ExpiresAt: expires}, nil
	}

	endpoints, err := s.listOAuthEndpoints(ctx, name)
	if err != nil {
		return OAuthRefreshOutcome{}, err
	}
	now := s.clock()
	outcome := OAuthRefreshOutcome{EndpointIDs: []string{}}
	for _, endpoint := range endpoints {
		if domain.OAuthRefreshState(endpoint.OAuth(), refreshLead(entry.OAuth), now) != domain.RefreshDue {
			continue
		}
		expires, err := s.refreshEndpoint(ctx, entry.OAuth, endpoint)
		if err != nil {
			return OAuthRefreshOutcome{}, err
		}
		outcome.Refreshed++
		outcome.EndpointIDs = append(outcome.EndpointIDs, endpoint.ID())
		if expires != nil {
			outcome.ExpiresAt = expires
		}
	}
	return outcome, nil
}

// refreshEndpoint opens the stored refresh token, performs the refresh grant,
// and stores the resealed token set. A response refresh token replaces the
// stored one (rotation); an absent one keeps it, per the grant's own rule.
func (s *OAuthFlowService) refreshEndpoint(ctx context.Context, oauth *registry.OAuth, endpoint domain.UpstreamEndpoint) (*time.Time, error) {
	credential := endpoint.OAuth()
	if credential == nil || strings.TrimSpace(credential.RefreshTokenEncrypted) == "" {
		return nil, domain.NewValidationError("the account has no refresh token")
	}
	opened, err := s.sealer.Open(credential.RefreshTokenEncrypted)
	if err != nil {
		return nil, err
	}
	grant := TokenGrant{
		GrantType:    "refresh_token",
		RefreshToken: opened,
		ClientID:     oauth.ClientID,
		ClientSecret: oauth.ClientSecret,
	}
	if oauth.Refresh != nil {
		grant.Scope = oauth.Refresh.Scope
	}
	token, err := s.tokens.Grant(ctx, oauth.TokenURL, tokenEncoding(oauth), grant)
	if err != nil {
		return nil, err
	}

	now := s.clock()
	accessSealed, err := s.sealer.Seal(token.AccessToken)
	if err != nil {
		return nil, domain.NewInternalError("the access token could not be stored")
	}
	next := *credential
	next.AccessTokenEncrypted = accessSealed
	if refresh := strings.TrimSpace(token.RefreshToken); refresh != "" {
		refreshSealed, err := s.sealer.Seal(refresh)
		if err != nil {
			return nil, domain.NewInternalError("the refresh token could not be stored")
		}
		next.RefreshTokenEncrypted = refreshSealed
	}
	next.ExpiresAt = nil
	if token.ExpiresIn > 0 {
		expires := now.Add(time.Duration(token.ExpiresIn) * time.Second)
		next.ExpiresAt = &expires
	}
	refreshed := now
	next.LastRefreshAt = &refreshed

	endpoint.SetOAuth(&next, now)
	if err := s.store.Update(ctx, endpoint); err != nil {
		return nil, err
	}
	return next.ExpiresAt, nil
}

// MarkRefreshDeadLetter moves an endpoint to the error state with the reason
// its refresh died, the terminal act of the worker's retry policy.
func (s *OAuthFlowService) MarkRefreshDeadLetter(ctx context.Context, endpointID, message string) error {
	endpoint, err := s.store.GetByID(ctx, endpointID)
	if err != nil {
		return err
	}
	endpoint.MarkUnhealthy(message, s.clock())
	return s.store.Update(ctx, endpoint)
}

// listOAuthEndpoints pages through the provider's endpoints and keeps only the
// OAuth ones, under the AGENTS.md §1.7 bounded-query rule.
func (s *OAuthFlowService) listOAuthEndpoints(ctx context.Context, providerID string) ([]domain.UpstreamEndpoint, error) {
	filter := repository.EndpointFilter{ProviderID: providerID}
	var collected []domain.UpstreamEndpoint
	for page := 1; page <= 50; page++ {
		endpoints, total, err := s.store.List(ctx, filter, repository.PageQuery{Page: page, PerPage: oauthListPerPage})
		if err != nil {
			return nil, err
		}
		for _, endpoint := range endpoints {
			if endpoint.AuthType() == domain.UpstreamAuthOAuth {
				collected = append(collected, endpoint)
			}
		}
		if int64(page*oauthListPerPage) >= total {
			break
		}
	}
	return collected, nil
}

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
	return credential.ExpiresAt
}

func lastRefresh(credential *domain.OAuthCredential) *time.Time {
	if credential == nil {
		return nil
	}
	return credential.LastRefreshAt
}
