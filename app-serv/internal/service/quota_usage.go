// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/quota_usage.go
// @for       The published-quota read for one connection: what the provider itself
//
//	says it has left, as opposed to what this gateway counted.
//
// @uses      context, errors, internal/domain, internal/registry,
//
//	internal/service/quotafetch, strings, time.
//
// @reason    SPEC-API-001 §7.12 windows are counted here from routed traffic, and
//
//	`quotafetch` has read providers' published allocations all along with
//	no caller. The two answers disagree by nature — a gateway counter
//	knows what it sent, a provider knows what it sold — so they are kept
//	apart rather than merged into one row: `quota_windows` names a fixed
//	set of window kinds and no unit, and a credit balance forced into it
//	would be rendered as a percentage of something it is not.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-28
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/quotafetch"
)

// PublishedQuotaFetcher reads one provider family's published allocation. It is a
// field rather than a direct call so the routing rules below can be tested without a
// network, while production uses the real `quotafetch.Fetch`.
type PublishedQuotaFetcher func(ctx context.Context, family string, creds quotafetch.Credentials) quotafetch.Result

// PublishedWindow is one bucket the provider published. `Total` is absent rather than
// zero when the provider states a usage with no allocation behind it, because the
// panel's card has to tell "unlimited" from "spent".
type PublishedWindow struct {
	Label    string
	Used     float64
	Total    float64
	HasTotal bool
	ResetsAt *time.Time
}

// PublishedUsage is the answer for one connection: the provider's own words, a soft
// message when it has none, and the instant the read happened.
type PublishedUsage struct {
	EndpointID string
	ProviderID string
	Plan       string
	Windows    []PublishedWindow
	Message    string
	FetchedAt  time.Time
}

// PublishedUsage reads what the provider of one connection publishes about its own
// allocation. A provider that declares no usage endpoint, or an account with no
// credential to ask with, is a refusal the operator can act on; a provider that
// answers with an error or nothing at all is a soft message, because the reference
// renders that sentence on the card rather than failing the page.
func (s *QuotaService) PublishedUsage(ctx context.Context, endpointID string) (PublishedUsage, error) {
	if s.usageFetch == nil || s.providers == nil || s.sealer == nil {
		return PublishedUsage{}, domain.NewInternalError("published quota is not wired")
	}
	id := strings.TrimSpace(endpointID)
	if id == "" {
		return PublishedUsage{}, domain.NewValidationError("endpoint_id is required")
	}

	endpoint, err := s.endpoints.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrEndpointNotFound) {
			return PublishedUsage{}, domain.ErrEndpointNotFound
		}
		return PublishedUsage{}, err
	}
	entry, known := s.providers.Provider(endpoint.ProviderID())
	if !known {
		return PublishedUsage{}, domain.NewValidationError("unknown provider_id: " + endpoint.ProviderID())
	}
	if !entry.Features.Usage {
		return PublishedUsage{}, domain.NewValidationError(
			"provider " + entry.ID + " publishes no usage endpoint for a connection")
	}

	credentials, err := s.publishedCredential(endpoint, entry)
	if err != nil {
		return PublishedUsage{}, err
	}

	result := s.usageFetch(ctx, endpoint.ProviderID(), credentials)
	return PublishedUsage{
		EndpointID: id, ProviderID: endpoint.ProviderID(),
		Plan: strings.TrimSpace(result.Plan), Windows: toPublishedWindows(result.Quotas),
		Message: strings.TrimSpace(result.Message), FetchedAt: s.clock(),
	}, nil
}

// publishedCredential opens the one secret the provider's usage endpoint reads and
// fills only the field that secret belongs to: `quotafetch` reads whichever it is
// given, and a device token handed over as a key would be asked of the provider in
// the wrong shape. The endpoint's own auth type decides, exactly as routing decides
// it — an account that authenticated by flow presents its access token, a key
// account presents its next usable key.
func (s *QuotaService) publishedCredential(endpoint domain.UpstreamEndpoint, entry registry.Provider) (quotafetch.Credentials, error) {
	switch endpoint.AuthType() {
	case domain.UpstreamAuthOAuth:
		credential := endpoint.OAuth()
		if credential == nil || strings.TrimSpace(credential.AccessTokenEncrypted) == "" {
			return quotafetch.Credentials{}, domain.NewValidationError("the account has no stored access token to ask with")
		}
		token, err := s.sealer.Open(credential.AccessTokenEncrypted)
		if err != nil {
			return quotafetch.Credentials{}, err
		}
		// The opened value is checked too: an account can hold a sealed empty
		// token, and asking the provider with no bearer costs a call that comes
		// back as its 401 rather than as this fact about the row.
		if strings.TrimSpace(token) == "" {
			return quotafetch.Credentials{}, domain.NewValidationError("the account's stored access token is empty")
		}
		return quotafetch.Credentials{
			AccessToken: token, UsageURL: entry.Transport.Usage.URL, UsageHeaders: entry.Transport.Headers,
		}, nil
	case domain.UpstreamAuthNone:
		return quotafetch.Credentials{}, domain.NewValidationError("the account presents no credential to ask with")
	default:
		// A provider whose usage endpoint reads an account token cannot be asked
		// with a key, and saying so here costs nothing: the alternative is an
		// outbound call the provider answers 401 for, rendered as its error
		// rather than as this fact about the registry.
		if !entry.Features.UsageAPIKey {
			return quotafetch.Credentials{}, domain.NewValidationError(
				"provider " + entry.ID + " reads usage with an account token, not a key")
		}
		key, ok := endpoint.NextKey(s.clock())
		if !ok {
			return quotafetch.Credentials{}, domain.NewValidationError("the account has no usable key to ask with")
		}
		value, err := s.sealer.Open(key.EncryptedValue())
		if err != nil {
			return quotafetch.Credentials{}, err
		}
		return quotafetch.Credentials{
			APIKey: value, UsageURL: entry.Transport.Usage.URL, UsageHeaders: entry.Transport.Headers,
		}, nil
	}
}

func toPublishedWindows(quotas []quotafetch.Quota) []PublishedWindow {
	windows := make([]PublishedWindow, 0, len(quotas))
	for _, quota := range quotas {
		window := PublishedWindow{
			Label: strings.TrimSpace(quota.Label),
			Used:  quota.Used,
		}
		// An unlimited bucket states no ceiling even when it states a zero total:
		// the card must show the amount spent with no bar, not a bar of nothing.
		if quota.Total > 0 && !quota.Unlimited {
			window.Total, window.HasTotal = quota.Total, true
		}
		if !quota.ResetAt.IsZero() {
			resets := quota.ResetAt
			window.ResetsAt = &resets
		}
		windows = append(windows, window)
	}
	return windows
}
