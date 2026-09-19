// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_media_routes_test.go
// @for       Route-level tests for the §7.10 media provider routes.
// @uses      net/http, net/http/httptest, strings, testing.
// @reason    §7.10's management routes are their own vertical and the §7.6–§7.8
//
//	route-table files sit close to the AGENTS.md §1.1 line limit, so the
//	table lives here. The session-gating table is duplicated on purpose:
//	each vertical proves its own routes are guarded. The doubles are in
//	router_media_stub_test.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-19
package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mediaRoutes is every §7.10 management route with a body for the verb that
// needs one. The table is the audit against the spec table: a route that stops
// being registered fails here.
var mediaRoutes = []struct {
	name   string
	method string
	path   string
	body   string
}{
	{"list", http.MethodGet, "/api/v1/media-providers", ""},
	{"list by kind", http.MethodGet, "/api/v1/media-providers?kind=tts", ""},
	{"detail", http.MethodGet, "/api/v1/media-providers/openai", ""},
	{"patch", http.MethodPatch, "/api/v1/media-providers/openai",
		`{"kind":"tts","default_model":"gpt-4o-mini-tts"}`},
}

// TestMediaRoutesRequireSession proves every §7.10 management route is
// session-gated: without a cookie each answers 401 in the §8 envelope.
func TestMediaRoutesRequireSession(t *testing.T) {
	mux := newManagementRouter(t)
	for _, route := range mediaRoutes {
		t.Run(route.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequest(route.method, route.path, strings.NewReader(route.body)))
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401 (body: %s)", recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), `"UNAUTHORIZED"`) {
				t.Fatalf("body = %s, want the UNAUTHORIZED code", recorder.Body.String())
			}
		})
	}
}

// TestMediaRoutes_HappyPath drives the routes the panel uses, in order: list
// every kind, filter to one, read a provider's detail, then save an override
// and read the resolved value back.
func TestMediaRoutes_HappyPath(t *testing.T) {
	mux := newManagementRouter(t)
	cookie := loginCookie(t, mux)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		status int
		want   string
	}{
		{
			name: "list", method: http.MethodGet, path: "/api/v1/media-providers",
			status: http.StatusOK, want: `"provider_id":"openai"`,
		},
		{
			name: "list by kind", method: http.MethodGet, path: "/api/v1/media-providers?kind=image",
			status: http.StatusOK, want: `"kind":"image"`,
		},
		{
			name: "detail", method: http.MethodGet, path: "/api/v1/media-providers/openai",
			status: http.StatusOK, want: `"default_model":"gpt-4o-mini-tts"`,
		},
		{
			name: "patch", method: http.MethodPatch, path: "/api/v1/media-providers/openai",
			body:   `{"kind":"tts","base_url":"http://127.0.0.1:8000/v1"}`,
			status: http.StatusOK, want: `"base_url_source":"override"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.AddCookie(cookie)
			mux.ServeHTTP(recorder, request)
			if recorder.Code != tc.status {
				t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, tc.status, recorder.Body.String())
			}
			if tc.want != "" && !strings.Contains(recorder.Body.String(), tc.want) {
				t.Fatalf("body = %s, want it to contain %s", recorder.Body.String(), tc.want)
			}
		})
	}
}

// TestMediaRoutes_ValidationFailure pins the §8 envelope for a bad request on
// each route that can refuse one.
func TestMediaRoutes_ValidationFailure(t *testing.T) {
	mux := newManagementRouter(t)
	cookie := loginCookie(t, mux)
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"list with an unknown kind", http.MethodGet, "/api/v1/media-providers?kind=music", ""},
		{"patch with an unknown kind", http.MethodPatch, "/api/v1/media-providers/openai",
			`{"kind":"web","base_url":"https://x.example.com"}`},
		{"patch with a relative base URL", http.MethodPatch, "/api/v1/media-providers/openai",
			`{"kind":"tts","base_url":"/v1/audio/speech"}`},
		{"patch a kind the provider does not offer", http.MethodPatch, "/api/v1/media-providers/chatonly",
			`{"kind":"tts","base_url":"https://x.example.com"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.AddCookie(cookie)
			mux.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), `"VALIDATION_ERROR"`) {
				t.Fatalf("body = %s, want the VALIDATION_ERROR code", recorder.Body.String())
			}
		})
	}
}

// TestMediaRoutes_UnknownProvider pins the not-found mapping on both routes that
// address a provider.
func TestMediaRoutes_UnknownProvider(t *testing.T) {
	mux := newManagementRouter(t)
	cookie := loginCookie(t, mux)
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"detail", http.MethodGet, "/api/v1/media-providers/ghost", ""},
		{"patch", http.MethodPatch, "/api/v1/media-providers/ghost",
			`{"kind":"tts","base_url":"https://x.example.com"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.AddCookie(cookie)
			mux.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404 (body: %s)", recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), `"NOT_FOUND"`) {
				t.Fatalf("body = %s, want the NOT_FOUND code", recorder.Body.String())
			}
		})
	}
}
