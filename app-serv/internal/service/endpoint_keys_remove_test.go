// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_keys_remove_test.go
// @for       Key removal against unknown ids and the keep-one-credential rule.
// @uses      testing, internal/domain.
// @reason    SPEC-API-001 §7.5 refuses to strip an api_key endpoint of its last
//
//	usable credential, which is the invariant the aggregate cannot
//	check from a single key.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04

package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestEndpointService_UpdateKeyOnUnknownIDs(t *testing.T) {
	svc, _ := newEndpointSvc(t)
	ctx := context.Background()
	endpoint := keyedEndpoint(t, svc, "primary")

	cases := []struct {
		name       string
		endpointID string
		keyID      string
		wantCode   string
	}{
		{"an unknown key is not found", endpoint.ID(), "uky_absent", "NOT_FOUND"},
		{"an unknown endpoint is not found", "ep_absent", endpoint.Keys()[0].ID(), "NOT_FOUND"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.UpdateKey(ctx, tc.endpointID, tc.keyID, KeyPatch{Label: strPtr("x")})
			mustAppError(t, err, tc.wantCode)
		})
	}
}

// TestEndpointService_RemoveKeyKeepsOneActiveCredential is the §7.5 invariant: an
// api_key endpoint must always retain at least one usable credential, and the refusal
// is a CONFLICT the panel anticipates.
func TestEndpointService_RemoveKeyKeepsOneActiveCredential(t *testing.T) {
	cases := []struct {
		name      string
		authType  domain.UpstreamAuthType
		labels    []string
		removeAll bool
		wantCode  string
		wantKept  int
	}{
		{name: "an api_key endpoint with two keys may drop one", authType: domain.UpstreamAuthAPIKey,
			labels: []string{"a", "b"}, wantKept: 1},
		{name: "an api_key endpoint refuses to drop its last key", authType: domain.UpstreamAuthAPIKey,
			labels: []string{"only"}, removeAll: true, wantCode: "CONFLICT", wantKept: 1},
		{name: "an oauth endpoint may hold no keys", authType: domain.UpstreamAuthOAuth, labels: nil, wantKept: 0},
		{name: "a no_auth endpoint may hold no keys", authType: domain.UpstreamAuthNone, labels: nil, wantKept: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newEndpointSvc(t)
			ctx := context.Background()
			endpoint := seedEndpoint(t, svc, "deepseek", "acct", tc.authType, toKeyInputs(tc.labels)...)

			var err error
			switch {
			case tc.removeAll:
				for len(endpoint.Keys()) > 0 {
					keys := endpoint.Keys()
					err = svc.RemoveKey(ctx, endpoint.ID(), keys[0].ID())
					if err != nil {
						break
					}
					reloaded, getErr := svc.Get(ctx, endpoint.ID())
					if getErr != nil {
						t.Fatal(getErr)
					}
					endpoint = reloaded
				}
			case len(tc.labels) > 0:
				err = svc.RemoveKey(ctx, endpoint.ID(), endpoint.Keys()[0].ID())
			default:
				err = svc.RemoveKey(ctx, endpoint.ID(), "uky_absent")
				tc.wantCode = "NOT_FOUND"
			}

			if tc.wantCode != "" {
				mustAppError(t, err, tc.wantCode)
			} else if err != nil {
				t.Fatalf("RemoveKey() error = %v", err)
			}

			current, getErr := svc.Get(ctx, endpoint.ID())
			if getErr != nil {
				t.Fatal(getErr)
			}
			if len(current.Keys()) != tc.wantKept {
				t.Fatalf("keys = %d, want %d", len(current.Keys()), tc.wantKept)
			}
		})
	}
}
