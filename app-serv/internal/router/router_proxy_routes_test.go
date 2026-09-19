// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_proxy_routes_test.go
// @for       Route-level tests for the §7.11 proxy pool routes.
// @uses      encoding/json, net/http, net/http/httptest, strings, testing.
// @reason    §7.11's six routes are their own vertical, and the §7.6–§7.8
//
//	route-table files sit close to the AGENTS.md §1.1 line limit, so the
//	proxy table lives here rather than growing those files. The
//	session-gating table is duplicated on purpose: each vertical proves
//	its own routes are guarded. The doubles are in
//	router_proxy_stub_test.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-19
package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// proxyCreateBody is a valid create body; the happy-path test reads its id back.
const proxyCreateBody = `{"label":"pool-a","protocol":"http","host":"proxy.example.com","port":3128,` +
	`"username":"operator","password":"s3cret-value"}`

// proxyRoutes is every §7.11 route with a valid body for the verbs that need
// one. The table is the audit against the spec table: a route that stops being
// registered fails here.
var proxyRoutes = []struct {
	name   string
	method string
	path   string
	body   string
}{
	{"list", http.MethodGet, "/api/v1/proxies", ""},
	{"create", http.MethodPost, "/api/v1/proxies", proxyCreateBody},
	{"patch", http.MethodPatch, "/api/v1/proxies/prx_x", proxyCreateBody},
	{"delete", http.MethodDelete, "/api/v1/proxies/prx_x", ""},
	{"test stored", http.MethodPost, "/api/v1/proxies/prx_x/test", ""},
	{"test candidate", http.MethodPost, "/api/v1/proxies/test",
		`{"protocol":"http","host":"proxy.example.com","port":3128}`},
}

// TestProxyRoutesRequireSession proves every §7.11 route is session-gated:
// without a cookie each answers 401 in the §8 envelope.
func TestProxyRoutesRequireSession(t *testing.T) {
	mux := newManagementRouter(t)
	for _, route := range proxyRoutes {
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

// TestProxyRoutes_HappyPath drives the six routes through the real mux with a
// valid session cookie, in the order the panel uses them: create, read, patch,
// test, and delete the candidate it created.
func TestProxyRoutes_HappyPath(t *testing.T) {
	mux := newManagementRouter(t)
	cookie := loginCookie(t, mux)

	created := proxyCall(t, mux, cookie, http.MethodPost, "/api/v1/proxies", proxyCreateBody, http.StatusCreated)
	id, _ := created["id"].(string)
	if !strings.HasPrefix(id, "prx_") {
		t.Fatalf("created id = %q, want the prx_ prefix", id)
	}

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		status int
		want   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/v1/proxies", status: http.StatusOK, want: `"data"`},
		{
			name: "patch", method: http.MethodPatch, path: "/api/v1/proxies/" + id,
			body:   `{"label":"pool-b","protocol":"http","host":"proxy.example.com","port":3128}`,
			status: http.StatusOK, want: `"pool-b"`,
		},
		{
			name: "test stored", method: http.MethodPost, path: "/api/v1/proxies/" + id + "/test",
			status: http.StatusOK, want: `"ok"`,
		},
		{
			name: "test candidate", method: http.MethodPost, path: "/api/v1/proxies/test",
			body:   `{"protocol":"socks5","host":"proxy.example.com","port":1080}`,
			status: http.StatusOK, want: `"ok"`,
		},
		{name: "delete", method: http.MethodDelete, path: "/api/v1/proxies/" + id, status: http.StatusNoContent},
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

// TestProxyRoutes_ValidationFailure pins the §8 envelope for a bad payload on
// each write route, through the mux.
func TestProxyRoutes_ValidationFailure(t *testing.T) {
	mux := newManagementRouter(t)
	cookie := loginCookie(t, mux)
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"create with an unknown protocol", http.MethodPost, "/api/v1/proxies",
			`{"label":"pool","protocol":"ftp","host":"proxy.example.com","port":3128}`},
		{"create with a URL in the host field", http.MethodPost, "/api/v1/proxies",
			`{"label":"pool","protocol":"http","host":"http://proxy.example.com","port":3128}`},
		{"patch with a port outside the range", http.MethodPatch, "/api/v1/proxies/prx_x",
			`{"label":"pool","protocol":"http","host":"proxy.example.com","port":70000}`},
		{"test candidate without a host", http.MethodPost, "/api/v1/proxies/test",
			`{"protocol":"http","port":3128}`},
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

// TestProxyRoutes_UnknownID pins the not-found mapping on the routes that
// address a stored candidate.
func TestProxyRoutes_UnknownID(t *testing.T) {
	mux := newManagementRouter(t)
	cookie := loginCookie(t, mux)
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"patch", http.MethodPatch, "/api/v1/proxies/prx_missing",
			`{"label":"pool","protocol":"http","host":"proxy.example.com","port":3128}`},
		{"delete", http.MethodDelete, "/api/v1/proxies/prx_missing", ""},
		{"test", http.MethodPost, "/api/v1/proxies/prx_missing/test", ""},
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

// proxyCall drives one request and decodes the answer.
func proxyCall(t *testing.T, mux *Mux, cookie *http.Cookie, method, path, body string, status int) map[string]any {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != status {
		t.Fatalf("%s %s = %d, want %d (body: %s)", method, path, recorder.Code, status, recorder.Body.String())
	}
	var out map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&out); err != nil {
		t.Fatalf("decoding %s %s response: %v", method, path, err)
	}
	return out
}
