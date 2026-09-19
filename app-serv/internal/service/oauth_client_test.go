// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_client_test.go
// @for       Table-driven tests for the OAuth token client's userinfo decode
//
//	(SPEC-API-001 §7.4: the account identity a callback matches on).
//
// @uses      encoding/json, io, net/http, net/http/httptest, net/url,
//
//	strings, testing.
//
// @reason    The identity decides whether a connect creates an account or
// updates one, so the decode must survive every spelling a provider
// uses for the same field: a numeric id, a string id, a null id, and
// a payload with no id at all. A decode that fails on one spelling
// would fail the whole callback and leave the account unconnectable.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestOAuthIdentityDecodesEveryProviderSpelling(t *testing.T) {
	cases := []struct {
		name        string
		payload     string
		wantEmail   string
		wantName    string
		wantAccount string
	}{
		{name: "numeric id with a login name", payload: `{"id":12345,"login":"octo"}`,
			wantEmail: "octo", wantName: "octo", wantAccount: "12345"},
		{name: "string id with sub and email", payload: `{"id":"user_abc","sub":"sub-1","email":"dev@example.com","name":"Dev One"}`,
			wantEmail: "dev@example.com", wantName: "Dev One", wantAccount: "user_abc"},
		{name: "null id falls back to the sub", payload: `{"id":null,"sub":"sub-2"}`,
			wantEmail: "sub-2", wantAccount: ""},
		{name: "no id at all", payload: `{"email":"solo@example.com"}`,
			wantEmail: "solo@example.com", wantAccount: ""},
		{name: "empty payload", payload: `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var identity OAuthIdentity
			if err := json.Unmarshal([]byte(tc.payload), &identity); err != nil {
				t.Fatalf("decoding %s: %v", tc.payload, err)
			}
			account := identity.Account()
			if account.Email != tc.wantEmail {
				t.Fatalf("email = %q, want %q", account.Email, tc.wantEmail)
			}
			if account.Name != tc.wantName {
				t.Fatalf("name = %q, want %q", account.Name, tc.wantName)
			}
			if account.WorkspaceID != tc.wantAccount {
				t.Fatalf("workspace id = %q, want %q", account.WorkspaceID, tc.wantAccount)
			}
		})
	}
}

func TestUserInfoReadsTheAccountAndRefusesARefusal(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		body     string
		wantCode string
		wantID   string
	}{
		{name: "a string id decodes", status: http.StatusOK, body: `{"id":"user_abc"}`,
			wantID: "user_abc"},
		{name: "a non-json body is an upstream error", status: http.StatusOK, body: `not json`,
			wantCode: "UPSTREAM_ERROR"},
		{name: "a rejection is an upstream error", status: http.StatusUnauthorized, body: `{"error":"bad token"}`,
			wantCode: "UPSTREAM_ERROR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("Authorization"); got != "Bearer the-token" {
					t.Errorf("authorization = %q, want the access token", got)
				}
				w.WriteHeader(tc.status)
				if _, err := w.Write([]byte(tc.body)); err != nil {
					t.Errorf("writing the answer: %v", err)
				}
			}))
			defer server.Close()

			client := NewOAuthHTTPClient(server.Client())
			identity, err := client.UserInfo(context.Background(), server.URL, "the-token")
			if tc.wantCode != "" {
				mustAppError(t, err, tc.wantCode)
				return
			}
			if err != nil {
				t.Fatalf("UserInfo(): %v", err)
			}
			if string(identity.ID) != tc.wantID {
				t.Fatalf("identity id = %q, want %q", identity.ID, tc.wantID)
			}
		})
	}
}

func TestGrantSendsTheDeclaredEncoding(t *testing.T) {
	cases := []struct {
		name     string
		encoding string
		wantType string
	}{
		{name: "form encoding is the default", encoding: "",
			wantType: "application/x-www-form-urlencoded"},
		{name: "json encoding when declared", encoding: "json",
			wantType: "application/json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("Content-Type"); got != tc.wantType {
					t.Errorf("content type = %q, want %q", got, tc.wantType)
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("reading the grant body: %v", err)
				}
				fields := map[string]string{}
				if tc.wantType == "application/json" {
					if err := json.Unmarshal(body, &fields); err != nil {
						t.Errorf("the json grant body is not a flat object: %v (%s)", err, body)
					}
				} else {
					values, err := url.ParseQuery(string(body))
					if err != nil {
						t.Errorf("parsing the form grant body: %v", err)
					}
					for name := range values {
						fields[name] = values.Get(name)
					}
				}
				if fields["refresh_token"] != "rt-1" || fields["grant_type"] != "refresh_token" {
					t.Errorf("grant fields = %v, want the refresh grant", fields)
				}
				if fields["code"] != "" {
					t.Errorf("a refresh grant carried a code: %v", fields)
				}
				if _, err := w.Write([]byte(`{"access_token":"at-2","expires_in":60}`)); err != nil {
					t.Errorf("writing the answer: %v", err)
				}
			}))
			defer server.Close()

			client := NewOAuthHTTPClient(server.Client())
			answer, err := client.Grant(context.Background(), server.URL, tc.encoding, TokenGrant{
				GrantType: "refresh_token", RefreshToken: "rt-1", ClientID: "client-1",
			})
			if err != nil {
				t.Fatalf("Grant(): %v", err)
			}
			if answer.AccessToken != "at-2" || answer.ExpiresIn != 60 {
				t.Fatalf("answer = %+v", answer)
			}
		})
	}
}

func TestGrantReportsAProviderRefusal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(`{"error":"invalid_grant","error_description":"code expired"}`)); err != nil {
			t.Errorf("writing the refusal: %v", err)
		}
	}))
	defer server.Close()

	client := NewOAuthHTTPClient(server.Client())
	_, err := client.Grant(context.Background(), server.URL, "", TokenGrant{GrantType: "authorization_code"})
	mustAppError(t, err, "UPSTREAM_ERROR")
	if !strings.Contains(err.Error(), "invalid_grant") {
		t.Fatalf("error %q does not carry the provider's reason", err)
	}
}

func TestGrantRefusesAnAnswerWithoutAToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(`{"expires_in":60}`)); err != nil {
			t.Errorf("writing the answer: %v", err)
		}
	}))
	defer server.Close()

	client := NewOAuthHTTPClient(server.Client())
	_, err := client.Grant(context.Background(), server.URL, "json", TokenGrant{GrantType: "refresh_token"})
	mustAppError(t, err, "UPSTREAM_ERROR")
}
