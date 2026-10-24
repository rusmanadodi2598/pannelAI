// Package handler tests dashboard authentication HTTP contracts.
//
// @file      internal/handler/auth_test.go
// @for       Table-driven auth endpoint tests through typed service contracts.
// @uses      net/http, net/http/httptest, strings, testing.
// @reason    Strict TDD requires happy, validation, and authentication cases for every P0 auth route without mocking handler output.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-09-17
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthHandlerLoginTable(t *testing.T) {
	cases := []struct {
		name, body string
		wantStatus int
		wantCookie bool
	}{
		{"valid password", `{"password":"correct"}`, http.StatusNoContent, true},
		{"missing password", `{}`, http.StatusBadRequest, false},
		{"malformed json", `{"password":`, http.StatusBadRequest, false},
		{"wrong password", `{"password":"wrong"}`, http.StatusUnauthorized, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newAuthHandlerForTest(t, "correct")
			r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(tc.body))
			r.RemoteAddr = "127.0.0.1:3456"
			w := httptest.NewRecorder()
			h.Login(w, r)
			if w.Code != tc.wantStatus {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.wantStatus, w.Body.String())
			}
			if (len(w.Result().Cookies()) > 0) != tc.wantCookie {
				t.Fatalf("cookie presence=%v want=%v", len(w.Result().Cookies()) > 0, tc.wantCookie)
			}
			if tc.wantCookie {
				cookie := w.Result().Cookies()[0]
				if cookie.Name != sessionCookieName || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" {
					t.Fatalf("unexpected cookie: %+v", cookie)
				}
			}
		})
	}
}

func TestAuthHandlerSessionLifecycle(t *testing.T) {
	h, _ := newAuthHandlerForTest(t, "correct")
	login := httptest.NewRecorder()
	h.Login(login, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"correct"}`)))
	if login.Code != http.StatusNoContent {
		t.Fatalf("login status=%d", login.Code)
	}
	cookie := login.Result().Cookies()[0]

	statusReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)
	statusReq.AddCookie(cookie)
	status := httptest.NewRecorder()
	h.Status(status, statusReq)
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"authenticated":true`) {
		t.Fatalf("status=%d body=%s", status.Code, status.Body.String())
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logoutReq.AddCookie(cookie)
	logout := httptest.NewRecorder()
	h.Logout(logout, logoutReq)
	if logout.Code != http.StatusNoContent || logout.Result().Cookies()[0].MaxAge >= 0 {
		t.Fatalf("logout=%d cookie=%+v", logout.Code, logout.Result().Cookies())
	}

	protected := httptest.NewRecorder()
	h.RequireSession(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(protected, logoutReq)
	if protected.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session status=%d", protected.Code)
	}
}

func TestAuthHandlerChangePasswordTable(t *testing.T) {
	cases := []struct {
		name, current, next string
		want                int
	}{
		{"valid", "correct", "next-password", http.StatusNoContent},
		{"wrong current", "wrong", "next-password", http.StatusUnauthorized},
		{"empty new", "correct", "", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newAuthHandlerForTest(t, "correct")
			login := httptest.NewRecorder()
			h.Login(login, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"correct"}`)))
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", strings.NewReader(`{"current_password":"`+tc.current+`","new_password":"`+tc.next+`"}`))
			req.AddCookie(login.Result().Cookies()[0])
			w := httptest.NewRecorder()
			h.ChangePassword(w, req)
			if w.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}
