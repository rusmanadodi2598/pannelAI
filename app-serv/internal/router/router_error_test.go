// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_error_test.go
// @for       Assertions that every error response is exactly one JSON document.
// @uses      internal/router, internal/schema, net/http, net/http/httptest,
//
//	strings, encoding/json, testing.
//
// @reason    A middleware that appends an envelope on top of a handler body still passes a status check, so this decodes and then proves no second document follows.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-16
package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestRoutes_ErrorBodyIsSingleJSON asserts each error response is exactly one
// JSON document. A middleware that appends an envelope on top of a handler's
// own body would still pass a status-only check, so this decodes the body and
// then asserts no second document follows.
func TestRoutes_ErrorBodyIsSingleJSON(t *testing.T) {
	mux := newTestRouter(t)

	cases := []struct {
		name   string
		method string
		path   string
		want   int
		code   string
	}{
		{"unknown resource", http.MethodGet, "/api/v1/gateway-keys/gky_missing", http.StatusNotFound, "NOT_FOUND"},
		{"unknown route", http.MethodGet, "/api/v1/nope", http.StatusNotFound, "NOT_FOUND"},
		{"verb mismatch", http.MethodDelete, "/api/v1/gateway-keys", http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(""))
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != tc.want {
				t.Fatalf("status = %d, want %d", rr.Code, tc.want)
			}

			dec := json.NewDecoder(rr.Body)
			var first schema.ErrorBody
			if err := dec.Decode(&first); err != nil {
				t.Fatalf("body is not one JSON document: %v (body: %q)", err, rr.Body.String())
			}
			if first.Error.Code != tc.code {
				t.Fatalf("code = %q, want %q", first.Error.Code, tc.code)
			}

			// A second document after the first is the double-body defect.
			var extra map[string]any
			if err := dec.Decode(&extra); err == nil {
				t.Fatalf("body carried a second JSON document: %q", rr.Body.String())
			}
		})
	}
}
