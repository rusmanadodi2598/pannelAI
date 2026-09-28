// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/quota_usage_test.go
// @for       Table-driven tests for the published-quota read: which credential it
//
//	asks with, what it refuses before calling, and how a provider's
//	buckets become windows.
//
// @uses      context, errors, internal/domain, internal/registry,
//
//	internal/service/quotafetch, testing, time.
//
// @reason    The read reaches the network through one seam, so the whole routing
//
//	decision — the feature gate, the credential kind, the ceiling that is
//	not a zero — is testable without a provider. Draft 036 §6 is the
//	definition of done: a spent account and an unlimited account must
//	not render as the same card.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-28
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/quotafetch"
)

// TestQuotaService_PublishedUsageRefusals covers what the read refuses before it
// reaches the network: each is a fact the operator can act on, and none may cost a
// provider call.
func TestQuotaService_PublishedUsageRefusals(t *testing.T) {
	storeDown := errors.New("storage is down")
	keyEntry := usageEntry("qoder", true)
	cases := []struct {
		name      string
		endpoint  domain.UpstreamEndpoint
		lookErr   error
		entry     registry.Provider
		id        string
		wantCode  string
		wantRaw   error
		wantCalls int
	}{
		{
			name:  "an empty endpoint id is a validation failure",
			entry: keyEntry, id: "   ",
			wantCode: "VALIDATION_ERROR",
		},
		{
			name:    "an endpoint the gateway does not know is not found",
			lookErr: domain.ErrEndpointNotFound,
			entry:   keyEntry, id: "ep_missing",
			wantCode: "NOT_FOUND",
		},
		{
			name:    "a storage failure is not reported as a missing endpoint",
			lookErr: storeDown,
			entry:   keyEntry, id: "ep_pub", wantRaw: storeDown,
		},
		{
			name:     "a provider that publishes no usage endpoint is refused by name",
			endpoint: keyPublishedEndpoint(t, "openai", "sk-x"),
			entry:    registry.Provider{ID: "openai"},
			id:       "ep_pub", wantCode: "VALIDATION_ERROR",
		},
		{
			name:     "a key on a provider whose usage endpoint wants a token is refused",
			endpoint: keyPublishedEndpoint(t, "qoder", "pt-x"),
			entry:    usageEntry("qoder", false),
			id:       "ep_pub", wantCode: "VALIDATION_ERROR",
		},
		{
			name:     "an oauth account with no stored token is refused",
			endpoint: oauthPublishedEndpoint(t, "qoder", ""),
			entry:    keyEntry, id: "ep_pub",
			wantCode: "VALIDATION_ERROR",
		},
		{
			name:     "an oauth account with no credential row at all is refused",
			endpoint: domain.RehydrateUpstreamEndpoint("ep_pub", "qoder", "acct@example.com", domain.UpstreamAuthOAuth, 1, domain.UpstreamEndpointActive, nil, domain.EndpointAccount{}, domain.EndpointTestStatus{}, nil, nil, testNow, testNow, nil, domain.EndpointParity{}),
			entry:    keyEntry, id: "ep_pub",
			wantCode: "VALIDATION_ERROR",
		},
		{
			name:     "a key account with no key at all is refused",
			endpoint: domain.RehydrateUpstreamEndpoint("ep_pub", "qoder", "empty", domain.UpstreamAuthAPIKey, 1, domain.UpstreamEndpointActive, nil, domain.EndpointAccount{}, domain.EndpointTestStatus{}, nil, nil, testNow, testNow, nil, domain.EndpointParity{}),
			entry:    keyEntry, id: "ep_pub",
			wantCode: "VALIDATION_ERROR",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			quotas, fetcher := newPublishedFixture(t, tc.endpoint, tc.lookErr, tc.entry, quotafetch.Result{})
			_, err := quotas.PublishedUsage(context.Background(), tc.id)
			if tc.wantRaw != nil {
				// The store's own failure travels: a read that cannot find the
				// row is missing, one that cannot ask is not, and folding the
				// second into the first would tell the operator to fix data.
				if !errors.Is(err, tc.wantRaw) {
					t.Fatalf("PublishedUsage() error = %v, want the storage failure itself", err)
				}
			} else {
				var appErr *domain.AppError
				if !errors.As(err, &appErr) {
					t.Fatalf("PublishedUsage() error = %v, want an app error", err)
				}
				if appErr.Code != tc.wantCode {
					t.Fatalf("code = %q, want %q (message %q)", appErr.Code, tc.wantCode, appErr.Message)
				}
			}
			if fetcher.calls != tc.wantCalls {
				t.Fatalf("provider calls = %d, want %d", fetcher.calls, tc.wantCalls)
			}
		})
	}
}

// TestQuotaService_PublishedUsageCredential pins which half of the credential pair
// the read fills: an account token must not arrive as a key, or a provider would be
// asked with a bearer it did not sell.
func TestQuotaService_PublishedUsageCredential(t *testing.T) {
	cases := []struct {
		name       string
		endpoint   domain.UpstreamEndpoint
		providerID string
		wantAccess string
		wantAPIKey string
	}{
		{
			name:       "an oauth account asks with its access token",
			endpoint:   oauthPublishedEndpoint(t, "qoder", "dt-device-token"),
			providerID: "qoder", wantAccess: "dt-device-token",
		},
		{
			name:       "a key account asks with its key",
			endpoint:   keyPublishedEndpoint(t, "qoder", "pt-personal-token"),
			providerID: "qoder", wantAPIKey: "pt-personal-token",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			quotas, fetcher := newPublishedFixture(t, tc.endpoint, nil,
				usageEntry(tc.providerID, true), quotafetch.Result{Plan: "pro"})
			usage, err := quotas.PublishedUsage(context.Background(), "ep_pub")
			if err != nil {
				t.Fatalf("PublishedUsage() error = %v", err)
			}
			if fetcher.calls != 1 {
				t.Fatalf("provider calls = %d, want 1", fetcher.calls)
			}
			if fetcher.family != tc.providerID {
				t.Fatalf("family = %q, want %q", fetcher.family, tc.providerID)
			}
			if fetcher.creds.AccessToken != tc.wantAccess || fetcher.creds.APIKey != tc.wantAPIKey {
				t.Fatalf("credentials = %+v, want access %q key %q", fetcher.creds, tc.wantAccess, tc.wantAPIKey)
			}
			if usage.EndpointID != "ep_pub" || usage.ProviderID != tc.providerID || usage.Plan != "pro" {
				t.Fatalf("usage = %+v, want the endpoint's own identity and plan", usage)
			}
		})
	}
}

// TestQuotaService_PublishedUsageWindows covers how the provider's buckets become
// windows, including the two shapes a card must not confuse: an unbounded bucket and
// one that states a refill.
func TestQuotaService_PublishedUsageWindows(t *testing.T) {
	resets := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	quotas, fetcher := newPublishedFixture(t,
		oauthPublishedEndpoint(t, "qoder", "dt-token"), nil, usageEntry("qoder", true),
		quotafetch.Result{
			Quotas: []quotafetch.Quota{
				{Label: "  Resource Pack  ", Used: 12.5, Total: 3000, ResetAt: resets},
				{Label: "Used (USD)", Used: 4, Unlimited: true},
				{Label: "Silent", Used: 0, Total: 0},
			},
		})

	usage, err := quotas.PublishedUsage(context.Background(), "ep_pub")
	if err != nil {
		t.Fatalf("PublishedUsage() error = %v", err)
	}
	if len(usage.Windows) != 3 {
		t.Fatalf("windows = %d, want 3 (%+v)", len(usage.Windows), usage.Windows)
	}
	first := usage.Windows[0]
	if first.Label != "Resource Pack" {
		t.Fatalf("label = %q, want the trimmed provider label", first.Label)
	}
	if !first.HasTotal || first.Total != 3000 || first.Used != 12.5 {
		t.Fatalf("first window = %+v, want 12.5 of 3000", first)
	}
	if first.ResetsAt == nil || !first.ResetsAt.Equal(resets) {
		t.Fatalf("resets_at = %v, want %s", first.ResetsAt, resets)
	}
	if usage.Windows[1].HasTotal {
		t.Fatalf("unlimited window = %+v, want no ceiling", usage.Windows[1])
	}
	if usage.Windows[2].ResetsAt != nil {
		t.Fatalf("window without a reset = %+v, want nil, not the zero instant", usage.Windows[2])
	}
	if usage.FetchedAt.IsZero() || fetcher.calls != 1 {
		t.Fatalf("usage = %+v calls = %d, want a stamped read that called once", usage.FetchedAt, fetcher.calls)
	}
}

// TestQuotaService_PublishedUsageSoftResult pins that a provider which answers with
// a sentence rather than buckets is not a failed read.
func TestQuotaService_PublishedUsageSoftResult(t *testing.T) {
	quotas, _ := newPublishedFixture(t, oauthPublishedEndpoint(t, "qoder", "dt-token"), nil,
		usageEntry("qoder", true),
		quotafetch.Result{Message: "Qoder credential not available."})

	usage, err := quotas.PublishedUsage(context.Background(), "ep_pub")
	if err != nil {
		t.Fatalf("PublishedUsage() error = %v, want the soft answer", err)
	}
	if usage.Message != "Qoder credential not available." {
		t.Fatalf("message = %q, want the provider's own sentence", usage.Message)
	}
	if len(usage.Windows) != 0 {
		t.Fatalf("windows = %+v, want none beside a message", usage.Windows)
	}
}

// TestQuotaService_PublishedUsageUnwired covers the wiring refusal: a quota service
// built for §7.12 alone keeps every counted read and has no live read to answer.
func TestQuotaService_PublishedUsageUnwired(t *testing.T) {
	quotas, _ := newQuotaServiceFixture(t, &stubEndpointLookup{known: map[string]bool{"ep_pub": true}})
	_, err := quotas.PublishedUsage(context.Background(), "ep_pub")
	var appErr *domain.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("PublishedUsage() error = %v, want an app error", err)
	}
	if appErr.Code != "INTERNAL_ERROR" {
		t.Fatalf("code = %q, want INTERNAL_ERROR for a read with no collaborators", appErr.Code)
	}
}
