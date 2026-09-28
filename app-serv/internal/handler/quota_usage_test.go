// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/quota_usage_test.go
// @for       HTTP tests for GET /api/v1/quotas/{endpoint_id}/usage: the body a
//
//	provider's buckets become, and the refusals the route answers.
//
// @uses      encoding/json, internal/domain, internal/registry, internal/service,
//
//	internal/service/quotafetch, net/http, net/http/httptest, testing,
//	time.
//
// @reason    The amounts on this route are the only ones in §7.12 that a provider
//
//	reports rather than this gateway counts, so the decimal-string rule
//	(§4) and the difference between "no ceiling" and "a ceiling of zero"
//	are pinned here at the wire, where a client would otherwise read a
//	spent account and an unlimited one as the same card. The 401 answer
//	is the router sweep's property, not this route's.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-28
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/quotafetch"
)

// TestQuotaHandler_GetUsageRendersProviderBuckets pins the wire shape of a live
// read: decimal strings, an omitted ceiling for an unbounded bucket, and the
// provider's plan beside the instant the read happened.
func TestQuotaHandler_GetUsageRendersProviderBuckets(t *testing.T) {
	resets := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	handler := newPublishedHandler(t, publishedOAuthEndpoint(t, "qoder", "dt-token"),
		publishedEntry("qoder", true, true),
		quotafetch.Result{
			Plan: "personal_professional_trial",
			Quotas: []quotafetch.Quota{
				{Label: "Resource Pack", Used: 12.5, Total: 3000, ResetAt: resets},
				{Label: "Used (USD)", Used: 4, Unlimited: true},
			},
		})

	rec := httptest.NewRecorder()
	handler.GetUsage(rec, quotaRequest(http.MethodGet, "", "ep_pub"))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	body := decodePublished(t, rec)
	if body.EndpointID != "ep_pub" || body.ProviderID != "qoder" {
		t.Fatalf("identity = %q/%q, want ep_pub/qoder", body.EndpointID, body.ProviderID)
	}
	if body.Plan != "personal_professional_trial" || body.FetchedAt == "" {
		t.Fatalf("body = %+v, want the provider's plan and a read stamp", body)
	}
	if len(body.Data) != 2 {
		t.Fatalf("data = %+v, want two buckets", body.Data)
	}
	first := body.Data[0]
	if first.Label != "Resource Pack" || first.Used != "12.5" {
		t.Fatalf("first bucket = %+v, want used as the decimal string 12.5", first)
	}
	if first.Total == nil || *first.Total != "3000" {
		t.Fatalf("total = %v, want 3000 as a decimal string", first.Total)
	}
	if first.ResetsAt == nil || *first.ResetsAt != "2026-10-01T00:00:00Z" {
		t.Fatalf("resets_at = %v, want the RFC3339 instant", first.ResetsAt)
	}
	second := body.Data[1]
	if second.Total != nil {
		t.Fatalf("unbounded bucket = %+v, want total omitted, not zero", second)
	}
	if second.ResetsAt != nil {
		t.Fatalf("bucket with no reset = %+v, want resets_at omitted", second)
	}
}

// TestQuotaHandler_GetUsageSoftMessage pins that a provider which answers with a
// sentence is a 200 the card renders, with an array rather than a null beside it.
func TestQuotaHandler_GetUsageSoftMessage(t *testing.T) {
	handler := newPublishedHandler(t, publishedOAuthEndpoint(t, "qoder", "dt-token"),
		publishedEntry("qoder", true, true),
		quotafetch.Result{Message: "Qoder reports this account's quota as exceeded."})

	rec := httptest.NewRecorder()
	handler.GetUsage(rec, quotaRequest(http.MethodGet, "", "ep_pub"))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("body is not valid JSON: %s", rec.Body.String())
	}
	body := decodePublished(t, rec)
	if body.Message != "Qoder reports this account's quota as exceeded." {
		t.Fatalf("message = %q, want the provider's own sentence", body.Message)
	}
	if body.Data == nil || len(body.Data) != 0 {
		t.Fatalf("data = %v, want an empty array", body.Data)
	}
	if !strings.Contains(rec.Body.String(), `"data":[]`) {
		t.Fatalf("body = %s, want the empty array written on the wire", rec.Body.String())
	}
}

// TestQuotaHandler_GetUsageRefusals covers the refusals, each of which is a fact the
// operator can act on rather than a provider's error echoing back.
func TestQuotaHandler_GetUsageRefusals(t *testing.T) {
	cases := []struct {
		name       string
		endpoint   domain.UpstreamEndpoint
		entry      registry.Provider
		wantStatus int
		wantCode   string
	}{
		{
			name:  "an endpoint the gateway does not know is NOT_FOUND",
			entry: publishedEntry("qoder", true, true),
			// Zero endpoint: the stub answers the missing-row error.
			wantStatus: http.StatusNotFound, wantCode: "NOT_FOUND",
		},
		{
			name:     "a provider the registry does not carry is refused",
			endpoint: publishedOAuthEndpoint(t, "ghost", "dt-token"),
			// The index knows qoder, not the ghost the row names: an endpoint
			// written before a provider was dropped from the registry reads as
			// this, and the route must not reach the network for it.
			entry:      publishedEntry("qoder", true, true),
			wantStatus: http.StatusBadRequest, wantCode: "VALIDATION_ERROR",
		},
		{
			name:       "a provider that publishes no usage endpoint is refused",
			endpoint:   publishedOAuthEndpoint(t, "openai", "sk-token"),
			entry:      publishedEntry("openai", false, false),
			wantStatus: http.StatusBadRequest, wantCode: "VALIDATION_ERROR",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := newPublishedHandler(t, tc.endpoint, tc.entry, quotafetch.Result{})
			rec := httptest.NewRecorder()
			handler.GetUsage(rec, quotaRequest(http.MethodGet, "", "ep_pub"))
			if rec.Code != tc.wantStatus {
				t.Fatalf("GET status = %d, want %d (body %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.wantCode) {
				t.Fatalf("body = %s, want code %s", rec.Body.String(), tc.wantCode)
			}
		})
	}
}

// TestQuotaHandler_GetUsageUnwired covers the wiring refusal: a quota service built
// for §7.12 alone has no live read to answer, and the route says so rather than
// rendering an empty card.
func TestQuotaHandler_GetUsageUnwired(t *testing.T) {
	quotas, err := service.NewQuotaService(service.QuotaServiceDeps{
		Quotas: newStubQuotaRepo(), Endpoints: &stubEndpointRepo{known: map[string]bool{"ep_pub": true}},
	})
	if err != nil {
		t.Fatalf("NewQuotaService() error = %v", err)
	}
	rec := httptest.NewRecorder()
	NewQuotaHandler(quotas).GetUsage(rec, quotaRequest(http.MethodGet, "", "ep_pub"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("GET status = %d, want 500 (body %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "INTERNAL_ERROR") {
		t.Fatalf("body = %s, want the INTERNAL_ERROR code", rec.Body.String())
	}
}
