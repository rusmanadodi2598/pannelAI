// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/proxy_bulk.go
// @for       The bulk actions over proxy candidates: the operator's switch across a set, and the erase that reports what goes down with it (SPEC-API-001 §7.11).
// @uses      context, fmt, sort, strings, internal/domain, internal/repository.
// @reason    A candidate is one address, so a pool-wide switch is exactly where an operator stops wanting to click each row. Deleting one is the dangerous half: `proxy_pool_id` on an endpoint is an id string and no foreign key refuses the erasure, so the batch that removes a provider's only egress path has to answer with the number of endpoints left dangling and the providers losing their pin, before it removes anything.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-16
package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// ProxyPinReader reports which endpoints route their egress through a pool, which is the
// only way a proxy delete can tell an operator what it takes down.
type ProxyPinReader interface {
	ProxyPins(ctx context.Context, poolIDs []string) (map[string][]repository.ProxyPin, error)
}

// BulkProxySelection names the candidates a proxy bulk action addresses.
//
// A pool row has no tags and no provider of its own, so the two narrowing axes the
// endpoint selector carries are refused rather than ignored: an operator who asked to act
// on "the free-tier proxies" and got every proxy instead would have no way to know.
type BulkProxySelection struct {
	IDs []string
	All bool
}

// ProxyPinReport is what a bulk erase takes down with it.
type ProxyPinReport struct {
	Endpoints int
	Providers []string
}

// BulkProxyOutcome is what a proxy bulk action did, with the pin report beside it.
type BulkProxyOutcome struct {
	Matched int
	Changed int
	Results []BulkRowResult
	Pins    ProxyPinReport
}

// resolveBulkProxies narrows the stored pool to the set the selection names, and applies
// the same counted guard the account routes use: the number the operator confirmed has to
// be the number the store gives, or nothing is written.
func (s *ProxyService) resolveBulkProxies(ctx context.Context, sel BulkProxySelection, confirmCount int) ([]domain.Proxy, error) {
	if len(sel.IDs) == 0 && !sel.All {
		return nil, domain.NewBulkSelectorRequiredError("select candidates by ids, or send all=true")
	}
	all, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	if len(sel.IDs) == 0 {
		if confirmCount != len(all) {
			return nil, mismatchCount(confirmCount, len(all), "candidates")
		}
		return all, nil
	}
	wanted := make(map[string]struct{}, len(sel.IDs))
	for _, id := range sel.IDs {
		wanted[id] = struct{}{}
	}
	named := make([]domain.Proxy, 0, len(sel.IDs))
	for _, proxy := range all {
		if _, ok := wanted[proxy.ID()]; ok {
			named = append(named, proxy)
		}
	}
	if confirmCount != len(named) {
		return nil, mismatchCount(confirmCount, len(named), "candidates")
	}
	return named, nil
}

// BulkStatus switches every candidate the selection names on or off.
//
// The switch is the operator's own column and it is read by RouteUsable, so a candidate
// leaving the plan needs no Redis park and no probe rewrite: clearing a park here would
// overwrite evidence the prober owns, and an address the engine skips on a failed probe is
// a different fact from one the operator switched off.
func (s *ProxyService) BulkStatus(ctx context.Context, sel BulkProxySelection, enabled bool, confirmCount int) (BulkProxyOutcome, error) {
	candidates, err := s.resolveBulkProxies(ctx, sel, confirmCount)
	if err != nil {
		return BulkProxyOutcome{}, err
	}
	if len(candidates) > MaxBulkTargets {
		return BulkProxyOutcome{}, tooLarge(len(candidates))
	}
	now := s.clock()
	changed := make([]domain.Proxy, 0, len(candidates))
	outcome := BulkProxyOutcome{Matched: len(candidates), Results: make([]BulkRowResult, 0, len(candidates))}
	for i, proxy := range candidates {
		outcome.Results = append(outcome.Results, BulkRowResult{Index: i, ID: proxy.ID()})
		if proxy.Enabled() == enabled {
			continue
		}
		proxy.SetEnabled(enabled, now)
		changed = append(changed, proxy)
	}
	outcome.Changed = len(changed)
	if len(changed) == 0 {
		return outcome, nil
	}
	if err := s.repo.UpdateBulk(ctx, changed); err != nil {
		return BulkProxyOutcome{}, err
	}
	return outcome, nil
}

// BulkPreview answers what a bulk erase would take down, without erasing anything.
//
// The operator can only confirm a number they have been given, and `proxy_pool_id` is an id
// string rather than a foreign key, so storage is the only place that knows how many
// connections hang on the selected candidates. This is that read.
func (s *ProxyService) BulkPreview(ctx context.Context, sel BulkProxySelection, confirmCount int) (BulkProxyOutcome, error) {
	candidates, err := s.resolveBulkProxies(ctx, sel, confirmCount)
	if err != nil {
		return BulkProxyOutcome{}, err
	}
	pins, err := s.pinsReport(ctx, candidates)
	if err != nil {
		return BulkProxyOutcome{}, err
	}
	results := make([]BulkRowResult, 0, len(candidates))
	for i, proxy := range candidates {
		results = append(results, BulkRowResult{Index: i, ID: proxy.ID()})
	}
	return BulkProxyOutcome{Matched: len(candidates), Results: results, Pins: pins}, nil
}

// BulkDelete erases every candidate the selection names, in one transaction, after the
// operator has confirmed the endpoints that will lose their egress path with them.
//
// confirmEndpoints is checked against storage, not accepted on faith: a pool a provider
// pinned an endpoint to two minutes ago is exactly the case this number exists for, and
// the refusal answers with the count the pins actually add up to.
func (s *ProxyService) BulkDelete(ctx context.Context, sel BulkProxySelection, confirmCount, confirmEndpoints int) (BulkProxyOutcome, error) {
	candidates, err := s.resolveBulkProxies(ctx, sel, confirmCount)
	if err != nil {
		return BulkProxyOutcome{}, err
	}
	if len(candidates) > MaxBulkTargets {
		return BulkProxyOutcome{}, tooLarge(len(candidates))
	}
	pins, err := s.pinsReport(ctx, candidates)
	if err != nil {
		return BulkProxyOutcome{}, err
	}
	if confirmEndpoints != pins.Endpoints {
		return BulkProxyOutcome{}, domain.NewBulkTargetCountMismatchError(fmt.Sprintf(
			"erasing these %d candidates takes down %d endpoint pin(s) across %s, not the %d this action confirmed; refresh and repeat",
			len(candidates), pins.Endpoints, providerPhrase(pins.Providers), confirmEndpoints))
	}

	ids := make([]string, len(candidates))
	outcome := BulkProxyOutcome{
		Matched: len(candidates), Changed: len(candidates),
		Results: make([]BulkRowResult, 0, len(candidates)), Pins: pins,
	}
	for i, proxy := range candidates {
		ids[i] = proxy.ID()
		outcome.Results = append(outcome.Results, BulkRowResult{Index: i, ID: proxy.ID()})
	}
	if len(ids) == 0 {
		return outcome, nil
	}
	if err := s.repo.DeleteBulk(ctx, ids); err != nil {
		return BulkProxyOutcome{}, err
	}
	return outcome, nil
}

// pinsReport adds up what the selected candidates carry. A deployment without the reader
// refuses the erase rather than promising an empty report it did not read.
func (s *ProxyService) pinsReport(ctx context.Context, candidates []domain.Proxy) (ProxyPinReport, error) {
	if s.pins == nil {
		return ProxyPinReport{}, domain.NewInternalError("proxy reference reader is unavailable; bulk delete needs it to report what it takes down")
	}
	ids := make([]string, len(candidates))
	for i, proxy := range candidates {
		ids[i] = proxy.ID()
	}
	byPool, err := s.pins.ProxyPins(ctx, ids)
	if err != nil {
		return ProxyPinReport{}, err
	}
	report := ProxyPinReport{Providers: []string{}}
	seen := make(map[string]struct{})
	for _, pins := range byPool {
		for _, pin := range pins {
			report.Endpoints += pin.Endpoints
			if _, dup := seen[pin.ProviderID]; dup {
				continue
			}
			seen[pin.ProviderID] = struct{}{}
			report.Providers = append(report.Providers, pin.ProviderID)
		}
	}
	sort.Strings(report.Providers)
	return report, nil
}

func mismatchCount(confirmed, resolved int, noun string) error {
	return domain.NewBulkTargetCountMismatchError(fmt.Sprintf(
		"the selection now resolves to %d %s, not the %d this action confirmed; refresh the list and repeat",
		resolved, noun, confirmed))
}

func tooLarge(resolved int) error {
	return domain.NewBulkTargetTooLargeError(fmt.Sprintf(
		"this selection resolves to %d rows, and one action may address at most %d", resolved, MaxBulkTargets))
}

// providerPhrase names the providers losing a pin, bounded so a wide erase still reads.
func providerPhrase(providers []string) string {
	if len(providers) == 0 {
		return "no provider"
	}
	if len(providers) > 4 {
		return fmt.Sprintf("%d providers", len(providers))
	}
	return strings.Join(providers, ", ")
}
