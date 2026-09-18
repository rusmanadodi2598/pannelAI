// Package handler adapts HTTP requests to app-serv services.
//
// @file      internal/handler/auth_cookie.go
// @for       Reads, writes, and clears the dashboard session cookie.
// @uses      net/http, time, internal/config.
// @reason    SPEC-API-001 §7.2 fixes an HttpOnly session cookie as the browser
//
//	credential while keeping token formatting outside HTTP handlers.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-09-17
package handler

import (
	"net/http"
	"time"
)

const sessionCookieName = "pannel_session"

// SessionCookieOptions controls deployment-specific cookie security attributes.
type SessionCookieOptions struct {
	Secure bool
	TTL    time.Duration
}

func sessionToken(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func setSessionCookie(w http.ResponseWriter, token string, options SessionCookieOptions) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: token, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: options.Secure,
		MaxAge: int(options.TTL / time.Second), Expires: time.Now().Add(options.TTL),
	})
}

func clearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: secure, MaxAge: -1,
		Expires: time.Unix(1, 0),
	})
}
