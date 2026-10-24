// Package handler adapts HTTP requests to app-serv services.
//
// @file      internal/handler/auth_cookie_test.go
// @for       The session cookie's security attributes at every deployment profile.
// @uses      net/http, net/http/httptest, testing, time.
// @reason    The cookie carries the only browser credential, so a wiring regression in Secure or SameSite is invisible to the auth flow tests that only read the token.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-10-24
package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionCookieAttributes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		secure bool
	}{
		{"secure deployment", true},
		{"local opt-out", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			setSessionCookie(rec, "token", SessionCookieOptions{Secure: tc.secure, TTL: time.Hour})
			cookies := rec.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatalf("cookies = %d, want 1", len(cookies))
			}
			cookie := cookies[0]
			if cookie.Name != sessionCookieName || cookie.Value != "token" ||
				cookie.Path != "/" || !cookie.HttpOnly ||
				cookie.SameSite != http.SameSiteLaxMode || cookie.Secure != tc.secure {
				t.Fatalf("cookie = %+v", cookie)
			}
		})
	}
}

func TestClearSessionCookieIsExpired(t *testing.T) {
	rec := httptest.NewRecorder()
	clearSessionCookie(rec, true)
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != sessionCookieName || cookie.MaxAge >= 0 || !cookie.Secure || !cookie.HttpOnly {
		t.Fatalf("cookie = %+v", cookie)
	}
}
