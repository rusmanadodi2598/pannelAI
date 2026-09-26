// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/quota.go
// @for       Quota window reads, budget-cap writes, and the Redis-to-PostgreSQL
//
//	counter flush worker.
//
// @uses      internal/domain, internal/repository, context, log/slog, sync, time.
// @reason    SPEC-API-001 §7.12 reads quota windows, writes budget caps the
//
//	router honours, and keeps hot counters in Redis flushed to
//	PostgreSQL. The flush is the one place in this vertical that owns a
//	long-lived goroutine, so its lifecycle is stated here rather than
//	implied: see the worker section below.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-18
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// EndpointFinder answers whether an endpoint id is configured. It is the one
// read the quota service needs from endpoint storage, declared narrowly so the
// service asks a question rather than depending on the whole store
// (AGENTS.md §1.5); repository.EndpointRepository satisfies it as is.
type EndpointFinder interface {
	GetByID(ctx context.Context, id string) (domain.UpstreamEndpoint, error)
}

// QuotaService implements SPEC-API-001 §7.12.
type QuotaService struct {
	quotas    repository.QuotaRepository
	usage     repository.UsageRecordRepository
	endpoints EndpointFinder
	clock     func() time.Time
}

// QuotaServiceDeps holds the collaborators the service needs.
type QuotaServiceDeps struct {
	Quotas    repository.QuotaRepository
	Usage     repository.UsageRecordRepository
	Endpoints EndpointFinder
}

// NewQuotaService validates deps and returns a ready service.
func NewQuotaService(deps QuotaServiceDeps) (*QuotaService, error) {
	if deps.Quotas == nil {
		return nil, domain.NewValidationError("quota repository is required")
	}
	if deps.Endpoints == nil {
		return nil, domain.NewValidationError("endpoint finder is required")
	}
	return &QuotaService{quotas: deps.Quotas, usage: deps.Usage, endpoints: deps.Endpoints, clock: time.Now}, nil
}

// ListWindows returns the quota windows for one endpoint, or for every endpoint
// when endpointID is empty.
func (s *QuotaService) ListWindows(ctx context.Context, endpointID string) ([]domain.QuotaWindow, error) {
	return s.quotas.ListWindows(ctx, endpointID)
}

// ListWindowsPaged returns one page of the collection read with the total
// provider-group count. The page unit is the provider group
// (docs/PORT/006-PORT-QUOTA-PAGING.md D1): one page carries every window of the
// page's groups, so a provider's card never splits across pages.
func (s *QuotaService) ListWindowsPaged(ctx context.Context, page, perPage int) ([]domain.QuotaWindow, int64, error) {
	return s.quotas.PageWindowsByProvider(ctx, page, perPage)
}

// SetCap replaces one endpoint's budget cap and returns the stored value.
//
// The endpoint must exist first: a cap on an endpoint that is not configured is
// a typo the caller should see (NOT_FOUND) rather than a row no selector will
// ever read. Existence is judged before the cap itself, because the endpoint is
// the resource the request names; a lookup that fails for any other reason is
// wrapped and stays an internal error instead of masquerading as not-found.
func (s *QuotaService) SetCap(ctx context.Context, endpointID string, monthlyCostUSD *domain.Decimal, monthlyTokens *int64) (domain.QuotaCap, error) {
	if endpointID == "" {
		return domain.QuotaCap{}, domain.NewValidationError("endpoint_id is required")
	}
	if _, err := s.endpoints.GetByID(ctx, endpointID); err != nil {
		if errors.Is(err, domain.ErrEndpointNotFound) {
			return domain.QuotaCap{}, domain.ErrEndpointNotFound
		}
		return domain.QuotaCap{}, fmt.Errorf("checking endpoint %s: %w", endpointID, err)
	}
	cap, err := domain.NewQuotaCap(endpointID, monthlyCostUSD, monthlyTokens, s.clock())
	if err != nil {
		return domain.QuotaCap{}, err
	}
	if err := s.quotas.SetCap(ctx, cap); err != nil {
		return domain.QuotaCap{}, fmt.Errorf("storing quota cap: %w", err)
	}
	return cap, nil
}

// GetCap returns one endpoint's budget cap. A missing cap is reported as the
// zero value with no error, because "no cap" is a state a caller displays, not
// a failure.
func (s *QuotaService) GetCap(ctx context.Context, endpointID string) (domain.QuotaCap, bool, error) {
	cap, err := s.quotas.GetCap(ctx, endpointID)
	if err != nil {
		if errors.Is(err, domain.ErrQuotaCapNotFound) {
			return domain.QuotaCap{}, false, nil
		}
		return domain.QuotaCap{}, false, err
	}
	return cap, true, nil
}

// Exhausted reports whether one endpoint has spent its budget cap, which is the
// question the router asks before picking an endpoint (SPEC-API-001 §7.12).
//
// An endpoint with no stored cap is never exhausted: "uncapped" and "used up" are
// different states, and treating a missing row as a zero cap would skip every
// endpoint an operator never capped.
//
// The usage read is the month-to-date aggregate, which is what a cap is defined
// against. It is read only when a cap exists, so an uncapped gateway does not pay
// an aggregate query per selection.
func (s *QuotaService) Exhausted(ctx context.Context, endpointID string) (bool, error) {
	if endpointID == "" {
		return false, nil
	}
	if s.usage == nil {
		return false, nil
	}
	cap, stored, err := s.GetCap(ctx, endpointID)
	if err != nil {
		return false, err
	}
	if !stored {
		return false, nil
	}
	totals, err := s.usage.MonthlyUsage(ctx, endpointID, s.clock())
	if err != nil {
		return false, fmt.Errorf("reading month-to-date usage: %w", err)
	}
	// The aggregate carries the cost as the decimal string the wire uses (§4),
	// so it is parsed back into the value object the cap compares with. An
	// unparseable total cannot come from this gateway's own writer; refusing to
	// read it as zero keeps a corrupt row from silently un-capping an endpoint.
	spent, err := domain.ParseDecimal(totals.CostUSD)
	if err != nil {
		return false, fmt.Errorf("parsing month-to-date cost: %w", err)
	}
	return cap.Exhausted(spent, totals.TokensIn+totals.TokensOut), nil
}
