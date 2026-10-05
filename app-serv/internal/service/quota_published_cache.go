// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/quota_published_cache.go
// @for       The cached published-quota reads: the collection page's one batched query, and the per-endpoint cache-first read.
// @uses      context, internal/domain, internal/repository, sort, time.
// @reason    The quota screen must show what each provider publishes about itself, and
//
//	asking providers during that read would spend one outbound call per account
//	, the N+1 shape AGENTS.md §1.7 blocks on this route, at the owner's
//	standing instruction. So a worker writes provider answers into a cache and
//	every read here takes them from it, in one statement for the whole page.
//	A card can still ask for a live number when the operator wants the instant
//	rather than the last poll, which is the `force` path below.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// PagePublished attaches the cached provider answer to the accounts one page
// names. The account list, not the counted windows, decides who gets an entry: an
// endpoint that has routed nothing yet has no window row, so a card list built
// from windows alone hides a provider the worker did answer for. Every account on
// the page is answered, and one the worker never reached carries `NeverPolled` so
// the card says that plainly rather than look empty. It stays one batched read for
// the page, so forty accounts cost one statement. A read failure is returned, not
// swallowed; the handler keeps the counted windows, as the gateway's own numbers
// are still true and losing them over a cache read is the worse answer.
func (s *QuotaService) PagePublished(ctx context.Context, accounts []domain.QuotaAccount) ([]PublishedUsage, error) {
	if s.publishedCache == nil || len(accounts) == 0 {
		return nil, nil
	}

	// One entry per account, in first-seen order: a page may name the same endpoint
	// through several rows, and a card that repeats is the same bug as a card missing.
	//
	// An account whose provider publishes no quota at all is left out rather than
	// answered with "not polled yet". That sentence promises a poll that will never
	// come, and it is the difference between a queue and a capability the provider
	// never had. The credential-free virtual lane is left out for the same reason;
	// the card it belongs to already says its counts are local.
	unique := make([]domain.QuotaAccount, 0, len(accounts))
	seen := make(map[string]struct{}, len(accounts))
	for _, account := range accounts {
		if _, duplicate := seen[account.EndpointID]; duplicate {
			continue
		}
		if !s.publishesQuota(account.ProviderID) {
			continue
		}
		seen[account.EndpointID] = struct{}{}
		unique = append(unique, account)
	}
	ids := make([]string, 0, len(unique))
	for _, account := range unique {
		ids = append(ids, account.EndpointID)
	}

	cached, err := s.publishedCache.ListPublishedByEndpointIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("reading cached published quota: %w", err)
	}

	out := make([]PublishedUsage, 0, len(unique))
	for _, account := range unique {
		quota, ok := cached[account.EndpointID]
		if !ok {
			// No row means the worker has never answered for this account. That is a
			// different claim from a provider that answered with nothing to report,
			// which arrives as a `message`, and collapsing the two would tell the
			// operator their provider was silent when in fact nobody asked.
			out = append(out, PublishedUsage{
				EndpointID: account.EndpointID, ProviderID: account.ProviderID,
				Windows: []PublishedWindow{}, NeverPolled: true,
			})
			continue
		}
		out = append(out, publishedFromCache(account.EndpointID, quota))
	}
	return out, nil
}

// publishesQuota reports whether a provider has a usage endpoint to ask at all. A
// provider index that is not wired answers yes for everything, because the older
// wiring has no index to consult and the live read path keeps its own gate.
func (s *QuotaService) publishesQuota(providerID string) bool {
	if s.providers == nil {
		return true
	}
	entry, known := s.providers.Provider(providerID)
	return known && entry.Features.Usage
}

// CachedPublished returns one endpoint's stored provider answer, or false when the
// worker has never answered for it.
func (s *QuotaService) CachedPublished(ctx context.Context, endpointID string) (PublishedUsage, bool, error) {
	if s.publishedCache == nil {
		return PublishedUsage{}, false, nil
	}
	cached, err := s.publishedCache.ListPublishedByEndpointIDs(ctx, []string{endpointID})
	if err != nil {
		return PublishedUsage{}, false, fmt.Errorf("reading cached published quota: %w", err)
	}
	quota, ok := cached[endpointID]
	if !ok {
		return PublishedUsage{}, false, nil
	}
	return publishedFromCache(endpointID, quota), true, nil
}

// publishedFromCache lowers one stored answer into the read the wire serves. The
// amounts are decimal strings in the cache because a credit balance has no integer
// spelling (§4), and the live fetcher reports float64, so both are rendered through
// the same PublishedWindow the handler already knows.
func publishedFromCache(endpointID string, quota domain.PublishedQuota) PublishedUsage {
	usage := PublishedUsage{
		EndpointID:    endpointID,
		ProviderID:    quota.State.ProviderID,
		Plan:          quota.State.Plan,
		Message:       quota.State.Message,
		Windows:       make([]PublishedWindow, 0, len(quota.Windows)),
		Cached:        true,
		FailuresRun:   quota.State.ConsecutiveFailures,
		LastAttemptAt: quota.State.LastAttemptAt,
	}
	if quota.State.FetchedAt != nil {
		usage.FetchedAt = *quota.State.FetchedAt
	}

	for _, row := range quota.Windows {
		window := PublishedWindow{
			Label:           row.Label,
			Used:            publishedAmount(row.Used),
			Unit:            row.Unit,
			Unlimited:       row.Unlimited,
			IsCreditBalance: row.IsCreditBalance,
			Recurring:       row.Recurring,
		}
		// A stored NULL total is the provider saying it publishes no ceiling, which
		// is not a ceiling of zero; the card draws no bar for it.
		if total, ok := row.Ceiling(); ok {
			window.Total, window.HasTotal = publishedAmount(total), true
		}
		if row.ResetsAt != nil {
			resets := *row.ResetsAt
			window.ResetsAt = &resets
		}
		usage.Windows = append(usage.Windows, window)
	}
	sortPublishedWindows(usage.Windows)
	return usage
}

// sortPublishedWindows orders a cached card's rows by the instant they refill, so the
// bucket that closes first reads first. Stored rows arrive in whatever order the
// database's index yields, and a card that reshuffled between two identical polls
// teaches the operator nothing.
func sortPublishedWindows(windows []PublishedWindow) {
	sort.SliceStable(windows, func(i, j int) bool {
		return publishedBefore(windows[i], windows[j])
	})
}

// publishedBefore orders a window with no reset last, because a bucket that never
// refills is not the one the operator is watching the clock on.
func publishedBefore(left, right PublishedWindow) bool {
	if left.ResetsAt == nil {
		return false
	}
	if right.ResetsAt == nil {
		return true
	}
	return left.ResetsAt.Before(*right.ResetsAt)
}

// publishedAmount reads a stored decimal back as the number the bar is drawn from.
//
// An unparseable amount answers zero, which is the honest limit of what a
// display-only bar can do: the row is corrupt, the ceiling it belongs to is
// unknown, and drawing no bar is the same visual answer as drawing an empty one.
// The value never reaches accounting, which reads the stored strings directly.
func publishedAmount(raw string) float64 {
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return parsed
}

// publishedAnswer is the fetcher's result for one endpoint in the shape the cache
// stores, so the worker and any future writer lower the same fields the read
// raises. The amounts cross that boundary as decimal strings for §4's reason.
func publishedAnswer(endpointID, providerID string, result PublishedUsage) domain.PublishedAnswer {
	rows := make([]domain.PublishedWindowRow, 0, len(result.Windows))
	for _, window := range result.Windows {
		row := domain.PublishedWindowRow{
			EndpointID:      endpointID,
			Label:           window.Label,
			Used:            strconv.FormatFloat(window.Used, 'f', -1, 64),
			Unlimited:       window.Unlimited,
			IsCreditBalance: window.IsCreditBalance,
			Recurring:       window.Recurring,
			Unit:            window.Unit,
			ResetsAt:        window.ResetsAt,
			FetchedAt:       result.FetchedAt,
		}
		if window.HasTotal {
			total := strconv.FormatFloat(window.Total, 'f', -1, 64)
			row.Total = &total
		}
		rows = append(rows, row)
	}
	return domain.PublishedAnswer{
		EndpointID: endpointID,
		ProviderID: providerID,
		Plan:       result.Plan,
		Message:    result.Message,
		FetchedAt:  result.FetchedAt,
		Windows:    rows,
	}
}
