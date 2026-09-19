// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_test.go
// @for       Table-driven tests for starting an authorization (SPEC-API-001
//
//	§7.4 POST .../oauth/start).
//
// @uses      net/url, strings, testing, time.
// @reason    Start is the only place a state and a PKCE verifier are born,
//
//	so the table pins the rules a provider entry must satisfy, the
//	authorize URL's query contract, and the ten-minute staging TTL.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

func TestOAuthStart(t *testing.T) {
	const base = "https://panel.example.com"
	cases := []struct {
		name        string
		providerID  string
		redirectURI string
		wantCode    string
		wantErrPart string
		check       func(t *testing.T, f oauthFlowFixture, started OAuthStart)
	}{
		{
			name:       "code flow builds authorize URL with PKCE and staged state",
			providerID: "pkce-provider",
			check: func(t *testing.T, f oauthFlowFixture, started OAuthStart) {
				parsed, err := url.Parse(started.AuthorizeURL)
				if err != nil {
					t.Fatalf("authorize URL unparseable: %v", err)
				}
				query := parsed.Query()
				if parsed.Scheme != "https" || parsed.Host != "auth.example.com" || parsed.Path != "/pkce-provider/authorize" {
					t.Fatalf("authorize endpoint = %s", started.AuthorizeURL)
				}
				for name, want := range map[string]string{
					"response_type":         "code",
					"client_id":             "client-pkce-provider",
					"redirect_uri":          base + "/api/v1/providers/pkce-provider/oauth/callback",
					"scope":                 "scope:one scope:two",
					"code_challenge_method": "S256",
					"code_challenge":        "", // verified against the verifier below
					"state":                 started.State,
				} {
					if name == "code_challenge" {
						continue
					}
					if got := query.Get(name); got != want {
						t.Fatalf("query[%s] = %q, want %q", name, got, want)
					}
				}
				staged := f.states.staged[started.State]
				if len(staged.payload) == 0 {
					t.Fatalf("state %q was never staged", started.State)
				}
				if f.states.ttl != 10*time.Minute {
					t.Fatalf("staging TTL = %s, want 10m", f.states.ttl)
				}
				payload := decodeStagedState(t, staged.payload)
				if len(payload.Verifier) < 43 {
					t.Fatalf("PKCE verifier = %d chars, want at least 43", len(payload.Verifier))
				}
				if query.Get("code_challenge") != pkceChallenge(payload.Verifier) {
					t.Fatalf("code_challenge does not match S256(verifier)")
				}
				if payload.RedirectURI != base+"/api/v1/providers/pkce-provider/oauth/callback" {
					t.Fatalf("staged redirect = %q", payload.RedirectURI)
				}
			},
		},
		{
			name:        "explicit redirect_uri replaces the derived one",
			providerID:  "pkce-provider",
			redirectURI: "https://panel.example.com/settings/providers",
			check: func(t *testing.T, f oauthFlowFixture, started OAuthStart) {
				payload := decodeStagedState(t, f.states.staged[started.State].payload)
				if payload.RedirectURI != "https://panel.example.com/settings/providers" {
					t.Fatalf("staged redirect = %q", payload.RedirectURI)
				}
			},
		},
		{
			name:       "provider without scopes omits the scope parameter",
			providerID: "bare-provider",
			check: func(t *testing.T, f oauthFlowFixture, started OAuthStart) {
				parsed, _ := url.Parse(started.AuthorizeURL)
				if got := parsed.Query().Get("scope"); got != "" {
					t.Fatalf("scope = %q, want absent", got)
				}
				if got := parsed.Query().Get("code_challenge_method"); got != "" {
					t.Fatalf("PKCE applied without a declared method: %q", got)
				}
			},
		},
		{
			name:       "provider extra params ride along without overriding core params",
			providerID: "extra-params-provider",
			check: func(t *testing.T, f oauthFlowFixture, started OAuthStart) {
				parsed, _ := url.Parse(started.AuthorizeURL)
				query := parsed.Query()
				if got := query.Get("prompt"); got != "consent" {
					t.Fatalf("extra param prompt = %q, want consent", got)
				}
				if got := query.Get("response_type"); got != "code" {
					t.Fatalf("extra params overrode response_type: %q", got)
				}
			},
		},
		{name: "unknown provider", providerID: "nope", wantCode: "VALIDATION_ERROR"},
		{
			name:        "provider without an oauth block",
			providerID:  "plain-provider",
			wantCode:    "VALIDATION_ERROR",
			wantErrPart: "oauth",
		},
		{
			name:        "device-flow provider refuses the generic flow",
			providerID:  "device-provider",
			wantCode:    "VALIDATION_ERROR",
			wantErrPart: "device",
		},
		{
			name:        "connector-required provider refuses the generic flow",
			providerID:  "connector-provider",
			wantCode:    "VALIDATION_ERROR",
			wantErrPart: "connector",
		},
		{
			name:        "non-http scheme redirect refused",
			providerID:  "pkce-provider",
			redirectURI: "javascript:alert(1)",
			wantCode:    "VALIDATION_ERROR",
		},
		{
			name:        "file scheme redirect refused",
			providerID:  "pkce-provider",
			redirectURI: "file:///etc/passwd",
			wantCode:    "VALIDATION_ERROR",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newOAuthFlowFixture(t,
				providerWithCodeFlow("pkce-provider"),
				providerBareCodeFlow("bare-provider"),
				providerWithIdentity("identity-provider"),
				providerWithDeviceFlow("device-provider"),
				providerNeedingConnector("connector-provider"),
				bareProvider("plain-provider"),
				extraParamsProvider("extra-params-provider"),
			)
			started, err := fixture.service.Start(context.Background(), OAuthStartInput{
				ProviderID: tc.providerID, RedirectURI: tc.redirectURI, BaseURL: base,
			})
			if tc.wantCode != "" {
				mustAppError(t, err, tc.wantCode)
				if tc.wantErrPart != "" && !strings.Contains(err.Error(), tc.wantErrPart) {
					t.Fatalf("error %q does not mention %q", err.Error(), tc.wantErrPart)
				}
				return
			}
			if err != nil {
				t.Fatalf("Start() error = %v", err)
			}
			if tc.check != nil {
				tc.check(t, fixture, started)
			}
		})
	}
}

func TestOAuthStartStateStagingIsSingleUse(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithCodeFlow("pkce-provider"))
	ctx := context.Background()
	first, err := fixture.service.Start(ctx, OAuthStartInput{ProviderID: "pkce-provider", BaseURL: "https://panel.example.com"})
	if err != nil {
		t.Fatalf("first start: %v", err)
	}
	if _, taken, err := fixture.states.Take(ctx, first.State); err != nil || !taken {
		t.Fatalf("Take() = (%v, %v), want the staged payload", taken, err)
	}
	// The take removed the state, so a second callback with the same value is
	// a replay and must find nothing.
	if _, taken, err := fixture.states.Take(ctx, first.State); err != nil || taken {
		t.Fatalf("replay Take() = (%v, %v), want taken=false", taken, err)
	}
}

// bareProvider mirrors a vendor that only sells API keys.
func bareProvider(id string) registry.Provider {
	return registry.Provider{ID: id, Transport: registry.Transport{Format: "openai"}}
}

// providerBareCodeFlow declares a code flow with no scopes and no PKCE, so a
// provider that never declared either gets neither query parameter.
func providerBareCodeFlow(id string) registry.Provider {
	provider := providerWithCodeFlow(id)
	provider.OAuth.Scopes = nil
	provider.OAuth.CodeChallenge = ""
	return provider
}

// extraParamsProvider declares provider-specific authorize parameters.
func extraParamsProvider(id string) registry.Provider {
	provider := providerWithCodeFlow(id)
	provider.OAuth.ExtraParams = map[string]string{"prompt": "consent"}
	return provider
}
