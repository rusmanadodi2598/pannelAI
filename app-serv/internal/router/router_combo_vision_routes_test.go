// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_combo_vision_routes_test.go
// @for       The §7.7 and §7.8 routes' session gate and verb table.
// @uses      net/http, net/http/httptest, strings, testing.
// @reason    The combo and vision-adapter journeys are pinned at the service
//
//	and handler layers; what only the mux can prove is the access
//	control (OWASP A01 function-level rule: every route denies an
//	unauthenticated caller, deny by default) and the method table (a
//	verb the spec does not register is a 405, never a handler).
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
)

// TestComboAndVisionRoutes_SessionGated pins the OWASP A01 deny-by-default rule
// across every §7.7 and §7.8 route: without a session the answer is always the
// §8 401 envelope, and the two read rows also prove the gate is the only thing
// standing in the way by serving 200 with one.
func TestComboAndVisionRoutes_SessionGated(t *testing.T) {
	mux := newManagementRouter(t)
	cases := []struct {
		name   string
		method string
		path   string
	}{
		{"list", http.MethodGet, "/api/v1/combos"},
		{"create", http.MethodPost, "/api/v1/combos"},
		{"detail", http.MethodGet, "/api/v1/combos/cmb_seeded"},
		{"update", http.MethodPatch, "/api/v1/combos/cmb_seeded"},
		{"delete", http.MethodDelete, "/api/v1/combos/cmb_seeded"},
		{"probe", http.MethodPost, "/api/v1/combos/cmb_seeded/test"},
		{"adapter read", http.MethodGet, "/api/v1/vision-adapter"},
		{"adapter replace", http.MethodPut, "/api/v1/vision-adapter"},
	}
	for _, tc := range cases {
		t.Run("without a session: "+tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`)))
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("%s %s = %d, want 401 (body: %s)", tc.method, tc.path, recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), `"UNAUTHORIZED"`) {
				t.Fatalf("body = %s, want the UNAUTHORIZED code", recorder.Body.String())
			}
		})
	}
	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{"list", http.MethodGet, "/api/v1/combos"},
		{"adapter read", http.MethodGet, "/api/v1/vision-adapter"},
	} {
		t.Run("with a session: "+tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tc.method, tc.path, nil)
			request.AddCookie(loginCookie(t, mux))
			mux.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("%s = %d, want 200 (body: %s)", tc.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

// TestComboAndVisionRoutes_VerbEnforcement pins the method table: the verbs §7.7
// and §7.8 do not register are refused by the mux with the §8 code before any
// handler runs, for a caller who holds a valid session.
func TestComboAndVisionRoutes_VerbEnforcement(t *testing.T) {
	mux := newManagementRouter(t)
	cookie := loginCookie(t, mux)
	cases := []struct {
		name   string
		method string
		path   string
	}{
		{"replace a collection", http.MethodPut, "/api/v1/combos"},
		{"delete a collection", http.MethodDelete, "/api/v1/combos"},
		{"probe with a read", http.MethodGet, "/api/v1/combos/cmb_seeded/test"},
		{"patch the adapter", http.MethodPatch, "/api/v1/vision-adapter"},
		{"post the adapter", http.MethodPost, "/api/v1/vision-adapter"},
		{"delete the adapter", http.MethodDelete, "/api/v1/vision-adapter"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
			request.AddCookie(cookie)
			mux.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusMethodNotAllowed {
				t.Fatalf("%s %s = %d, want 405 (body: %s)", tc.method, tc.path, recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), `"METHOD_NOT_ALLOWED"`) {
				t.Fatalf("body = %s, want the METHOD_NOT_ALLOWED code", recorder.Body.String())
			}
		})
	}
}
