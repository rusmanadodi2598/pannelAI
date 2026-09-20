// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_settings_routes_test.go
// @for       Route-table tests for the §7.14 settings routes.
//
// @uses      internal/handler, internal/schema, internal/service, net/http,
//
//	net/http/httptest, strings, testing.
//
// @reason    The settings PATCH is the one door every gateway configuration
//
//	change goes through, so its session gate, its per-key validation,
//	and its round-trip belong to the mux where §7.14 places them, not
//	to the handler alone.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-20
package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// TestSettingsRoutes_SessionGated pins that both §7.14 verbs are management
// routes: an unauthenticated request is a 401 in the §8 envelope, never the
// document.
func TestSettingsRoutes_SessionGated(t *testing.T) {
	mux := newSettingsRouter(t)
	for _, tc := range []struct {
		verb string
		body string
	}{
		{http.MethodGet, ""},
		{http.MethodPatch, `{"logging":{"retention_days":14}}`},
	} {
		request := httptest.NewRequest(tc.verb, "/api/v1/settings", strings.NewReader(tc.body))
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s without a session = %d, want 401 (body: %s)", tc.verb, recorder.Code, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), `"UNAUTHORIZED"`) {
			t.Fatalf("body = %s, want the UNAUTHORIZED code", recorder.Body.String())
		}
	}
}

// TestSettingsRoutes_ThroughMux drives the read and the partial patch through
// the real mux with a session: the documented defaults on a fresh install, a
// patch that round-trips over the wire, and the rejections §7.14's per-key
// validation owes the client.
func TestSettingsRoutes_ThroughMux(t *testing.T) {
	mux := newSettingsRouter(t)
	cookie := loginCookie(t, mux)

	get := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	get.AddCookie(cookie)
	getRecorder := httptest.NewRecorder()
	mux.ServeHTTP(getRecorder, get)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get = %d (body: %s)", getRecorder.Code, getRecorder.Body.String())
	}
	body := getRecorder.Body.String()
	for _, want := range []string{`"require_login":true`, `"require_api_key":true`, `"retention_days":7`} {
		if !strings.Contains(body, want) {
			t.Fatalf("get body = %s, want the §7.14 defaults to carry %s", body, want)
		}
	}
	if strings.Contains(body, "caveman") {
		t.Fatalf("get body = %s, want no deprecated key in the response (§7.9)", body)
	}

	patch := httptest.NewRequest(http.MethodPatch, "/api/v1/settings",
		strings.NewReader(`{"logging":{"retention_days":14}}`))
	patch.AddCookie(cookie)
	patchRecorder := httptest.NewRecorder()
	mux.ServeHTTP(patchRecorder, patch)
	if patchRecorder.Code != http.StatusOK {
		t.Fatalf("patch = %d (body: %s)", patchRecorder.Code, patchRecorder.Body.String())
	}
	if !strings.Contains(patchRecorder.Body.String(), `"retention_days":14`) {
		t.Fatalf("patch body = %s, want the patched document echoed back", patchRecorder.Body.String())
	}

	after := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	after.AddCookie(cookie)
	afterRecorder := httptest.NewRecorder()
	mux.ServeHTTP(afterRecorder, after)
	if !strings.Contains(afterRecorder.Body.String(), `"retention_days":14`) {
		t.Fatalf("get after patch = %s, want the change to persist", afterRecorder.Body.String())
	}

	invalid := httptest.NewRequest(http.MethodPatch, "/api/v1/settings",
		strings.NewReader(`{"routing":{"combo_sticky_limit":0}}`))
	invalid.AddCookie(cookie)
	invalidRecorder := httptest.NewRecorder()
	mux.ServeHTTP(invalidRecorder, invalid)
	if invalidRecorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid patch = %d (body: %s)", invalidRecorder.Code, invalidRecorder.Body.String())
	}
	if !strings.Contains(invalidRecorder.Body.String(), `"VALIDATION_ERROR"`) {
		t.Fatalf("body = %s, want the VALIDATION_ERROR code", invalidRecorder.Body.String())
	}

	malformed := httptest.NewRequest(http.MethodPatch, "/api/v1/settings", strings.NewReader(`{`))
	malformed.AddCookie(cookie)
	malformedRecorder := httptest.NewRecorder()
	mux.ServeHTTP(malformedRecorder, malformed)
	if malformedRecorder.Code != http.StatusBadRequest {
		t.Fatalf("malformed body = %d, want 400", malformedRecorder.Code)
	}

	verb := httptest.NewRequest(http.MethodPut, "/api/v1/settings", strings.NewReader(`{}`))
	verb.AddCookie(cookie)
	verbRecorder := httptest.NewRecorder()
	mux.ServeHTTP(verbRecorder, verb)
	if verbRecorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("put = %d, want 405", verbRecorder.Code)
	}
}

// TestSettingsRoutes_PatchKeepsUntouchedGroupsIntact pins the §7.14 partial
// update on the wire: a patch naming one group leaves the stored values of the
// other groups answering, which is the property two concurrent PATCHes of
// different groups rely on.
func TestSettingsRoutes_PatchKeepsUntouchedGroupsIntact(t *testing.T) {
	mux := newSettingsRouter(t)
	cookie := loginCookie(t, mux)

	patch := func(body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPatch, "/api/v1/settings", strings.NewReader(body))
		request.AddCookie(cookie)
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("patch %s = %d (body: %s)", body, recorder.Code, recorder.Body.String())
		}
		return recorder
	}

	patch(`{"logging":{"request_capture_enabled":true}}`)
	patch(`{"network":{"outbound_proxy_url":"http://proxy.internal:8080"}}`)

	get := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	get.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, get)
	body := recorder.Body.String()
	if !strings.Contains(body, `"request_capture_enabled":true`) {
		t.Fatalf("body = %s, want the first patch's value kept", body)
	}
	if !strings.Contains(body, `"outbound_proxy_url":"http://proxy.internal:8080"`) {
		t.Fatalf("body = %s, want the second patch's value kept", body)
	}
	if !strings.Contains(body, `"retention_days":7`) {
		t.Fatalf("body = %s, want the untouched default kept", body)
	}
}

// newSettingsRouter builds the real mux with the §7.14 handler wired alongside
// the auth fixture, over the in-memory settings repository the §7.9 route tests
// already declare.
func newSettingsRouter(t *testing.T) *Mux {
	t.Helper()
	_, authHandler := newAuthenticatedRouter(t)
	settings, err := service.NewSettingsService(service.SettingsServiceDeps{Repo: newMemSettingsRepo()})
	if err != nil {
		t.Fatalf("settings service: %v", err)
	}
	return New(Deps{
		System: handler.NewSystemHandler(handler.SystemHandlerDeps{
			Info:   schema.SystemInfo{Version: "test", Commit: "test"},
			Health: service.NewHealthService(service.HealthServiceDeps{}),
		}),
		Auth:     authHandler,
		Settings: handler.NewSettingsHandler(settings),
	})
}
