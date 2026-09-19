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
		wantHeader    string
		wantHeaderSet bool
		wantErrorText string
	}{
		{
			name:       "a bearer header carries the key",
			media:      registry.MediaConfig{AuthType: registry.AuthAPIKey, AuthHeader: "bearer"},
			credential: provider.Credential{APIKey: "sk-x"},
			wantHeader: "Bearer sk-x", wantHeaderSet: true,
		},
		{
			name:  "an empty credential sends no bearer header",
			media: registry.MediaConfig{AuthType: registry.AuthAPIKey, AuthHeader: "bearer"},
		},
		{
			name:       "the registry default is a bearer header",
			media:      registry.MediaConfig{AuthType: registry.AuthAPIKey},
			credential: provider.Credential{APIKey: "sk-x"},
			wantHeader: "Bearer sk-x", wantHeaderSet: true,
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
			got, set := headers[provider.DefaultAuthHeader]
			if set != tc.wantHeaderSet {
				t.Fatalf("header %s set = %v (value %q), want set = %v",
					provider.DefaultAuthHeader, set, got, tc.wantHeaderSet)
			}
			if set && got != tc.wantHeader {
				t.Fatalf("header %s = %q, want %q", provider.DefaultAuthHeader, got, tc.wantHeader)
			}
		})
	}
}
