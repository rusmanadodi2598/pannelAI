// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_refresh.go
// @for       Reporting per-endpoint token state and refreshing tokens, forced or due (SPEC-API-001 §7.4 GET .../oauth/status, POST .../oauth/refresh).
// @uses      context, strings, time, internal/domain, internal/registry.
// @reason    Status is the panel's view of credential health and refresh is the operator's manual override of the worker; both derive "due" from the same domain rule so they can never disagree, and both write through the aggregate so a rotated token set replaces the sealed pair atomically.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-19
package service

import (
	"context"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

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

// OAuthRefreshSkipped is one account the bulk sweep passed over, named with the
// reason it stopped there so the operator can act on that account alone.
type OAuthRefreshSkipped struct {
	EndpointID string
	Reason     string
}

// OAuthRefreshOutcome reports what a forced refresh did.
type OAuthRefreshOutcome struct {
	Refreshed   int
	EndpointIDs []string
	ExpiresAt   *time.Time
	// Skipped is only ever filled by the sweep. Refreshing one named account reports
	// its failure as an error instead, which is the answer a forced action needs.
	Skipped []OAuthRefreshSkipped
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

// Refresh forces one endpoint's token refresh, or every due endpoint of the provider
// when no endpoint id is given. Naming one account fails fast on its refusal, because
// the concrete reason is the answer that action was asked for; the sweep of a whole
// provider finishes and names what it passed over, because its work is already
// committed one account at a time.
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

	return s.sweepProvider(ctx, entry)
}

// refreshEndpoint opens the stored refresh token, performs the refresh grant,
// and stores the resealed token set. A response refresh token replaces the
// stored one (rotation); an absent one keeps it, per the grant's own rule.
func (s *OAuthFlowService) refreshEndpoint(ctx context.Context, oauth *registry.OAuth, endpoint domain.UpstreamEndpoint) (*time.Time, error) {
	credential := endpoint.OAuth()
	if credential == nil || strings.TrimSpace(credential.RefreshTokenEncrypted()) == "" {
		return nil, domain.NewValidationError("the account has no refresh token")
	}
	opened, err := s.sealer.Open(credential.RefreshTokenEncrypted())
	if err != nil {
		return nil, err
	}
	token, err := s.refreshGrant(ctx, oauth, opened)
	if err != nil {
		return nil, err
	}

	now := s.clock()
	// The credential as the row holds it before anything is written: the persist
	// below is conditioned on it, so a refresh that raced another one loses cleanly
	// instead of writing the older credential over the token the vendor already
	// rotated.
	loaded := *credential
	accessSealed, err := s.sealer.Seal(token.AccessToken)
	if err != nil {
		return nil, domain.NewInternalError("the access token could not be stored")
	}
	// A blank here is the vendor's answer, not ours: Rotated keeps the refresh
	// ciphertext the account already holds when the grant issued no new one.
	refreshSealed := ""
	if refresh := strings.TrimSpace(token.RefreshToken); refresh != "" {
		if refreshSealed, err = s.sealer.Seal(refresh); err != nil {
			return nil, domain.NewInternalError("the refresh token could not be stored")
		}
	}
	var expiresAt *time.Time
	if token.ExpiresIn > 0 {
		expires := now.Add(time.Duration(token.ExpiresIn) * time.Second)
		expiresAt = &expires
	}
	next, err := credential.Rotated(accessSealed, refreshSealed, expiresAt, now)
	if err != nil {
		return nil, err
	}

	endpoint.SetOAuth(&next, now)
	if err := s.store.UpdateIfUnchanged(ctx, endpoint, loaded); err != nil {
		return nil, err
	}
	return next.ExpiresAt(), nil
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
