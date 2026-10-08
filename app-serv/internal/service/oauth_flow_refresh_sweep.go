// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_refresh_sweep.go
// @for       The provider-wide OAuth refresh sweep and the account listing it works over (SPEC-API-001 §7.4 POST .../oauth/refresh with no endpoint).
// @uses      context, internal/domain, internal/registry, internal/repository.
// @reason    Sweeping a provider is a different job from forcing one account: the sweep must finish and name every row it passed over, while a forced single refresh owes the caller the one concrete reason it stopped.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-08
package service

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// oauthListPerPage bounds one page of the endpoint listing behind Status and
// the due sweep; the loop pages until the store's total is covered.
const oauthListPerPage = 100

// sweepProvider refreshes every due endpoint of the provider and names the ones it
// could not. An account that failed is skipped rather than fatal: the accounts that did
// refresh are already committed, and returning the first error would report a batch that
// partly succeeded as though none of it had. Each reason still reaches the operator,
// beside the one account it belongs to.
func (s *OAuthFlowService) sweepProvider(ctx context.Context, entry registry.Provider) (OAuthRefreshOutcome, error) {
	endpoints, err := s.listOAuthEndpoints(ctx, entry.ID)
	if err != nil {
		return OAuthRefreshOutcome{}, err
	}
	now := s.clock()
	outcome := OAuthRefreshOutcome{EndpointIDs: []string{}, Skipped: []OAuthRefreshSkipped{}}
	for _, endpoint := range endpoints {
		if domain.OAuthRefreshState(endpoint.OAuth(), refreshLead(entry.OAuth), now) != domain.RefreshDue {
			continue
		}
		expires, err := s.refreshEndpoint(ctx, entry.OAuth, endpoint)
		if err != nil {
			outcome.Skipped = append(outcome.Skipped, OAuthRefreshSkipped{
				EndpointID: endpoint.ID(),
				// AsAppError is the same sanitiser the response writer applies, so a
				// reason here is English client text and never a wrapped internal error.
				Reason: domain.AsAppError(err).Message,
			})
			continue
		}
		outcome.Refreshed++
		outcome.EndpointIDs = append(outcome.EndpointIDs, endpoint.ID())
		if expires != nil {
			outcome.ExpiresAt = expires
		}
	}
	return outcome, nil
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
