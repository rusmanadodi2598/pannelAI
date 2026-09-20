// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/quota_read_test.go
// @for       Table-driven HTTP tests for the §7.12 quota reads: the per-endpoint
//
//	detail (cap plus windows) and the windows-only collection.
//
// @uses      encoding/json, internal/domain, net/http, net/http/httptest,
//
//	strings, testing, time.
//
// @reason    Owner decision D5 = (b) made the detail route carry the stored cap
//
//	so a client reads back what it wrote, and the collection route stay
//	windows-only because a cap belongs to one endpoint. Both shapes are
//	pinned as tables (draft 005 F6, TDD.md §2.5): the fresh-endpoint
//	answer, the read-back, and the rendered windows are variations of
//	one read, and the null cap must stay an explicit null the panel can
//	render as an empty form.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-20
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestQuotaHandler_Get covers the detail route as one read with variations:
// the fresh-endpoint null cap, the cap a PUT wrote read back before any window
// exists (the G12 round trip), and the windows rendered beneath the cap field.
func TestQuotaHandler_Get(t *testing.T) {
	cases := []struct {
		name  string
		seed  func(*testing.T, *QuotaHandler, *stubQuotaRepo)
		check func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "a fresh endpoint answers an explicit null cap and no windows",
			seed: func(*testing.T, *QuotaHandler, *stubQuotaRepo) {},
			check: func(t *testing.T, rec *httptest.ResponseRecorder) {
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
				if detail.EndpointID != "ep_g12" {
					t.Fatalf("GET endpoint_id = %q, want ep_g12", detail.EndpointID)
				}
				if len(detail.Data) != 0 {
					t.Fatalf("GET data = %d windows, want 0 before any traffic", len(detail.Data))
				}
			},
		},
		{
			name: "the cap a PUT wrote is read back before any window exists",
			seed: func(t *testing.T, h *QuotaHandler, _ *stubQuotaRepo) {
				put := httptest.NewRecorder()
				h.PutCap(put, quotaRequest(http.MethodPut, `{"monthly_cost_usd":"5","monthly_tokens":1000}`, "ep_g12"))
				if put.Code != http.StatusOK {
					t.Fatalf("PUT status = %d, want %d (body %s)", put.Code, http.StatusOK, put.Body.String())
				}
			},
			check: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var detail schema.QuotaEndpointDetail
				if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
					t.Fatalf("decoding GET body: %v", err)
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
			},
		},
		{
			name: "windows render beneath an explicit null cap",
			seed: func(_ *testing.T, _ *QuotaHandler, repo *stubQuotaRepo) {
				window, err := domain.NewQuotaWindow("ep_g12", "openai", domain.QuotaWindowMonthly, nil, nil, time.Now())
				if err != nil {
					t.Fatalf("NewQuotaWindow() error = %v", err)
				}
				repo.windows = append(repo.windows, window)
			},
			check: func(t *testing.T, rec *httptest.ResponseRecorder) {
				if !strings.Contains(rec.Body.String(), `"cap":null`) {
					t.Fatalf("GET body = %s, want an explicit \"cap\":null", rec.Body.String())
				}
				var detail schema.QuotaEndpointDetail
				if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
					t.Fatalf("decoding GET body: %v", err)
				}
				if len(detail.Data) != 1 || detail.Data[0].Window != "monthly" {
					t.Fatalf("GET data = %+v, want the stored monthly window", detail.Data)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, repo, _ := newQuotaFixture(t)
			tc.seed(t, handler, repo)
			rec := httptest.NewRecorder()
			handler.Get(rec, quotaRequest(http.MethodGet, "", "ep_g12"))
			if rec.Code != http.StatusOK {
				t.Fatalf("GET status = %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body.String())
			}
			tc.check(t, rec)
		})
	}
}

// TestQuotaHandler_List keeps the collection route's body as it was: windows
// for every endpoint, and never a cap, because a cap belongs to one endpoint.
func TestQuotaHandler_List(t *testing.T) {
	cases := []struct {
		name      string
		seed      func(*testing.T, *stubQuotaRepo)
		wantCount int
	}{
		{name: "an empty table answers an empty array", seed: func(*testing.T, *stubQuotaRepo) {}, wantCount: 0},
		{
			name: "stored windows render for every endpoint",
			seed: func(t *testing.T, repo *stubQuotaRepo) {
				window, err := domain.NewQuotaWindow("ep_g12", "openai", domain.QuotaWindowMonthly, nil, nil, time.Now())
				if err != nil {
					t.Fatalf("NewQuotaWindow() error = %v", err)
				}
				repo.windows = append(repo.windows, window)
			},
			wantCount: 1,
		},
		{
			name: "a stored cap is not leaked into the collection",
			seed: func(t *testing.T, repo *stubQuotaRepo) {
				cost, err := domain.ParseDecimal("5")
				if err != nil {
					t.Fatalf("ParseDecimal(5) error = %v", err)
				}
				cap, err := domain.NewQuotaCap("ep_g12", &cost, nil, time.Now())
				if err != nil {
					t.Fatalf("NewQuotaCap() error = %v", err)
				}
				repo.caps["ep_g12"] = cap
			},
			wantCount: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, repo, _ := newQuotaFixture(t)
			tc.seed(t, repo)
			rec := httptest.NewRecorder()
			handler.List(rec, httptest.NewRequest(http.MethodGet, "/api/v1/quotas", strings.NewReader("")))
			if rec.Code != http.StatusOK {
				t.Fatalf("List status = %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), `"cap"`) {
				t.Fatalf("List body = %s, want no cap field", rec.Body.String())
			}
			var list schema.QuotaWindowList
			if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
				t.Fatalf("decoding List body: %v", err)
			}
			if len(list.Data) != tc.wantCount {
				t.Fatalf("List data = %d windows, want %d", len(list.Data), tc.wantCount)
			}
		})
	}
}
