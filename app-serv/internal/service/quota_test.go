// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/quota_test.go
// @for       Table-driven tests for the cap write: which endpoint ids a cap is
//
//	accepted for, and which refusals the service answers with.
//
// @uses      context, errors, internal/domain, testing.
// @reason    Draft 005 F2 found SetCap writing caps for endpoints that are not
//
//	configured, rows the router would never read, while its own doc
//	comment claimed the opposite. These cases pin the refusal
//	(NOT_FOUND for an unknown endpoint, ahead of any cap validation)
//	and the wrapped storage failure, so the comment and the behaviour
//	cannot drift apart again.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-20
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// stubCapRepo is an in-memory QuotaRepository that records the caps written.
type stubCapRepo struct {
	caps     map[string]domain.QuotaCap
	storeErr error
}

func (r *stubCapRepo) ListWindows(context.Context, string) ([]domain.QuotaWindow, error) {
	return nil, nil
}

func (r *stubCapRepo) UpsertWindows(context.Context, []domain.QuotaWindow) error { return nil }

func (r *stubCapRepo) GetCap(_ context.Context, endpointID string) (domain.QuotaCap, error) {
	stored, ok := r.caps[endpointID]
	if !ok {
		return domain.QuotaCap{}, domain.ErrQuotaCapNotFound
	}
	return stored, nil
}

func (r *stubCapRepo) SetCap(_ context.Context, cap domain.QuotaCap) error {
	if r.storeErr != nil {
		return r.storeErr
	}
	r.caps[cap.EndpointID()] = cap
	return nil
}

// stubEndpointLookup answers GetByID from a fixed set, so a test decides which
// endpoint ids are configured and when the lookup itself fails.
type stubEndpointLookup struct {
	known   map[string]bool
	lookErr error
}

func (r *stubEndpointLookup) GetByID(_ context.Context, id string) (domain.UpstreamEndpoint, error) {
	if r.lookErr != nil {
		return domain.UpstreamEndpoint{}, r.lookErr
	}
	if r.known[id] {
		return domain.UpstreamEndpoint{}, nil
	}
	return domain.UpstreamEndpoint{}, domain.ErrEndpointNotFound
}

// newQuotaServiceFixture wires the real service over in-memory doubles.
func newQuotaServiceFixture(t *testing.T, endpoints *stubEndpointLookup) (*QuotaService, *stubCapRepo) {
	t.Helper()
	repo := &stubCapRepo{caps: map[string]domain.QuotaCap{}}
	quotas, err := NewQuotaService(QuotaServiceDeps{Quotas: repo, Endpoints: endpoints})
	if err != nil {
		t.Fatalf("NewQuotaService() error = %v", err)
	}
	return quotas, repo
}

func capCostPtr(value string) *domain.Decimal {
	parsed, err := domain.ParseDecimal(value)
	if err != nil {
		panic("capCostPtr: " + err.Error())
	}
	return &parsed
}

func capTokensPtr(value int64) *int64 { return &value }

// TestQuotaService_SetCap covers the cap write as one behaviour with
// variations: a cap is stored for a configured endpoint, refused for an unknown
// one ahead of any cap validation, and a lookup or store failure never becomes
// a not-found answer.
func TestQuotaService_SetCap(t *testing.T) {
	zeroCost := domain.ZeroDecimal()

	cases := []struct {
		name       string
		endpoints  *stubEndpointLookup
		endpointID string
		cost       *domain.Decimal
		tokens     *int64
		wantCode   string
	}{
		{
			name:       "a cap for a configured endpoint is stored",
			endpoints:  &stubEndpointLookup{known: map[string]bool{"ep_live": true}},
			endpointID: "ep_live", cost: capCostPtr("5"), tokens: capTokensPtr(1000),
		},
		{
			name:       "a cap for an unknown endpoint is NOT_FOUND",
			endpoints:  &stubEndpointLookup{},
			endpointID: "ep_typo", cost: capCostPtr("5"), tokens: capTokensPtr(1000),
			wantCode: "NOT_FOUND",
		},
		{
			name:       "an unknown endpoint is refused before the cap is judged",
			endpoints:  &stubEndpointLookup{},
			endpointID: "ep_typo", tokens: capTokensPtr(-1),
			wantCode: "NOT_FOUND",
		},
		{
			name:       "an empty endpoint id is a validation error",
			endpoints:  &stubEndpointLookup{known: map[string]bool{"ep_live": true}},
			endpointID: "",
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "an invalid cap on a configured endpoint is a validation error",
			endpoints:  &stubEndpointLookup{known: map[string]bool{"ep_live": true}},
			endpointID: "ep_live", cost: &zeroCost,
			wantCode: "VALIDATION_ERROR",
		},
		{
			name:       "a lookup failure is not answered as NOT_FOUND",
			endpoints:  &stubEndpointLookup{lookErr: errors.New("pool exhausted")},
			endpointID: "ep_live", cost: capCostPtr("5"),
			wantCode: "INTERNAL_ERROR",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			quotas, repo := newQuotaServiceFixture(t, tc.endpoints)
			_, err := quotas.SetCap(context.Background(), tc.endpointID, tc.cost, tc.tokens)
			if tc.wantCode == "" {
				if err != nil {
					t.Fatalf("SetCap() error = %v, want nil", err)
				}
				if len(repo.caps) != 1 {
					t.Fatalf("stored caps = %d, want 1", len(repo.caps))
				}
				return
			}
			appErr := domain.AsAppError(err)
			if appErr.Code != tc.wantCode {
				t.Fatalf("SetCap() code = %s (error %v), want %s", appErr.Code, err, tc.wantCode)
			}
			if tc.wantCode == "NOT_FOUND" && !errors.Is(err, domain.ErrEndpointNotFound) {
				t.Fatalf("SetCap() error = %v, want the endpoint not-found sentinel", err)
			}
			if len(repo.caps) != 0 {
				t.Fatalf("stored caps = %d, want 0 after a refused write", len(repo.caps))
			}
		})
	}
}
