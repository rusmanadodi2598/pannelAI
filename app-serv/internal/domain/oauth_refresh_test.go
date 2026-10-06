// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/oauth_refresh_test.go
// @for       Table-driven tests for the token freshness rules and the unhealthy transition.
// @uses      testing, time.
// @reason    The status route and the refresh worker must agree on "due", so the boundary rows (exact lead instant, expired, no expiry) are pinned here rather than in either caller's tests.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-09-19
package domain

import (
	"testing"
	"time"
)

func TestOAuthRefreshState(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	lead := time.Hour
	cases := []struct {
		name string
		cred *OAuthCredential
		lead time.Duration
		want string
	}{
		{"nil credential", nil, lead, RefreshMissing},
		{"no expiry recorded", &OAuthCredential{}, lead, RefreshMissing},
		{"far from expiry", &OAuthCredential{expiresAt: ptrTime(now.Add(6 * time.Hour))}, lead, RefreshFresh},
		{"exact lead boundary is due", &OAuthCredential{expiresAt: ptrTime(now.Add(time.Hour))}, lead, RefreshDue},
		{"inside lead window", &OAuthCredential{expiresAt: ptrTime(now.Add(10 * time.Minute))}, lead, RefreshDue},
		{"already expired", &OAuthCredential{expiresAt: ptrTime(now.Add(-time.Minute))}, lead, RefreshDue},
		{"zero lead falls back to default", &OAuthCredential{expiresAt: ptrTime(now.Add(10 * time.Minute))}, 0, RefreshDue},
		{"zero lead far expiry is fresh", &OAuthCredential{expiresAt: ptrTime(now.Add(DefaultRefreshLead + time.Minute))}, 0, RefreshFresh},
		{"negative lead falls back to default", &OAuthCredential{expiresAt: ptrTime(now.Add(time.Minute))}, -time.Hour, RefreshDue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := OAuthRefreshState(tc.cred, tc.lead, now); got != tc.want {
				t.Fatalf("state = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestOAuthCredentialRotatedKeepsAnExpiryTheGrantOmits(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	held := ptrTime(now.Add(2 * time.Hour))
	replacement := ptrTime(now.Add(3 * time.Hour))
	cases := []struct {
		name       string
		grant      *time.Time
		wantExpiry *time.Time
	}{
		{"a grant without expires_in keeps the expiry the account holds", nil, held},
		{"a grant with expires_in replaces it", replacement, replacement},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cred := OAuthCredential{
				accessTokenEncrypted:  "old-access",
				refreshTokenEncrypted: "old-refresh",
				expiresAt:             held,
			}
			rotated, err := cred.Rotated("new-access", "", tc.grant, now)
			if err != nil {
				t.Fatalf("Rotated() error = %v", err)
			}
			got := rotated.ExpiresAt()
			if got == nil || *got != *tc.wantExpiry {
				t.Fatalf("expiry = %v, want %v", got, tc.wantExpiry)
			}
			// A nil expiry is RefreshMissing, and the sweep only renews RefreshDue,
			// so losing the expiry silently retires the account.
			if state := OAuthRefreshState(&rotated, time.Hour, now); state == RefreshMissing {
				t.Fatalf("state = %q: the account left the refresh sweep", state)
			}
			if refreshed := rotated.RefreshTokenEncrypted(); refreshed != "old-refresh" {
				t.Fatalf("refresh token = %q, want the one the account already held", refreshed)
			}
		})
	}
}

func TestMarkUnhealthySetsErrorStatusAndReason(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		start   UpstreamEndpointStatus
		message string
	}{
		{"from active", UpstreamEndpointActive, "refresh failed: token refused"},
		{"from disabled", UpstreamEndpointDisabled, ""},
		{"empty message still records", UpstreamEndpointActive, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint, err := NewUpstreamEndpoint("ep_test", "provider", "acc", UpstreamAuthOAuth, 1, now)
			if err != nil {
				t.Fatalf("building endpoint: %v", err)
			}
			if tc.start == UpstreamEndpointDisabled {
				if err := endpoint.Update("acc", 1, string(UpstreamEndpointDisabled), now); err != nil {
					t.Fatalf("disabling: %v", err)
				}
			}
			endpoint.MarkUnhealthy(tc.message, now)
			if endpoint.Status() != UpstreamEndpointError {
				t.Fatalf("status = %q, want error", endpoint.Status())
			}
			if got := endpoint.TestStatus().State(); got != EndpointTestFail {
				t.Fatalf("test state = %q, want fail", got)
			}
			if endpoint.TestStatus().Message() != tc.message {
				t.Fatalf("message = %q, want %q", endpoint.TestStatus().Message(), tc.message)
			}
		})
	}
}
