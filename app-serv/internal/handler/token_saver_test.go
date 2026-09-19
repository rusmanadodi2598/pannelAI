// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/token_saver_test.go
// @for       HTTP tests for the §7.9 token-saver routes.
// @uses      internal/domain, internal/service, net/http, strings, testing.
// @reason    AGENTS.md §2.1 requires a happy path and a validation-failure path
//
//	per route. §7.9's write is a whole replacement, so the table also
//	pins that a rejected body leaves the served document untouched.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// stubTokenSaverSettings is an in-memory SettingsRepository for the §7.9 tests.
type stubTokenSaverSettings struct {
	mu   sync.Mutex
	rows map[domain.SettingsKey]string
}

func newStubTokenSaverSettings() *stubTokenSaverSettings {
	return &stubTokenSaverSettings{rows: map[domain.SettingsKey]string{}}
}

func (r *stubTokenSaverSettings) Load(context.Context) (map[domain.SettingsKey]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[domain.SettingsKey]string, len(r.rows))
	for key, value := range r.rows {
		out[key] = value
	}
	return out, nil
}

func (r *stubTokenSaverSettings) Save(_ context.Context, key domain.SettingsKey, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[key] = value
	return nil
}

// newTokenSaverFixture wires the §7.9 handler over an in-memory settings store.
func newTokenSaverFixture(t *testing.T) *TokenSaverHandler {
	t.Helper()
	settings, err := service.NewSettingsService(service.SettingsServiceDeps{Repo: newStubTokenSaverSettings()})
	if err != nil {
		t.Fatalf("NewSettingsService() error = %v", err)
	}
	saver, err := service.NewTokenSaverService(service.TokenSaverServiceDeps{Settings: settings})
	if err != nil {
		t.Fatalf("NewTokenSaverService() error = %v", err)
	}
	return NewTokenSaverHandler(saver)
}

// TestTokenSaverHandler_GetServesTheDefaults documents the fresh-install
// payload: the §7.9 defaults with every level rendered, never a 404.
func TestTokenSaverHandler_GetServesTheDefaults(t *testing.T) {
	h := newTokenSaverFixture(t)
	rr := do(t, http.MethodGet, "/api/v1/token-saver", "", h.Get)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{`"rtk":{"enabled":true,"level":"full"}`, `"headroom"`, `"ponytail":{"enabled":false,"level":"full"}`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body %s must contain %s", body, want)
		}
	}
}

// TestTokenSaverHandler_Put covers every rejection the route owes its client.
func TestTokenSaverHandler_Put(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		status int
	}{
		{
			name:   "every saver on at the ultra level",
			body:   `{"rtk":{"enabled":true,"level":"ultra"},"headroom":{"enabled":true,"url":"http://localhost:8787","compress_user_messages":true},"ponytail":{"enabled":true,"level":"lite"}}`,
			status: http.StatusOK,
		},
		{
			name:   "the documented defaults",
			body:   `{"rtk":{"enabled":true,"level":"full"},"headroom":{"enabled":false,"url":"","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"}}`,
			status: http.StatusOK,
		},
		{name: "a missing rtk group", body: `{"headroom":{"enabled":false,"url":"","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"}}`, status: http.StatusBadRequest},
		{name: "an unknown level word", body: `{"rtk":{"enabled":true,"level":"maximum"},"headroom":{"enabled":false,"url":"","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"}}`, status: http.StatusBadRequest},
		{name: "a headroom url with a non-http scheme", body: `{"rtk":{"enabled":false,"level":"full"},"headroom":{"enabled":true,"url":"gopher://localhost:8787","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"}}`, status: http.StatusBadRequest},
		{name: "headroom enabled without a url", body: `{"rtk":{"enabled":false,"level":"full"},"headroom":{"enabled":true,"url":"","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"}}`, status: http.StatusBadRequest},
		{name: "an unknown field", body: `{"rtk":{"enabled":true,"level":"full"},"headroom":{"enabled":false,"url":"","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"},"caveman":{"enabled":true}}`, status: http.StatusBadRequest},
		{name: "a malformed body", body: `{"rtk":`, status: http.StatusBadRequest},
		{name: "an empty body", body: ``, status: http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newTokenSaverFixture(t)
			rr := do(t, http.MethodPut, "/api/v1/token-saver", tc.body, h.Put)
			if rr.Code != tc.status {
				t.Fatalf("status = %d, want %d (body: %s)", rr.Code, tc.status, rr.Body.String())
			}
			if tc.status != http.StatusBadRequest {
				return
			}
			body := decodeBody(t, rr)
			errObj, ok := body["error"].(map[string]any)
			if !ok {
				t.Fatalf("response must carry the error envelope: %v", body)
			}
			if code, _ := errObj["code"].(string); code != "VALIDATION_ERROR" {
				t.Fatalf("code = %q, want VALIDATION_ERROR", code)
			}
			if msg, _ := errObj["message"].(string); msg == "" {
				t.Fatal("error message must not be empty")
			}
		})
	}
}

// TestTokenSaverHandler_PutThenGetRoundTrip proves a document the PUT stores
// comes back out of GET field for field, which the panel's save relies on.
func TestTokenSaverHandler_PutThenGetRoundTrip(t *testing.T) {
	h := newTokenSaverFixture(t)
	put := `{"rtk":{"enabled":true,"level":"ultra"},"headroom":{"enabled":true,"url":"https://compress.internal:8787","compress_user_messages":true},"ponytail":{"enabled":true,"level":"lite"}}`
	rr := do(t, http.MethodPut, "/api/v1/token-saver", put, h.Put)
	if rr.Code != http.StatusOK {
		t.Fatalf("put = %d (body: %s)", rr.Code, rr.Body.String())
	}
	after := do(t, http.MethodGet, "/api/v1/token-saver", "", h.Get)
	if after.Code != http.StatusOK {
		t.Fatalf("get = %d (body: %s)", after.Code, after.Body.String())
	}
	if after.Body.String() != rr.Body.String() {
		t.Fatalf("GET after PUT = %s, want the PUT response %s", after.Body.String(), rr.Body.String())
	}
	for _, want := range []string{`"level":"ultra"`, `"compress_user_messages":true`, `"level":"lite"`} {
		if !strings.Contains(after.Body.String(), want) {
			t.Fatalf("body %s must contain %s", after.Body.String(), want)
		}
	}
}
