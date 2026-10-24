// Package router tests versioned auth routes and management protection.
//
// @file      internal/router/router_origin_test.go
// @for       Proves the cross-origin guard is wired into the management mux.
// @uses      net/http, net/http/httptest, strings, testing.
// @reason    The guard lives in the session middleware, so a handler test alone cannot show that every management mutation is behind it.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability stable
// @since     2026-10-24
package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestManagementMutationsRefuseAForeignOrigin(t *testing.T) {
	mux, _ := newAuthenticatedRouter(t)

	foreignLogin := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"correct"}`))
	foreignLogin.Header.Set("Origin", "https://evil.example")
	refusedLogin := httptest.NewRecorder()
	mux.ServeHTTP(refusedLogin, foreignLogin)
	if refusedLogin.Code != http.StatusForbidden || len(refusedLogin.Result().Cookies()) != 0 {
		t.Fatalf("foreign login=%d cookies=%+v want 403 with no cookie",
			refusedLogin.Code, refusedLogin.Result().Cookies())
	}

	login := httptest.NewRecorder()
	mux.ServeHTTP(login, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"correct"}`)))
	if login.Code != http.StatusNoContent {
		t.Fatalf("login=%d body=%s", login.Code, login.Body.String())
	}
	cookie := login.Result().Cookies()[0]

	cases := []struct {
		name   string
		origin string
		want   int
	}{
		{name: "a foreign origin is refused", origin: "https://evil.example", want: http.StatusForbidden},
		{name: "a non-browser client without headers passes", origin: "", want: http.StatusCreated},
		{name: "a same-origin browser passes", origin: "http://example.com", want: http.StatusCreated},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"name":"` + strings.ReplaceAll(tc.name, " ", "-") + `"}`
			r := httptest.NewRequest(http.MethodPost, "/api/v1/gateway-keys", strings.NewReader(body))
			r.AddCookie(cookie)
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}
