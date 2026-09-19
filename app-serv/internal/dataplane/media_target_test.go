// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/media_target_test.go
// @for       The credential placement every media call shares (§8.1).
// @uses      internal/provider, internal/registry, strings, testing.
// @reason    G16 in the P2 register: the media path wrote
//
//	`Authorization: Bearer` with an empty secret where the chat path
//	sends no header at all, so one account presented two credential
//	rules. These rows pin all three placement branches and both
//	credential states, because the branch that quietly adds a header is
//	the one no live pass notices.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestMediaTarget_CredentialPlacement pins where a media call's credential goes,
// and that an empty one is not placed at all.
func TestMediaTarget_CredentialPlacement(t *testing.T) {
	cases := []struct {
		name          string
		media         registry.MediaConfig
		credential    provider.Credential
		wantQuery     string
		wantHeaders   map[string]string
		wantErrorText string
	}{
		{
			name:        "a bearer header carries the key",
			media:       registry.MediaConfig{AuthType: registry.AuthAPIKey, AuthHeader: "bearer"},
			credential:  provider.Credential{APIKey: "sk-x"},
			wantHeaders: map[string]string{"Authorization": "Bearer sk-x"},
		},
		{
			name:  "an empty credential sends no bearer header",
			media: registry.MediaConfig{AuthType: registry.AuthAPIKey, AuthHeader: "bearer"},
		},
		{
			name:        "a token header uses the Token scheme",
			media:       registry.MediaConfig{AuthType: registry.AuthAPIKey, AuthHeader: "token"},
			credential:  provider.Credential{APIKey: "dg-x"},
			wantHeaders: map[string]string{"Authorization": "Token dg-x"},
		},
		{
			name:  "an empty token credential sends no Authorization header",
			media: registry.MediaConfig{AuthType: registry.AuthAPIKey, AuthHeader: "token"},
		},
		{
			name:        "the registry default is a bearer header",
			media:       registry.MediaConfig{AuthType: registry.AuthAPIKey},
			credential:  provider.Credential{APIKey: "sk-x"},
			wantHeaders: map[string]string{"Authorization": "Bearer sk-x"},
		},
		{
			name:  "the registry default sends nothing without a credential",
			media: registry.MediaConfig{AuthType: registry.AuthAPIKey},
		},
		{
			name:       "a query parameter carries the key",
			media:      registry.MediaConfig{AuthType: registry.AuthAPIKey, AuthHeader: "key"},
			credential: provider.Credential{APIKey: "sk-x"},
			wantQuery:  "key=sk-x",
		},
		{
			name:          "a query parameter without a credential is refused",
			media:         registry.MediaConfig{AuthType: registry.AuthAPIKey, AuthHeader: "key"},
			wantErrorText: "no credential",
		},
		{
			name:       "a no_auth kind sends nothing even with a credential",
			media:      registry.MediaConfig{AuthType: registry.AuthNone},
			credential: provider.Credential{APIKey: "sk-x"},
		},
		{
			name:        "a basic header carries the encoded pair",
			media:       registry.MediaConfig{AuthType: registry.AuthAPIKey, AuthHeader: "basic"},
			credential:  provider.Credential{APIKey: "dXNlcjpwYXNz"},
			wantHeaders: map[string]string{"Authorization": "Basic dXNlcjpwYXNz"},
		},
		{
			name:  "an empty basic credential sends no Authorization header",
			media: registry.MediaConfig{AuthType: registry.AuthAPIKey, AuthHeader: "basic"},
		},
		{
			name:       "a playht pair splits into the user id and the bearer",
			media:      registry.MediaConfig{AuthType: registry.AuthAPIKey, AuthHeader: "playht"},
			credential: provider.Credential{APIKey: "user_1:ph_key"},
			wantHeaders: map[string]string{
				"X-USER-ID": "user_1", "Authorization": "Bearer ph_key",
			},
		},
		{
			name:        "a playht value without a pair is sent as a bearer alone",
			media:       registry.MediaConfig{AuthType: registry.AuthAPIKey, AuthHeader: "playht"},
			credential:  provider.Credential{APIKey: "ph_key"},
			wantHeaders: map[string]string{"Authorization": "Bearer ph_key"},
		},
		{
			name:  "an empty playht credential sends no header",
			media: registry.MediaConfig{AuthType: registry.AuthAPIKey, AuthHeader: "playht"},
		},
		{
			name:        "a declared header name carries the raw key",
			media:       registry.MediaConfig{AuthType: registry.AuthAPIKey, AuthHeader: "xi-api-key"},
			credential:  provider.Credential{APIKey: "el-x"},
			wantHeaders: map[string]string{"Xi-Api-Key": "el-x"},
		},
		{
			name:  "a declared header name sends nothing without a credential",
			media: registry.MediaConfig{AuthType: registry.AuthAPIKey, AuthHeader: "xi-api-key"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target, headers, err := MediaTarget(tc.media, "https://api.example.com/v1", tc.credential, nil)
			if tc.wantErrorText != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrorText) {
					t.Fatalf("error = %v, want it to mention %q", err, tc.wantErrorText)
				}
				return
			}
			if err != nil {
				t.Fatalf("MediaTarget() error = %v", err)
			}
			if tc.wantQuery != "" && !strings.Contains(target, "?"+tc.wantQuery) {
				t.Fatalf("target = %q, want it to carry %q", target, tc.wantQuery)
			}
			placed := map[string]string{}
			for name, value := range headers {
				if name != "Content-Type" {
					placed[name] = value
				}
			}
			if len(placed) != len(tc.wantHeaders) {
				t.Fatalf("credential headers = %v, want %v", placed, tc.wantHeaders)
			}
			for name, value := range tc.wantHeaders {
				if placed[name] != value {
					t.Fatalf("header %s = %q, want %q", name, placed[name], value)
				}
			}
		})
	}
}

// TestMediaPath_KeepsTheCredentialQuery pins the one URL rule the provider
// adapters that name a model or a voice in their path depend on: the suffix
// lands on the path, not after the query the credential placement added.
func TestMediaPath_KeepsTheCredentialQuery(t *testing.T) {
	target, err := MediaPath("https://generativelanguage.googleapis.com/v1beta/models?key=k1", "/gemini-2.5-flash-preview-tts:generateContent")
	if err != nil {
		t.Fatalf("MediaPath() error = %v", err)
	}
	want := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash-preview-tts:generateContent?key=k1"
	if target != want {
		t.Fatalf("target = %q, want %q", target, want)
	}

	if _, err := MediaPath("://not a url", "/x"); err == nil {
		t.Fatal("MediaPath() = nil error, want a malformed target to be refused")
	}
}
