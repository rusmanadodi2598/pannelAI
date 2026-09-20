// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/quota_test.go
// @for       Table-driven HTTP tests for the §7.12 cap write: what the route
//
//	stores, what it echoes, and every refusal it answers.
//
// @uses      context, encoding/json, internal/domain, internal/service,
//
//	net/http, net/http/httptest, strings, testing, time.
//
// @reason    G12 in the P2 register locked the PUT round trip as one script;
//
//	draft 005 F2 adds the endpoint-existence refusal and F6 turns the
//	single-scenario tests into tables (TDD.md §2.5), so the route's
//	refusals are pinned as variations of one behaviour instead of four
//	unrelated ones. The 401 answers of the three quota routes are the
//	router sweep's property (TestEveryManagementRouteRejectsAnonymousCallers,
//	draft 004 F4), so no per-route auth test is duplicated here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// stubQuotaRepo is an in-memory QuotaRepository: windows and caps per endpoint.
type stubQuotaRepo struct {
	windows []domain.QuotaWindow
	caps    map[string]domain.QuotaCap
}

func newStubQuotaRepo() *stubQuotaRepo {
	return &stubQuotaRepo{caps: map[string]domain.QuotaCap{}}
}

func (r *stubQuotaRepo) ListWindows(_ context.Context, endpointID string) ([]domain.QuotaWindow, error) {
	if endpointID == "" {
		return r.windows, nil
	}
	var out []domain.QuotaWindow
	for _, window := range r.windows {
		if window.EndpointID() == endpointID {
			out = append(out, window)
		}
	}
	return out, nil
}

func (r *stubQuotaRepo) UpsertWindows(context.Context, []domain.QuotaWindow) error { return nil }

func (r *stubQuotaRepo) GetCap(_ context.Context, endpointID string) (domain.QuotaCap, error) {
	stored, ok := r.caps[endpointID]
	if !ok {
		return domain.QuotaCap{}, domain.ErrQuotaCapNotFound
	}
	return stored, nil
}

func (r *stubQuotaRepo) SetCap(_ context.Context, cap domain.QuotaCap) error {
	r.caps[cap.EndpointID()] = cap
	return nil
}

// stubEndpointRepo answers GetByID from a fixed set, so a test decides which
// endpoint ids the gateway knows. lookErr stands in for a storage failure that
// is not a missing row.
type stubEndpointRepo struct {
	known   map[string]bool
	lookErr error
}

func (r *stubEndpointRepo) GetByID(_ context.Context, id string) (domain.UpstreamEndpoint, error) {
	if r.lookErr != nil {
		return domain.UpstreamEndpoint{}, r.lookErr
	}
	if r.known[id] {
		return domain.UpstreamEndpoint{}, nil
	}
	return domain.UpstreamEndpoint{}, domain.ErrEndpointNotFound
}

// newQuotaFixture wires the real handler over the in-memory repositories, with
// ep_g12 as the one configured endpoint.
func newQuotaFixture(t *testing.T) (*QuotaHandler, *stubQuotaRepo, *stubEndpointRepo) {
	t.Helper()
	repo := newStubQuotaRepo()
	endpoints := &stubEndpointRepo{known: map[string]bool{"ep_g12": true}}
	quotas, err := service.NewQuotaService(service.QuotaServiceDeps{Quotas: repo, Endpoints: endpoints})
	if err != nil {
		t.Fatalf("NewQuotaService() error = %v", err)
	}
	return NewQuotaHandler(quotas), repo, endpoints
}

// quotaRequest builds a request with the path value a Go 1.22 ServeMux would
// install for /quotas/{endpoint_id}.
func quotaRequest(method, body, endpointID string) *http.Request {
	req := httptest.NewRequest(method, "/api/v1/quotas/"+endpointID, strings.NewReader(body))
	if endpointID != "" {
		req.SetPathValue("endpoint_id", endpointID)
	}
	return req
}

// TestQuotaHandler_PutCap covers the write route as one behaviour with
// variations: what a legal cap stores and echoes, and every refusal, of which
// the unknown endpoint is draft 005 F2's definition of done.
func TestQuotaHandler_PutCap(t *testing.T) {
	cases := []struct {
		name       string
		endpointID string
		body       string
		wantStatus int
		wantCode   string
		wantCaps   int
		check      func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:       "a cap on a configured endpoint is stored and echoed",
			endpointID: "ep_g12",
			body:       `{"monthly_cost_usd":"5","monthly_tokens":1000}`,
			wantStatus: http.StatusOK, wantCaps: 1,
			check: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var written schema.QuotaCapResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &written); err != nil {
					t.Fatalf("decoding PUT body: %v", err)
				}
				if written.EndpointID != "ep_g12" {
					t.Fatalf("endpoint_id = %q, want ep_g12", written.EndpointID)
				}
				if written.MonthlyCostUSD == nil || *written.MonthlyCostUSD != "5.00000000" {
					t.Fatalf("monthly_cost_usd = %v, want 5.00000000", written.MonthlyCostUSD)
				}
				if written.MonthlyTokens == nil || *written.MonthlyTokens != 1000 {
					t.Fatalf("monthly_tokens = %v, want 1000", written.MonthlyTokens)
				}
				if written.UpdatedAt == "" {
					t.Fatal("updated_at is empty, want the write's timestamp")
				}
			},
		},
		{
			name:       "a cap at the cost ceiling is accepted",
			endpointID: "ep_g12",
			body:       `{"monthly_cost_usd":"1000000000"}`,
			wantStatus: http.StatusOK, wantCaps: 1,
		},
		{
			name:       "a cap on an endpoint the gateway does not know is NOT_FOUND",
			endpointID: "ep_missing",
			body:       `{"monthly_tokens":10}`,
			wantStatus: http.StatusNotFound, wantCode: "NOT_FOUND",
		},
		{
			name:       "a negative cost is refused and stores nothing",
			endpointID: "ep_g12",
			body:       `{"monthly_cost_usd":"-1"}`,
			wantStatus: http.StatusBadRequest, wantCode: "VALIDATION_ERROR",
		},
		{
			name:       "a zero cost with no token cap is refused",
			endpointID: "ep_g12",
			body:       `{"monthly_cost_usd":"0"}`,
			wantStatus: http.StatusBadRequest, wantCode: "VALIDATION_ERROR",
		},
		{
			name:       "a cost past the ceiling is refused",
			endpointID: "ep_g12",
			body:       `{"monthly_cost_usd":"1000000000.00000001"}`,
			wantStatus: http.StatusBadRequest, wantCode: "VALIDATION_ERROR",
		},
		{
			name:       "a body that is not JSON is refused",
			endpointID: "ep_g12",
			body:       `{`,
			wantStatus: http.StatusBadRequest, wantCode: "VALIDATION_ERROR",
		},
		{
			name:       "clearing both caps on a configured endpoint succeeds",
			endpointID: "ep_g12",
			body:       `{}`,
			wantStatus: http.StatusOK, wantCaps: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, repo, _ := newQuotaFixture(t)
			rec := httptest.NewRecorder()
			handler.PutCap(rec, quotaRequest(http.MethodPut, tc.body, tc.endpointID))
			if rec.Code != tc.wantStatus {
				t.Fatalf("PUT status = %d, want %d (body %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantCode != "" && !strings.Contains(rec.Body.String(), tc.wantCode) {
				t.Fatalf("PUT body = %s, want code %s", rec.Body.String(), tc.wantCode)
			}
			if len(repo.caps) != tc.wantCaps {
				t.Fatalf("stored caps = %d, want %d", len(repo.caps), tc.wantCaps)
			}
			if tc.check != nil {
				tc.check(t, rec)
			}
		})
	}
}
