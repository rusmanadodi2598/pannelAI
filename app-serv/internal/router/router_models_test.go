// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_models_test.go
// @for       Route-level tests for the §7.6–§7.8 model, combo, and vision (first half; split at the AGENTS.md §1.1 line limit).
// @uses      internal/domain, internal/handler, internal/registry,
// @reason    AGENTS.md §2.1 requires a happy path, a validation-failure path,
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-17
package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// managementRoutes is every route this vertical registers, with a body for the
// verbs that need one. The table is the audit against the §7.6–§7.8 spec tables:
// a route that stops being registered fails here.
var managementRoutes = []struct {
	name   string
	method string
	path   string
	body   string
}{
	{"catalog", http.MethodGet, "/api/v1/models/catalog", ""},
	{"custom list", http.MethodGet, "/api/v1/models/custom", ""},
	{"custom create", http.MethodPost, "/api/v1/models/custom", `{"provider_id":"openai","model_id":"x","display_name":"X"}`},
	{"custom delete", http.MethodDelete, "/api/v1/models/custom/mdl_x", ""},
	{"aliases get", http.MethodGet, "/api/v1/models/aliases", ""},
	{"aliases put", http.MethodPut, "/api/v1/models/aliases", `{"aliases":[]}`},
	{"disabled get", http.MethodGet, "/api/v1/models/disabled", ""},
	{"disabled put", http.MethodPut, "/api/v1/models/disabled", `{"models":[]}`},
	{"combo list", http.MethodGet, "/api/v1/combos", ""},
	{"combo create", http.MethodPost, "/api/v1/combos", `{"name":"daily","strategy":"fallback","models":[{"ref":"openai/gpt-4o","priority":1}]}`},
	{"combo detail", http.MethodGet, "/api/v1/combos/cmb_x", ""},
	{"combo update", http.MethodPatch, "/api/v1/combos/cmb_x", `{"name":"daily","strategy":"fallback","models":[{"ref":"openai/gpt-4o","priority":1}]}`},
	{"combo delete", http.MethodDelete, "/api/v1/combos/cmb_x", ""},
	{"vision get", http.MethodGet, "/api/v1/vision-adapter", ""},
	{"vision put", http.MethodPut, "/api/v1/vision-adapter", `{"enabled":false,"round_robin":false,"models":[]}`},
}

// TestManagementRoutesRequireSession proves every new route is session-gated:
// without a cookie each answers 401 in the §8 envelope, never a redirect or a 500
// from an unguarded handler.
func TestManagementRoutesRequireSession(t *testing.T) {
	mux := newManagementRouter(t)
	for _, route := range managementRoutes {
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

// TestManagementRoutes_HappyPath drives the read routes and the write routes
// through the real mux with a valid session cookie, which is what the panel does.
func TestManagementRoutes_HappyPath(t *testing.T) {
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
		{name: "catalog", method: http.MethodGet, path: "/api/v1/models/catalog", status: http.StatusOK, want: `"data"`},
		{
			name: "catalog filtered", method: http.MethodGet,
			path:   "/api/v1/models/catalog?provider_id=openai&capability=vision",
			status: http.StatusOK, want: `"openai/gpt-4o"`,
		},
		{name: "custom list", method: http.MethodGet, path: "/api/v1/models/custom", status: http.StatusOK, want: `"data"`},
		{
			name: "custom create", method: http.MethodPost, path: "/api/v1/models/custom",
			body:   `{"provider_id":"openai","model_id":"gpt-5","display_name":"GPT-5"}`,
			status: http.StatusCreated, want: `"mdl_`,
		},
		{name: "aliases get", method: http.MethodGet, path: "/api/v1/models/aliases", status: http.StatusOK, want: `"data"`},
		{
			name: "aliases put", method: http.MethodPut, path: "/api/v1/models/aliases",
			body:   `{"aliases":[{"alias":"fast","target":"openai/gpt-4o-mini"}]}`,
			status: http.StatusOK, want: `"fast"`,
		},
		{name: "disabled get", method: http.MethodGet, path: "/api/v1/models/disabled", status: http.StatusOK, want: `"data"`},
		{
			name: "disabled put", method: http.MethodPut, path: "/api/v1/models/disabled",
			body:   `{"models":[{"provider_id":"openai","model_id":"gpt-4o-mini"}]}`,
			status: http.StatusOK, want: `"gpt-4o-mini"`,
		},
		{name: "combo list", method: http.MethodGet, path: "/api/v1/combos", status: http.StatusOK, want: `"meta"`},
		{
			name: "combo create", method: http.MethodPost, path: "/api/v1/combos",
			body:   `{"name":"daily","strategy":"fallback","models":[{"ref":"openai/gpt-4o","priority":1}]}`,
			status: http.StatusCreated, want: `"cmb_`,
		},
		{
			name: "combo detail", method: http.MethodGet, path: "/api/v1/combos/cmb_seeded",
			status: http.StatusOK, want: `"seeded-combo"`,
		},
		{
			name: "combo update", method: http.MethodPatch, path: "/api/v1/combos/cmb_seeded",
			body:   `{"name":"seeded-combo","strategy":"round_robin","sticky_limit":2,"models":[{"ref":"openai/gpt-4o","priority":1}]}`,
			status: http.StatusOK, want: `"round_robin"`,
		},
		{name: "vision get", method: http.MethodGet, path: "/api/v1/vision-adapter", status: http.StatusOK, want: `"enabled"`},
		{
			name: "vision put", method: http.MethodPut, path: "/api/v1/vision-adapter",
			body:   `{"enabled":false,"round_robin":false,"models":[]}`,
			status: http.StatusOK, want: `"round_robin"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.AddCookie(cookie)
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, request)
			if recorder.Code != tc.status {
				t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, tc.status, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), tc.want) {
				t.Fatalf("body = %s, want it to contain %s", recorder.Body.String(), tc.want)
			}
		})
	}
}

// TestManagementRoutes_ValidationFailure pins the §8 envelope for a bad payload
// on each write route, through the mux, so the code the panel branches on is the
// one the spec fixes.
func TestManagementRoutes_ValidationFailure(t *testing.T) {
	mux := newManagementRouter(t)
	cookie := loginCookie(t, mux)
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"custom create without a provider", http.MethodPost, "/api/v1/models/custom", `{"model_id":"x","display_name":"X"}`},
		{"custom create for an unknown provider", http.MethodPost, "/api/v1/models/custom", `{"provider_id":"nope","model_id":"x","display_name":"X"}`},
		{"aliases put with an unknown target", http.MethodPut, "/api/v1/models/aliases", `{"aliases":[{"alias":"ghost","target":"openai/ghost"}]}`},
		{"disabled put with an unknown model", http.MethodPut, "/api/v1/models/disabled", `{"models":[{"provider_id":"openai","model_id":"ghost"}]}`},
		{"combo create with a bad strategy", http.MethodPost, "/api/v1/combos", `{"name":"daily","strategy":"sequential","models":[{"ref":"openai/gpt-4o","priority":1}]}`},
		{"combo create without models", http.MethodPost, "/api/v1/combos", `{"name":"empty","strategy":"fallback","models":[]}`},
		{"combo create with an unresolved ref", http.MethodPost, "/api/v1/combos", `{"name":"daily","strategy":"fallback","models":[{"ref":"openai/ghost","priority":1}]}`},
		{"combo update with an unresolved ref", http.MethodPatch, "/api/v1/combos/cmb_seeded", `{"name":"seeded-combo","strategy":"fallback","models":[{"ref":"openai/ghost","priority":1}]}`},
		{"combo update to fusion without a judge", http.MethodPatch, "/api/v1/combos/cmb_seeded", `{"name":"seeded-combo","strategy":"fusion","models":[{"ref":"openai/gpt-4o","priority":1}]}`},
		{"vision put without models", http.MethodPut, "/api/v1/vision-adapter", `{"enabled":true,"round_robin":false}`},
		{"vision put with an out-of-catalog model", http.MethodPut, "/api/v1/vision-adapter", `{"enabled":true,"round_robin":false,"models":["openai/ghost"]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.AddCookie(cookie)
			recorder := httptest.NewRecorder()
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

// TestManagementRoutes_WrongVerbIsMethodNotAllowed proves the patterns carry
// their verb, so a wrong verb is a 405 from the mux rather than a handler call.
func TestManagementRoutes_WrongVerbIsMethodNotAllowed(t *testing.T) {
	mux := newManagementRouter(t)
	cookie := loginCookie(t, mux)
	cases := []struct {
		name   string
		method string
		path   string
	}{
		{"deleting the catalog", http.MethodDelete, "/api/v1/models/catalog"},
		{"posting to the vision adapter", http.MethodPost, "/api/v1/vision-adapter"},
		{"putting a combo detail", http.MethodPut, "/api/v1/combos/cmb_seeded"},
		{"patching the alias set", http.MethodPatch, "/api/v1/models/aliases"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader("{}"))
			request.AddCookie(cookie)
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405 (body: %s)", recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), `"METHOD_NOT_ALLOWED"`) {
				t.Fatalf("body = %s, want the METHOD_NOT_ALLOWED code", recorder.Body.String())
			}
		})
	}
}
