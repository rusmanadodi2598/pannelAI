// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/quota_paging_test.go
// @for       Table-driven HTTP tests for the paged §7.12 collection read:
//
//	group pages, the meta block, and the pagination refusals.
//
// @uses      encoding/json, internal/domain, net/http, net/http/httptest,
//
//	sort, strings, testing, time.
//
// @reason    Owner directive 2026-09-26 moved the collection read onto the
//
//	house pagination convention (docs/PORT/006-PORT-QUOTA-CARDS.md
//	D5 filing): the page unit is the provider group, so a card never
//	splits across pages, and the meta block states the total group
//	count. The tables pin the page walk, the no-provider group
//	counting as one group, the defaults, and the refusals, so the
//	wire contract is proven rather than assumed.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-26
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// PageWindowsByProvider answers the paged read from the in-memory set, grouping
// the way the SQL does: distinct provider ids ordered by their smallest
// endpoint id, sliced to the page, with the total group count. The no-provider
// lane (empty provider id) is one group like any other.
func (r *stubQuotaRepo) PageWindowsByProvider(_ context.Context, page, perPage int) ([]domain.QuotaWindow, int64, error) {
	order := []string{}
	first := map[string]string{}
	for _, window := range r.windows {
		pid := window.ProviderID()
		id := window.EndpointID()
		if seen, ok := first[pid]; !ok {
			first[pid] = id
			order = append(order, pid)
		} else if id < seen {
			first[pid] = id
		}
	}
	sort.Slice(order, func(i, j int) bool { return first[order[i]] < first[order[j]] })

	total := int64(len(order))
	start := (page - 1) * perPage
	if start >= len(order) {
		return []domain.QuotaWindow{}, total, nil
	}
	end := start + perPage
	if end > len(order) {
		end = len(order)
	}
	keep := map[string]bool{}
	for _, pid := range order[start:end] {
		keep[pid] = true
	}
	out := make([]domain.QuotaWindow, 0)
	for _, window := range r.windows {
		if keep[window.ProviderID()] {
			out = append(out, window)
		}
	}
	return out, total, nil
}

// TestQuotaHandler_ListPaging covers the collection read as a page walk over
// provider groups: whole groups per page in first-seen order, the meta block
// with the honest total, the defaults, and the pagination refusals.
func TestQuotaHandler_ListPaging(t *testing.T) {
	seed := func(t *testing.T, repo *stubQuotaRepo) {
		t.Helper()
		rows := []struct {
			endpoint string
			provider string
			kind     domain.QuotaWindowKind
		}{
			{endpoint: "ep_a", provider: "alpha", kind: domain.QuotaWindowMonthly},
			{endpoint: "ep_a", provider: "alpha", kind: domain.QuotaWindowDaily},
			{endpoint: "ep_b", provider: "alpha", kind: domain.QuotaWindowMonthly},
			{endpoint: "ep_c", provider: "bravo", kind: domain.QuotaWindowMonthly},
			{endpoint: "ep_c", provider: "bravo", kind: domain.QuotaWindowDaily},
			{endpoint: "ep_y", provider: "", kind: domain.QuotaWindowMonthly},
			{endpoint: "ep_z", provider: "", kind: domain.QuotaWindowDaily},
		}
		for _, row := range rows {
			window, err := domain.NewQuotaWindow(row.endpoint, row.provider, row.kind, nil, nil, time.Now())
			if err != nil {
				t.Fatalf("NewQuotaWindow(%s) error = %v", row.endpoint, err)
			}
			repo.windows = append(repo.windows, window)
		}
	}

	cases := []struct {
		name          string
		query         string
		wantStatus    int
		wantCode      string
		wantPage      int
		wantPerPage   int
		wantTotal     int64
		wantProviders []string
	}{
		{
			name:     "the first page returns whole provider groups in first-seen order",
			query:    "?per_page=2",
			wantPage: 1, wantPerPage: 2, wantTotal: 3,
			wantProviders: []string{"alpha", "bravo"},
		},
		{
			name:     "the second page returns the remaining group with its whole windows",
			query:    "?page=2&per_page=2",
			wantPage: 2, wantPerPage: 2, wantTotal: 3,
			wantProviders: []string{""},
		},
		{
			name:     "a page past the end answers an empty array with the honest total",
			query:    "?page=9&per_page=2",
			wantPage: 9, wantPerPage: 2, wantTotal: 3,
			wantProviders: []string{},
		},
		{
			name:     "the defaults apply when no params are sent",
			query:    "",
			wantPage: 1, wantPerPage: 25, wantTotal: 3,
			wantProviders: []string{"alpha", "bravo", ""},
		},
		{
			name:       "per_page above the cap is refused, not clamped",
			query:      "?per_page=101",
			wantStatus: http.StatusBadRequest, wantCode: "per_page must be at most 100",
		},
		{
			name:       "a non-numeric page is refused",
			query:      "?page=x",
			wantStatus: http.StatusBadRequest, wantCode: "page must be an integer",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, repo, _ := newQuotaFixture(t)
			seed(t, repo)
			rec := httptest.NewRecorder()
			handler.List(rec, httptest.NewRequest(http.MethodGet, "/api/v1/quotas"+tc.query, strings.NewReader("")))

			if tc.wantStatus != 0 {
				if rec.Code != tc.wantStatus {
					t.Fatalf("List status = %d, want %d (body %s)", rec.Code, tc.wantStatus, rec.Body.String())
				}
				if !strings.Contains(rec.Body.String(), tc.wantCode) {
					t.Fatalf("List body = %s, want %q", rec.Body.String(), tc.wantCode)
				}
				return
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("List status = %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body.String())
			}

			var list schema.QuotaWindowList
			if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
				t.Fatalf("decoding List body: %v", err)
			}
			if list.Meta.Page != tc.wantPage || list.Meta.PerPage != tc.wantPerPage || list.Meta.Total != tc.wantTotal {
				t.Fatalf("List meta = %+v, want page %d per_page %d total %d", list.Meta, tc.wantPage, tc.wantPerPage, tc.wantTotal)
			}

			got := []string{}
			seen := map[string]bool{}
			for _, window := range list.Data {
				if !seen[window.ProviderID] {
					seen[window.ProviderID] = true
					got = append(got, window.ProviderID)
				}
			}
			if len(got) != len(tc.wantProviders) {
				t.Fatalf("List data groups = %v, want %v", got, tc.wantProviders)
			}
			for i := range got {
				if got[i] != tc.wantProviders[i] {
					t.Fatalf("List data groups = %v, want %v", got, tc.wantProviders)
				}
			}
		})
	}
}
