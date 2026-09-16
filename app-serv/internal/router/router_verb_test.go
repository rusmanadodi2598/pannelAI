// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_verb_test.go
// @for       Status assertions for every registered route plus the wrong-verb path.
// @uses      internal/router, internal/schema, net/http, net/http/httptest,
//
//	strings, encoding/json, testing.
//
// @reason    A route that was never registered answers 405 where the contract promises a handler, which is exactly the defect a status-only test catches cheaply.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-16
package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRoutes_VerbTable walks every registered route through the real mux and
// asserts the status the contract promises, including the verb-mismatch case
// that a missing registration would otherwise hide.
func TestRoutes_VerbTable(t *testing.T) {
	mux := newTestRouter(t)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{"health", http.MethodGet, "/api/v1/health", "", http.StatusOK},
		{"version", http.MethodGet, "/api/v1/version", "", http.StatusOK},
		{"list keys", http.MethodGet, "/api/v1/gateway-keys", "", http.StatusOK},
		{"create key", http.MethodPost, "/api/v1/gateway-keys", `{"name":"cli"}`, http.StatusCreated},
		{"create key without name", http.MethodPost, "/api/v1/gateway-keys", `{}`, http.StatusBadRequest},
		{"unknown id", http.MethodGet, "/api/v1/gateway-keys/gky_missing", "", http.StatusNotFound},
		{"unknown route", http.MethodGet, "/api/v1/nope", "", http.StatusNotFound},
		{"wrong verb on collection", http.MethodDelete, "/api/v1/gateway-keys", "", http.StatusMethodNotAllowed},
		{"wrong verb on item", http.MethodPost, "/api/v1/gateway-keys/gky_x", "", http.StatusMethodNotAllowed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body *strings.Reader
			if tc.body != "" {
				body = strings.NewReader(tc.body)
			} else {
				body = strings.NewReader("")
			}
			req := httptest.NewRequest(tc.method, tc.path, body)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != tc.want {
				t.Fatalf("%s %s = %d, want %d (body: %s)", tc.method, tc.path, rr.Code, tc.want, rr.Body.String())
			}
		})
	}
}
