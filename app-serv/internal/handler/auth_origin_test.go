// Package handler tests dashboard authentication HTTP contracts.
//
// @file      internal/handler/auth_origin_test.go
// @for       Pins the cross-origin rule applied to cookie-authenticated mutations.
// @uses      net/http, net/http/httptest, strings, testing.
// @reason    A foreign origin must be refused before the mutation runs, while a non-browser client without browser headers must still reach the same handler.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-10-24
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSameOriginRequest(t *testing.T) {
	cases := []struct {
		name    string
		method  string
		host    string
		headers map[string]string
		want    bool
	}{
		{name: "a mutation without browser headers is a non-browser client", method: http.MethodPost, host: "api.internal:9090", want: true},
		{name: "safe methods pass whatever the origin", method: http.MethodGet, host: "api.internal:9090", headers: map[string]string{"Origin": "https://evil.example"}, want: true},
		{name: "a same-origin mutation passes", method: http.MethodPost, host: "panel.example.com", headers: map[string]string{"Origin": "https://panel.example.com"}, want: true},
		{name: "the host comparison ignores case", method: http.MethodPost, host: "panel.example.com", headers: map[string]string{"Origin": "https://PANEL.example.com"}, want: true},
		{name: "a foreign origin is refused", method: http.MethodPost, host: "panel.example.com", headers: map[string]string{"Origin": "https://evil.example"}, want: false},
		{name: "the forwarded host names the origin the browser sees", method: http.MethodPost, host: "127.0.0.1:9090", headers: map[string]string{"Origin": "https://panel.example.com", "X-Forwarded-Host": "panel.example.com"}, want: true},
		{name: "a same-origin referer stands in for a missing origin", method: http.MethodPost, host: "panel.example.com", headers: map[string]string{"Referer": "https://panel.example.com/settings"}, want: true},
		{name: "a foreign referer is refused when the origin is absent", method: http.MethodPost, host: "panel.example.com", headers: map[string]string{"Referer": "https://evil.example/page"}, want: false},
		{name: "the null origin is refused", method: http.MethodPost, host: "panel.example.com", headers: map[string]string{"Origin": "null"}, want: false},
		{name: "an unparseable origin is refused", method: http.MethodPost, host: "panel.example.com", headers: map[string]string{"Origin": "://bad"}, want: false},
		{name: "cross-site fetch metadata is refused", method: http.MethodPost, host: "panel.example.com", headers: map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://panel.example.com"}, want: false},
		{name: "same-site fetch metadata is refused", method: http.MethodPost, host: "panel.example.com", headers: map[string]string{"Sec-Fetch-Site": "same-site"}, want: false},
		{name: "same-origin fetch metadata passes behind the panel", method: http.MethodPost, host: "127.0.0.1:9090", headers: map[string]string{"Sec-Fetch-Site": "same-origin"}, want: true},
		{name: "a navigation with no initiator passes", method: http.MethodPost, host: "127.0.0.1:9090", headers: map[string]string{"Sec-Fetch-Site": "none"}, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "/api/v1/proxies", nil)
			r.Host = tc.host
			for key, value := range tc.headers {
				r.Header.Set(key, value)
			}
			if got := sameOriginRequest(r); got != tc.want {
				t.Fatalf("sameOriginRequest() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAuthHandlerRefusesAForeignOrigin(t *testing.T) {
	h, _ := newAuthHandlerForTest(t, "correct")

	foreign := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"correct"}`))
	foreign.Header.Set("Origin", "https://evil.example")
	login := httptest.NewRecorder()
	h.Login(login, foreign)
	if login.Code != http.StatusForbidden {
		t.Fatalf("foreign login status=%d want=%d", login.Code, http.StatusForbidden)
	}
	if !strings.Contains(login.Body.String(), `"FORBIDDEN"`) || len(login.Result().Cookies()) != 0 {
		t.Fatalf("foreign login body=%s cookies=%+v", login.Body.String(), login.Result().Cookies())
	}

	sameOrigin := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"correct"}`))
	sameOrigin.Header.Set("Origin", "http://panel.test")
	sameOrigin.Host = "panel.test"
	allowed := httptest.NewRecorder()
	h.Login(allowed, sameOrigin)
	if allowed.Code != http.StatusNoContent {
		t.Fatalf("same-origin login status=%d body=%s", allowed.Code, allowed.Body.String())
	}
	cookie := allowed.Result().Cookies()[0]
	if status := guardedRequest(h, cookie, "https://evil.example"); status != http.StatusForbidden {
		t.Fatalf("foreign guarded status=%d want=%d", status, http.StatusForbidden)
	}
	if status := guardedRequest(h, cookie, ""); status != http.StatusNoContent {
		t.Fatalf("non-browser guarded status=%d want=%d", status, http.StatusNoContent)
	}
}

// guardedRequest drives the session guard with a valid cookie and the given
// origin, reporting the status the caller's handler would have produced.
func guardedRequest(h *AuthHandler, cookie *http.Cookie, origin string) int {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/proxies", nil)
	r.AddCookie(cookie)
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	h.RequireSession(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(w, r)
	return w.Code
}
