// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/quota_usage.go
// @for       The published-quota read for one connection: what the provider itself says it has left, as opposed to what this gateway counted.
// @uses      context, errors, internal/domain, internal/registry, internal/service/quotafetch, strings, time.
// @reason    SPEC-API-001 §7.12 counts windows from routed traffic, and a provider's published allocation is a different answer: this gateway knows what it sent, the provider knows what it sold. They stay in separate rows because `quota_windows` names a fixed set of window kinds and no unit.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
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

// PublishedWindow is one bucket the provider published. `Total` is absent rather
// than zero when a provider states a usage with no allocation behind it, so the
// panel's card can tell "unlimited" from "spent".
//
// `Unit`, `Unlimited` and `IsCreditBalance` carry the three ways a provider's
// number is not a share of a ceiling; flattening them into "used of total" would
// draw a bar over figures that were never shares of anything.
type PublishedWindow struct {
	Label           string
	Used            float64
	Total           float64
	HasTotal        bool
	ResetsAt        *time.Time
	Unit            string
	Unlimited       bool
	IsCreditBalance bool
	Recurring       bool
}

// PublishedUsage is the answer for one connection: the provider's own words, a soft
// message when it has none, and the instant the read happened.
//
// `Cached` says which of the two it is: an answer the poll worker stored earlier, or
// one this process asked the provider for just now. The distinction travels because
// the card prints it, "3000 left" is only a fact as of its stamp, and a screen that
// cannot say whether a number is a poll behind is a screen the operator cannot act on.
type PublishedUsage struct {
	EndpointID string
	ProviderID string
	Plan       string
	Windows    []PublishedWindow
	Message    string
	FetchedAt  time.Time
	Cached     bool
	// Failed says the provider's answer was a refusal the reference would have raised,
	// not a sentence it would have returned. It decides whether the poll counts as a
	// failure, so a family that errors cannot look healthy forever.
	Failed bool

	// FailuresRun is the stored count of consecutive polls that did not produce an
	// answer. It reaches the card because a surviving figure and a failing poll are both
	// true at once, and an operator reading a stale number is entitled to know the last
	// attempt did not replace it.
	FailuresRun int
	// NeverPolled marks an account the poll worker has not answered for yet. It is
	// the absence of an answer, not an answer of absence: the card must not print the
	// provider note or an "asked" stamp for a number nobody has fetched, and it must
	// not imply the provider stays silent about this account.
	NeverPolled bool
	// LastAttemptAt is when the worker last asked, whether or not the answer arrived.
	// It is a different instant from FetchedAt, which dates the figures: a failed poll
	// moves this one and leaves that stamp where the surviving numbers were said.
	LastAttemptAt *time.Time
}

// PublishedUsage reads what the provider of one connection publishes about its own
// allocation, cache-first: the number the poll worker stored is the number this
// route serves, because asking providers inside a screen read is the fan-out this
// route refuses. `force` is the operator's explicit override, one live call for one
// connection, opt-in per press rather than on load, so a screen of a hundred
// accounts costs a provider call only when one is asked for. A provider with no
// usage endpoint, or an account with no credential, is a refusal the operator can
// act on; a provider that errors or answers nothing is a soft message on the card,
// not a failed page.
func (s *QuotaService) PublishedUsage(ctx context.Context, endpointID string, force bool) (PublishedUsage, error) {
	if s.usageFetch == nil || s.providers == nil || s.sealer == nil {
		return PublishedUsage{}, domain.NewInternalError("published quota is not wired")
	}
	id := strings.TrimSpace(endpointID)
	if id == "" {
		return PublishedUsage{}, domain.NewValidationError("endpoint_id is required")
	}

	if !force {
		cached, found, err := s.CachedPublished(ctx, id)
		if err == nil && found {
			return cached, nil
		}
		// A cache read that failed, or that holds nothing for this account yet, is not
		// the operator's problem to see: fall through to the live read and let its own
		// answer stand. An endpoint the worker has never reached is exactly the case
		// this branch is for, and refusing it would hide a number the provider will
		// happily give.
	}

	return s.livePublishedUsage(ctx, id)
}

// livePublishedUsage asks the provider for one connection at this instant. It is what
// `force` reaches for and what the poll worker calls for its own schedule, so the
// refusal rules, unknown provider, no usage endpoint, no credential, are stated once
// and are the same answers the worker sees.
func (s *QuotaService) livePublishedUsage(ctx context.Context, id string) (PublishedUsage, error) {
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
		Failed: result.Failed,
	}, nil
}

// publishedCredential opens the one secret the provider's usage endpoint reads and
// fills only the field that secret belongs to: `quotafetch` reads whichever it is
// given, and a device token handed over as a key would be asked of the provider in
// the wrong shape. The endpoint's own auth type decides, exactly as routing decides
// it, an account that authenticated by flow presents its access token, a key
// account presents its next usable key.
func (s *QuotaService) publishedCredential(endpoint domain.UpstreamEndpoint, entry registry.Provider) (quotafetch.Credentials, error) {
	switch endpoint.AuthType() {
	case domain.UpstreamAuthOAuth:
		credential := endpoint.OAuth()
		if credential == nil || strings.TrimSpace(credential.AccessTokenEncrypted()) == "" {
			return quotafetch.Credentials{}, domain.NewValidationError("the account has no stored access token to ask with")
		}
		token, err := s.sealer.Open(credential.AccessTokenEncrypted())
		if err != nil {
			return quotafetch.Credentials{}, err
		}
		// The opened value is checked too: an account can hold a sealed empty
		// token, and asking the provider with no bearer costs a call that comes
		// back as its 401 rather than as this fact about the row.
		if strings.TrimSpace(token) == "" {
			return quotafetch.Credentials{}, domain.NewValidationError("the account's stored access token is empty")
		}
		credentials := publishedCredentials(entry, endpoint.Account(), credential)
		credentials.AccessToken = token
		return credentials, nil
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
		credentials := publishedCredentials(entry, endpoint.Account(), nil)
		credentials.APIKey = value
		return credentials, nil
	}
}

func toPublishedWindows(quotas []quotafetch.Quota) []PublishedWindow {
	windows := make([]PublishedWindow, 0, len(quotas))
	for _, quota := range quotas {
		window := PublishedWindow{
			Label:           strings.TrimSpace(quota.Label),
			Used:            quota.Used,
			Unit:            strings.TrimSpace(quota.Unit),
			Unlimited:       quota.Unlimited,
			IsCreditBalance: quota.IsCreditBalance,
			Recurring:       quota.Recurring,
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
