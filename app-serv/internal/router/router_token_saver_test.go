// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_token_saver_test.go
// @for       Route-table tests for the §7.9 token-saver routes.
// @uses      internal/handler, internal/registry, internal/schema,
//
//	internal/service, net/http, net/http/httptest, strings, testing.
//
// @reason    The route's session gate and verb table are the mux's job, so they
//
//	are pinned through the real mux with the production auth fixture,
//	not through the handler alone.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-19
package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// TestTokenSaverRoutes_SessionGated pins that both §7.9 verbs are management
// routes: an unauthenticated request is a 401 in the §8 envelope, never the
// handler's answer.
func TestTokenSaverRoutes_SessionGated(t *testing.T) {
	mux := newTokenSaverRouter(t)
	for _, tc := range []struct {
		verb string
		path string
	}{
		{http.MethodGet, "/api/v1/token-saver"},
		{http.MethodPut, "/api/v1/token-saver"},
	} {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(tc.verb, tc.path, nil))
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s without a session = %d, want 401 (body: %s)", tc.verb, recorder.Code, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), `"UNAUTHORIZED"`) {
			t.Fatalf("body = %s, want the UNAUTHORIZED code", recorder.Body.String())
		}
	}
}

// TestTokenSaverRoutes_ThroughMux drives the read and the whole replacement
// through the real mux with a session, including the verb the route does not
// register.
func TestTokenSaverRoutes_ThroughMux(t *testing.T) {
	mux := newTokenSaverRouter(t)
	cookie := loginCookie(t, mux)

	get := httptest.NewRequest(http.MethodGet, "/api/v1/token-saver", nil)
	get.AddCookie(cookie)
	getRecorder := httptest.NewRecorder()
	mux.ServeHTTP(getRecorder, get)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get = %d (body: %s)", getRecorder.Code, getRecorder.Body.String())
	}
	if !strings.Contains(getRecorder.Body.String(), `"rtk":{"enabled":true,"level":"full"}`) {
		t.Fatalf("get body = %s, want the §7.9 defaults", getRecorder.Body.String())
	}

	put := httptest.NewRequest(http.MethodPut, "/api/v1/token-saver",
		strings.NewReader(`{"rtk":{"enabled":false,"level":"lite"},"headroom":{"enabled":false,"url":"","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"}}`))
	put.AddCookie(cookie)
	putRecorder := httptest.NewRecorder()
	mux.ServeHTTP(putRecorder, put)
	if putRecorder.Code != http.StatusOK {
		t.Fatalf("put = %d (body: %s)", putRecorder.Code, putRecorder.Body.String())
	}

	after := httptest.NewRecorder()
	afterRequest := httptest.NewRequest(http.MethodGet, "/api/v1/token-saver", nil)
	afterRequest.AddCookie(cookie)
	mux.ServeHTTP(after, afterRequest)
	if !strings.Contains(after.Body.String(), `"rtk":{"enabled":false,"level":"lite"}`) {
		t.Fatalf("body after put = %s, want the replaced document", after.Body.String())
	}

	verb := httptest.NewRecorder()
	verbRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/token-saver", nil)
	verbRequest.AddCookie(cookie)
	mux.ServeHTTP(verb, verbRequest)
	if verb.Code != http.StatusMethodNotAllowed {
		t.Fatalf("delete = %d, want 405", verb.Code)
	}
	if !strings.Contains(verb.Body.String(), `"METHOD_NOT_ALLOWED"`) {
		t.Fatalf("body = %s, want the METHOD_NOT_ALLOWED code", verb.Body.String())
	}
}

// newTokenSaverRouter builds the real mux with the §7.9 handler wired alongside
// the auth fixture, over in-memory repositories.
func newTokenSaverRouter(t *testing.T) *Mux {
	t.Helper()
	_, authHandler := newAuthenticatedRouter(t)
	settings, err := service.NewSettingsService(service.SettingsServiceDeps{Repo: newMemSettingsRepo()})
	if err != nil {
		t.Fatalf("settings service: %v", err)
	}
	saver, err := service.NewTokenSaverService(service.TokenSaverServiceDeps{Settings: settings})
	if err != nil {
		t.Fatalf("token saver service: %v", err)
	}
	return New(Deps{
		System: handler.NewSystemHandler(handler.SystemHandlerDeps{
			Info:   schema.SystemInfo{Version: "test", Commit: "test"},
			Health: service.NewHealthService(service.HealthServiceDeps{}),
		}),
		Auth:       authHandler,
		TokenSaver: handler.NewTokenSaverHandler(saver),
	})
}

// memSettingsRepo is an in-memory SettingsRepository for the §7.9 route tests.
type memSettingsRepo struct {
	rows map[domain.SettingsKey]string
}

func newMemSettingsRepo() *memSettingsRepo {
	return &memSettingsRepo{rows: map[domain.SettingsKey]string{}}
}

func (r *memSettingsRepo) Load(context.Context) (map[domain.SettingsKey]string, error) {
	out := make(map[domain.SettingsKey]string, len(r.rows))
	for key, value := range r.rows {
		out[key] = value
	}
	return out, nil
}

func (r *memSettingsRepo) Save(_ context.Context, key domain.SettingsKey, value string) error {
	r.rows[key] = value
	return nil
}
