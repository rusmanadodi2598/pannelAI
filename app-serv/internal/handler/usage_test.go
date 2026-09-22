// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/usage_test.go
// @for       Table-driven HTTP tests for the §7.12 usage summary and timeseries
//
//	reads: what the routes answer, what they forward, and every refusal.
//
// @uses      internal/domain, net/http, net/http/httptest, strings, testing.
//
// @reason    Draft 010 F1: the four usage routes had service coverage and a
//
//	router session sweep but no request-to-response test, so AGENTS.md
//	§2.1 (happy, validation, auth per route) was unmet and a mapper
//	regression could only surface in the panel. Each route is one
//	behaviour with variations (TDD.md §2.5). The 401 answers are the
//	router sweep's property, so they live in the router package's usage
//	route test; the records and detail routes have their own sibling
//	file to stay inside the §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-22
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestUsageHandler_Summary covers the totals read: the happy shape with and
// without a breakdown, filter forwarding, and the refusals the boundary owes
// (malformed timestamps, an inverted range, unknown enum values, an over-long
// free-text filter, and a storage failure surfacing as 500).
func TestUsageHandler_Summary(t *testing.T) {
	cases := []usageCase{
		{
			name:     "totals with a provider breakdown",
			query:    "?group_by=provider",
			want:     http.StatusOK,
			contains: []string{`"group_by":"provider"`, `"requests":1`, `"error_count":1`, `"error_rate":"1.0000"`, `"key":"openai"`},
		},
		{
			name:     "totals alone carry an empty group_by, not a wrong one",
			query:    "",
			want:     http.StatusOK,
			contains: []string{`"group_by":""`, `"requests":1`, `"cost_usd":"0.25"`},
		},
		{
			name:     "a malformed from is a validation error",
			query:    "?from=banana",
			want:     http.StatusBadRequest,
			code:     "VALIDATION_ERROR",
			contains: []string{"from must be an RFC3339 timestamp"},
		},
		{
			name:     "an inverted range is a validation error",
			query:    "?from=" + rfc3339(usageWindow[1]) + "&to=" + rfc3339(usageWindow[0]),
			want:     http.StatusBadRequest,
			code:     "VALIDATION_ERROR",
			contains: []string{"to must not be earlier than from"},
		},
		{
			name:     "an unknown group_by is a validation error",
			query:    "?group_by=banana",
			want:     http.StatusBadRequest,
			code:     "VALIDATION_ERROR",
			contains: []string{"group_by must be one of provider, model, endpoint, gateway_key"},
		},
		{
			name:     "an unknown status is refused, never a silent empty read",
			query:    "?status=banana",
			want:     http.StatusBadRequest,
			code:     "VALIDATION_ERROR",
			contains: []string{"status must be one of success, error"},
		},
		{
			name:     "an over-long free-text filter is a validation error",
			query:    "?q=" + strings.Repeat("a", 201),
			want:     http.StatusBadRequest,
			code:     "VALIDATION_ERROR",
			contains: []string{"q must be at most 200 characters"},
		},
		{
			name: "a storage failure is an internal error, never driver text",
			want: http.StatusInternalServerError,
			code: "INTERNAL_ERROR",
			seed: func(r *stubUsageRepo) { r.sumErr = errUsageRepo },
		},
	}
	runUsageCases(t, cases, func(h *UsageHandler, w http.ResponseWriter, r *http.Request) { h.Summary(w, r) }, "/api/v1/usage/summary")
}

// TestUsageHandler_SummaryForwardsFilters proves the validated query reaches
// the repository as the domain filter the caller wrote, which no shape
// assertion can see.
func TestUsageHandler_SummaryForwardsFilters(t *testing.T) {
	h, repo := newUsageHandlerFixture(t, nil)
	query := "?from=" + rfc3339(usageWindow[0]) + "&to=" + rfc3339(usageWindow[1]) +
		"&group_by=model&status=error&provider_id=openai&endpoint_id=ep_010&model=gpt-4o&gateway_key_id=gky_010&q=gpt"
	rec := httptest.NewRecorder()
	h.Summary(rec, usageRequest(http.MethodGet, "/api/v1/usage/summary"+query, ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if !repo.lastSum.From.Equal(usageWindow[0]) || !repo.lastSum.To.Equal(usageWindow[1]) {
		t.Fatalf("forwarded window = %s to %s, want %s to %s",
			repo.lastSum.From, repo.lastSum.To, usageWindow[0], usageWindow[1])
	}
	if repo.lastSum.ProviderID != "openai" || repo.lastSum.EndpointID != "ep_010" ||
		repo.lastSum.Model != "gpt-4o" || repo.lastSum.GatewayKey != "gky_010" || repo.lastSum.Query != "gpt" {
		t.Fatalf("forwarded filters = %+v", repo.lastSum)
	}
	if repo.lastSum.Status != domain.UsageStatusError {
		t.Fatalf("forwarded status = %q, want error", repo.lastSum.Status)
	}
	if repo.lastGrp != domain.UsageGroupModel {
		t.Fatalf("forwarded group_by = %q, want model", repo.lastGrp)
	}
}

// TestUsageHandler_Timeseries covers the bucketed read: the default
// granularity, the explicit one, the closed-set refusal, and the storage
// failure.
func TestUsageHandler_Timeseries(t *testing.T) {
	cases := []usageCase{
		{
			name:     "the granularity defaults to hour",
			want:     http.StatusOK,
			contains: []string{`"granularity":"hour"`, `"buckets":[{"bucket"`},
		},
		{
			name:     "a day granularity is answered",
			query:    "?granularity=day",
			want:     http.StatusOK,
			contains: []string{`"granularity":"day"`},
		},
		{
			name:     "an unknown granularity is a validation error",
			query:    "?granularity=week",
			want:     http.StatusBadRequest,
			code:     "VALIDATION_ERROR",
			contains: []string{"granularity must be one of hour, day"},
		},
		{
			name:     "a malformed to is a validation error",
			query:    "?to=banana",
			want:     http.StatusBadRequest,
			code:     "VALIDATION_ERROR",
			contains: []string{"to must be an RFC3339 timestamp"},
		},
		{
			name: "a storage failure is an internal error",
			want: http.StatusInternalServerError,
			code: "INTERNAL_ERROR",
			seed: func(r *stubUsageRepo) { r.seriesErr = errUsageRepo },
		},
	}
	runUsageCases(t, cases, func(h *UsageHandler, w http.ResponseWriter, r *http.Request) { h.Timeseries(w, r) }, "/api/v1/usage/timeseries")
}
