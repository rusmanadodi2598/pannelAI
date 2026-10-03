// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_identity_test.go
// @for       The identity a Qoder request is signed as: where the user id comes from,
//
//	what is cached, and what is refused.
//
// @uses      context, encoding/json, internal/registry, net/http, net/http/httptest,
//
//	strings, testing, time.
//
// @reason    A Personal Access Token pasted into the panel is only a token: nothing on
//
//	the connection row names the account behind it, and the vendor refuses a
//	signature whose `uid` is empty. So the resolution rule — stored id first,
//	vendor read second, refusal when neither answers — is what decides whether
//	a PAT connection works at all, and it was missing: every request from such
//	a connection died as a shaping failure. These cases drive a stub rather
//	than the vendor so the cache and the refusals are observable.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-28
package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// identityStub answers /api/v1/userinfo with what the case declares and counts the
// requests that reached it, which is how "cached" and "never asked" are measured.
type identityStub struct {
	server   *httptest.Server
	status   int
	payload  map[string]any
	requests int
	seenAuth []string
}

func newIdentityStub(t *testing.T, status int, payload map[string]any) *identityStub {
	t.Helper()
	if status == 0 {
		status = http.StatusOK
	}
	stub := &identityStub{status: status, payload: payload}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.requests++
		stub.seenAuth = append(stub.seenAuth, r.Header.Get("Authorization"))
		if stub.status != http.StatusOK {
			w.WriteHeader(stub.status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(stub.payload)
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

// newIdentityConnector builds a connector whose userinfo endpoint is the stub.
func newIdentityConnector(t *testing.T, stubURL string) *Qoder {
	t.Helper()
	connector, err := NewQoder(registry.Provider{
		ID:        "qoder",
		Transport: registry.Transport{Format: "openai", BaseURL: "https://api3.qoder.sh/algo/x"},
		OAuth:     &registry.OAuth{OpenAPIBaseURL: "https://openapi.qoder.sh", UserInfoURL: stubURL},
	}, http.DefaultClient)
	if err != nil {
		t.Fatalf("NewQoder() error = %v", err)
	}
	return connector
}

func TestQoderSigningIdentity(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		payload    map[string]any
		cred       Credential
		wantUser   string
		wantEmail  string
		wantReqs   int
		wantErrSub string
	}{
		{
			name:     "a stored user id is used and the vendor is never asked",
			cred:     Credential{ProjectID: "user-stored", Account: "stored@example.com"},
			wantUser: "user-stored", wantEmail: "stored@example.com", wantReqs: 0,
		},
		{
			name:     "a token with no stored id reads its identity from the vendor",
			payload:  map[string]any{"id": "user-from-vendor", "email": "vendor@example.com"},
			cred:     Credential{},
			wantUser: "user-from-vendor", wantEmail: "vendor@example.com", wantReqs: 1,
		},
		{
			name:     "the userId spelling is read when id is absent",
			payload:  map[string]any{"userId": "user-camel"},
			cred:     Credential{},
			wantUser: "user-camel", wantReqs: 1,
		},
		{
			name:     "the user_id spelling is read when the other two are absent",
			payload:  map[string]any{"user_id": "user-snake"},
			cred:     Credential{},
			wantUser: "user-snake", wantReqs: 1,
		},
		{
			name:     "a stored email survives a vendor read for the id",
			payload:  map[string]any{"id": "user-from-vendor", "email": "vendor@example.com"},
			cred:     Credential{Account: "stored@example.com"},
			wantUser: "user-from-vendor", wantEmail: "stored@example.com", wantReqs: 1,
		},
		{
			name:       "a refusal is reported, not signed around",
			status:     http.StatusForbidden,
			payload:    map[string]any{},
			cred:       Credential{},
			wantReqs:   1,
			wantErrSub: "refused with http 403",
		},
		{
			name:       "an identity with no id in it is refused by name",
			payload:    map[string]any{"email": "nobody@example.com"},
			cred:       Credential{},
			wantReqs:   1,
			wantErrSub: "names no account",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := newIdentityStub(t, tc.status, tc.payload)
			connector := newIdentityConnector(t, stub.server.URL+"/api/v1/userinfo")

			identity, err := connector.signingIdentity(context.Background(), tc.cred, "jt-token")
			if tc.wantErrSub != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Fatalf("signingIdentity() error = %v, want one containing %q", err, tc.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("signingIdentity() error = %v", err)
			}
			if identity.UserID != tc.wantUser || identity.Email != tc.wantEmail {
				t.Fatalf("identity = %+v, want user %q email %q", identity, tc.wantUser, tc.wantEmail)
			}
			if identity.AuthToken != "jt-token" {
				t.Fatalf("auth token = %q, want the credential being signed with", identity.AuthToken)
			}
			if stub.requests != tc.wantReqs {
				t.Fatalf("userinfo requests = %d, want %d", stub.requests, tc.wantReqs)
			}
		})
	}
}

func TestQoderIdentityCache(t *testing.T) {
	stub := newIdentityStub(t, http.StatusOK, map[string]any{"id": "user-one", "email": "one@example.com"})
	connector := newIdentityConnector(t, stub.server.URL+"/api/v1/userinfo")
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	clock := now
	connector.identity.now = func() time.Time { return clock }

	ctx := context.Background()
	for range 3 {
		if _, err := connector.accountFor(ctx, "jt-token"); err != nil {
			t.Fatalf("accountFor() error = %v", err)
		}
	}
	if stub.requests != 1 {
		t.Fatalf("userinfo requests = %d, want one read cached across three calls", stub.requests)
	}

	// A different bearer is a different account as far as the cache is concerned:
	// sharing one answer between tokens would sign a request as the wrong account.
	if _, err := connector.accountFor(ctx, "jt-other-token"); err != nil {
		t.Fatalf("accountFor() error = %v", err)
	}
	if stub.requests != 2 {
		t.Fatalf("userinfo requests = %d, want a second read for a second token", stub.requests)
	}

	clock = now.Add(qoderIdentityTTL)
	if _, err := connector.accountFor(ctx, "jt-token"); err != nil {
		t.Fatalf("accountFor() error = %v", err)
	}
	if stub.requests != 3 {
		t.Fatalf("userinfo requests = %d, want a re-read once the answer is as old as the ttl", stub.requests)
	}
}

func TestQoderIdentityEndpoint(t *testing.T) {
	cases := []struct {
		name  string
		oauth *registry.OAuth
		want  string
	}{
		{
			name: "the declared endpoint wins",
			oauth: &registry.OAuth{
				OpenAPIBaseURL: "https://openapi.qoder.sh",
				UserInfoURL:    "https://openapi.qoder.sh/api/v1/userinfo",
			},
			want: "https://openapi.qoder.sh/api/v1/userinfo",
		},
		{
			name:  "the openapi host answers when no endpoint is declared",
			oauth: &registry.OAuth{OpenAPIBaseURL: "https://openapi.qoder.cn/", UserInfoURL: ""},
			want:  "https://openapi.qoder.cn" + qoderUserInfoPath,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			connector, err := NewQoder(registry.Provider{ID: "qoder", OAuth: tc.oauth}, http.DefaultClient)
			if err != nil {
				t.Fatalf("NewQoder() error = %v", err)
			}
			if got := connector.userInfoURL(); got != tc.want {
				t.Fatalf("userInfoURL() = %q, want %q", got, tc.want)
			}
		})
	}
}
