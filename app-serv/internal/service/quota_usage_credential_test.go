// The credential the published-quota read hands the fetcher (docs/SPEC-API/001-SPEC-API.md
// §7.12's published-read block, docs/PORT/010-PORT-QUOTA-PUBLISHED.md F6).
//
// @file      internal/service/quota_usage_credential_test.go
// @for       Which secret and which account facts a published read asks its provider with.
// @uses      context, internal/domain, internal/service/quotafetch, testing.
// @reason    The mapper from an endpoint's stored identity to a provider's account facts is unit-tested where it lives, which cannot show that the credential builder calls it.
//
//	This file reads through the seam the fetcher is faked at, so a family left to discover
//	its own project by paying for a bootstrap call per poll fails here rather than in production.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/quotafetch"
)

// TestQuotaService_PublishedUsageCredential pins which half of the credential pair
// the read fills: an account token must not arrive as a key, or a provider would be
// asked with a bearer it did not sell. It also pins that the account's own facts,
// the project a cloudcode family meters, the email a keyless provider bills by,
// reach the fetcher, because the mapper for them is unit-tested on its own and a
// credential builder that never called it would leave every such family paying a
// second provider call per poll to discover what this gateway already stores.
func TestQuotaService_PublishedUsageCredential(t *testing.T) {
	cases := []struct {
		name       string
		endpoint   domain.UpstreamEndpoint
		providerID string
		wantAccess string
		wantAPIKey string
		wantFacts  map[string]string
	}{
		{
			name:       "an oauth account asks with its access token",
			endpoint:   oauthPublishedEndpoint(t, "qoder", "dt-device-token"),
			providerID: "qoder", wantAccess: "dt-device-token",
			wantFacts: map[string]string{
				"projectId": "proj-pub", "email": "acct@example.com", "userId": "user-pub",
			},
		},
		{
			name:       "a key account asks with its key",
			endpoint:   keyPublishedEndpoint(t, "qoder", "pt-personal-token"),
			providerID: "qoder", wantAPIKey: "pt-personal-token",
			wantFacts: map[string]string{"email": "key@example.com"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			quotas, fetcher := newPublishedFixture(t, tc.endpoint, nil,
				usageEntry(tc.providerID, true), quotafetch.Result{Plan: "pro"})
			usage, err := quotas.PublishedUsage(context.Background(), "ep_pub", true)
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
			if len(fetcher.creds.ProviderSpecificData) != len(tc.wantFacts) {
				t.Fatalf("account facts = %v, want %v", fetcher.creds.ProviderSpecificData, tc.wantFacts)
			}
			for key, want := range tc.wantFacts {
				if got := fetcher.creds.ProviderSpecificData[key]; got != want {
					t.Fatalf("account fact %s = %q, want %q", key, got, want)
				}
			}
			if usage.EndpointID != "ep_pub" || usage.ProviderID != tc.providerID || usage.Plan != "pro" {
				t.Fatalf("usage = %+v, want the endpoint's own identity and plan", usage)
			}
		})
	}
}
