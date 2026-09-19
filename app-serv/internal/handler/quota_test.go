// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/quota_test.go
// @for       HTTP tests for the §7.12 quota routes, including the cap read-back
//
//	owner decision D5 = (b) added.
//
// @uses      context, encoding/json, internal/domain, internal/service,
//
//	net/http, net/http/httptest, strings, testing, time.
//
// @reason    G12 in the P2 register: a budget cap could be written but never
//
//	read back, because the read route answered windows only and the
//	service's GetCap had no caller. The PUT-then-GET round trip is what
//	locks the contract, and the null-cap case is pinned separately so
//	"no cap stored" cannot be confused with a field the response forgot.
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
	"time"

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

// newQuotaFixture wires the real handler over the in-memory repository.
func newQuotaFixture(t *testing.T) (*QuotaHandler, *stubQuotaRepo) {
	t.Helper()
	repo := newStubQuotaRepo()
	quotas, err := service.NewQuotaService(service.QuotaServiceDeps{Quotas: repo})
	if err != nil {
		t.Fatalf("NewQuotaService() error = %v", err)
	}
	return NewQuotaHandler(quotas), repo
}

// quotaRequest builds a request with the path value a Go 1.22 ServeMux would
// install for /quotas/{endpoint_id}.
func quotaRequest(method, body, endpointID string) *http.Request {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, "/api/v1/quotas/"+endpointID, reader)
	if endpointID != "" {
		req.SetPathValue("endpoint_id", endpointID)
	}
	return req
}

// TestQuotaHandler_PutThenGetReadsTheCapBack is the G12 definition of done: one
// PUT, then a GET that shows the same cap — even though no usage window exists
// yet, which is exactly the state the panel hits on a fresh endpoint.
func TestQuotaHandler_PutThenGetReadsTheCapBack(t *testing.T) {
	handler, _ := newQuotaFixture(t)

	put := quotaRequest(http.MethodPut, `{"monthly_cost_usd":"5","monthly_tokens":1000}`, "ep_g12")
	putRec := httptest.NewRecorder()
	handler.PutCap(putRec, put)
	if putRec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d (body %s)", putRec.Code, http.StatusOK, putRec.Body.String())
	}
	var written schema.QuotaCapResponse
	if err := json.Unmarshal(putRec.Body.Bytes(), &written); err != nil {
		t.Fatalf("decoding PUT body: %v", err)
	}
	if written.MonthlyCostUSD == nil || *written.MonthlyCostUSD != "5.00000000" {
		t.Fatalf("PUT monthly_cost_usd = %v, want 5.00000000", written.MonthlyCostUSD)
	}

	get := quotaRequest(http.MethodGet, "", "ep_g12")
	getRec := httptest.NewRecorder()
	handler.Get(getRec, get)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d (body %s)", getRec.Code, http.StatusOK, getRec.Body.String())
	}
	var detail schema.QuotaEndpointDetail
	if err := json.Unmarshal(getRec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decoding GET body: %v", err)
	}
	if detail.EndpointID != "ep_g12" {
		t.Fatalf("GET endpoint_id = %q, want ep_g12", detail.EndpointID)
	}
	if detail.Cap == nil {
		t.Fatal("GET cap = null, want the cap that was just written")
	}
	if detail.Cap.MonthlyCostUSD == nil || *detail.Cap.MonthlyCostUSD != "5.00000000" {
		t.Fatalf("GET cap.monthly_cost_usd = %v, want 5.00000000", detail.Cap.MonthlyCostUSD)
	}
	if detail.Cap.MonthlyTokens == nil || *detail.Cap.MonthlyTokens != 1000 {
		t.Fatalf("GET cap.monthly_tokens = %v, want 1000", detail.Cap.MonthlyTokens)
	}
	if detail.Cap.UpdatedAt == "" {
		t.Fatal("GET cap.updated_at is empty, want the write's timestamp")
	}
	if len(detail.Data) != 0 {
		t.Fatalf("GET data = %d windows, want 0 before any traffic", len(detail.Data))
	}
}

// TestQuotaHandler_GetWithoutACapAnswersNull pins the fresh-endpoint answer:
// the field is present and null, and the windows still render.
func TestQuotaHandler_GetWithoutACapAnswersNull(t *testing.T) {
	handler, repo := newQuotaFixture(t)
	window, err := domain.NewQuotaWindow("ep_g12", "openai", domain.QuotaWindowMonthly, nil, nil, time.Now())
	if err != nil {
		t.Fatalf("NewQuotaWindow() error = %v", err)
	}
	repo.windows = append(repo.windows, window)

	get := quotaRequest(http.MethodGet, "", "ep_g12")
	rec := httptest.NewRecorder()
	handler.Get(rec, get)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"cap":null`) {
		t.Fatalf("GET body = %s, want an explicit \"cap\":null", rec.Body.String())
	}
	var detail schema.QuotaEndpointDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decoding GET body: %v", err)
	}
	if detail.Cap != nil {
		t.Fatalf("GET cap = %+v, want null", detail.Cap)
	}
	if len(detail.Data) != 1 || detail.Data[0].Window != "monthly" {
		t.Fatalf("GET data = %+v, want the stored monthly window", detail.Data)
	}
}

// TestQuotaHandler_ListStaysWindowsOnly keeps the collection route's body as it
// was: a cap belongs to one endpoint, so the list carries none.
func TestQuotaHandler_ListStaysWindowsOnly(t *testing.T) {
	handler, repo := newQuotaFixture(t)
	window, err := domain.NewQuotaWindow("ep_g12", "openai", domain.QuotaWindowMonthly, nil, nil, time.Now())
	if err != nil {
		t.Fatalf("NewQuotaWindow() error = %v", err)
	}
	repo.windows = append(repo.windows, window)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/quotas", strings.NewReader(""))
	rec := httptest.NewRecorder()
	handler.List(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("List status = %d, want %d", rec.Code, http.StatusOK)
	}
	if strings.Contains(rec.Body.String(), `"cap"`) {
		t.Fatalf("List body = %s, want no cap field", rec.Body.String())
	}
	var list schema.QuotaWindowList
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decoding List body: %v", err)
	}
	if len(list.Data) != 1 {
		t.Fatalf("List data = %d windows, want 1", len(list.Data))
	}
}

// TestQuotaHandler_PutCapRejectsANegativeCost is the route's validation path:
// the refusal happens before the service, and nothing is stored.
func TestQuotaHandler_PutCapRejectsANegativeCost(t *testing.T) {
	handler, repo := newQuotaFixture(t)

	put := quotaRequest(http.MethodPut, `{"monthly_cost_usd":"-1"}`, "ep_g12")
	rec := httptest.NewRecorder()
	handler.PutCap(rec, put)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT status = %d, want %d (body %s)", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "VALIDATION_ERROR") {
		t.Fatalf("PUT body = %s, want VALIDATION_ERROR", rec.Body.String())
	}
	if len(repo.caps) != 0 {
		t.Fatalf("stored caps = %d, want 0 after a rejected write", len(repo.caps))
	}
}
