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

// QuotaService implements SPEC-API-001 §7.12.
type QuotaService struct {
	quotas repository.QuotaRepository
	usage  repository.UsageRecordRepository
	clock  func() time.Time
}

// QuotaServiceDeps holds the collaborators the service needs.
type QuotaServiceDeps struct {
	Quotas repository.QuotaRepository
	Usage  repository.UsageRecordRepository
}

// NewQuotaService validates deps and returns a ready service.
func NewQuotaService(deps QuotaServiceDeps) (*QuotaService, error) {
	if deps.Quotas == nil {
		return nil, domain.NewValidationError("quota repository is required")
	}
	return &QuotaService{quotas: deps.Quotas, usage: deps.Usage, clock: time.Now}, nil
}

// ListWindows returns the quota windows for one endpoint, or for every endpoint
// when endpointID is empty.
func (s *QuotaService) ListWindows(ctx context.Context, endpointID string) ([]domain.QuotaWindow, error) {
	return s.quotas.ListWindows(ctx, endpointID)
}

// SetCap replaces one endpoint's budget cap and returns the stored value.
//
// The cap is validated by the domain constructor, and the endpoint is required
// to exist first: a cap on an endpoint that is not configured is a typo the
// caller should see rather than a row nothing reads.
func (s *QuotaService) SetCap(ctx context.Context, endpointID string, monthlyCostUSD *domain.Decimal, monthlyTokens *int64) (domain.QuotaCap, error) {
	if endpointID == "" {
		return domain.QuotaCap{}, domain.NewValidationError("endpoint_id is required")
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
