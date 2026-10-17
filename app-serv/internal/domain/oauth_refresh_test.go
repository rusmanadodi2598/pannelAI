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
		want    UpstreamEndpointStatus
		message string
	}{
		{"from active", UpstreamEndpointActive, UpstreamEndpointError, "refresh failed: token refused"},
		{"from disabled the operator keeps the state", UpstreamEndpointDisabled, UpstreamEndpointDisabled, ""},
		{"empty message still records", UpstreamEndpointActive, UpstreamEndpointError, ""},
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
			if endpoint.Status() != tc.want {
				t.Fatalf("status = %q, want %q", endpoint.Status(), tc.want)
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

// TestUpstreamEndpoint_MarkHealthyReturnsOnlyFromError is the counterpart of the
// dead-letter above. It is limited to `error` because `disabled` is an operator
// decision and health tracking has no authority to reverse it.
func TestUpstreamEndpoint_MarkHealthyReturnsOnlyFromError(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		start     UpstreamEndpointStatus
		wantState UpstreamEndpointStatus
	}{
		{"an error endpoint returns to active", UpstreamEndpointError, UpstreamEndpointActive},
		{"an active endpoint is left alone", UpstreamEndpointActive, UpstreamEndpointActive},
		{"a disabled endpoint is not revived", UpstreamEndpointDisabled, UpstreamEndpointDisabled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint, err := NewUpstreamEndpoint("ep_test", "provider", "acc", UpstreamAuthOAuth, 1, now)
			if err != nil {
				t.Fatalf("building endpoint: %v", err)
			}
			switch tc.start {
			case UpstreamEndpointDisabled:
				if err := endpoint.Update("acc", 1, string(UpstreamEndpointDisabled), now); err != nil {
					t.Fatalf("disabling: %v", err)
				}
			case UpstreamEndpointError:
				endpoint.MarkUnhealthy("the refresh died", now)
			}
			endpoint.MarkHealthy(now)
			if got := endpoint.Status(); got != tc.wantState {
				t.Fatalf("status = %q, want %q", got, tc.wantState)
			}
		})
	}
}

// TestUpstreamEndpoint_MarkHealthyKeepsTheLastTestResult pins that recovery is
// about routability, not a connectivity claim: the refresh never probed the
// upstream, so the recorded test result stays what the last probe said.
func TestUpstreamEndpoint_MarkHealthyKeepsTheLastTestResult(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	endpoint, err := NewUpstreamEndpoint("ep_test", "provider", "acc", UpstreamAuthOAuth, 1, now)
	if err != nil {
		t.Fatalf("building endpoint: %v", err)
	}
	endpoint.MarkUnhealthy("the refresh died", now)
	endpoint.MarkHealthy(now)
	if got := endpoint.TestStatus().State(); got != EndpointTestFail {
		t.Fatalf("test state = %q, want the dead-letter's fail to remain", got)
	}
}
