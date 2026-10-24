// Package handler adapts HTTP requests to app-serv services.
//
// @file      internal/handler/auth_origin.go
// @for       Refuses cookie-authenticated mutations a foreign origin started.
// @uses      net/http, net/url, strings.
// @reason    SameSite=Lax already stops a cross-site POST in a modern browser, but a browser that still sends the cookie would have the mutation executed for it. The guard reads the browser's own signals first because the panel forwards server-side, so Host names app-serv rather than the origin the browser sees.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-10-24
package handler

import (
	"net/http"
	"net/url"
	"strings"
)

// sameOriginRequest reports whether a state-changing request names this host.
// Safe methods pass untouched. A request carrying no browser signal at all is
// a non-browser client: it can omit the headers instead of forging them, so
// refusing it would close nothing and break every CLI.
func sameOriginRequest(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	// Fetch metadata is the one signal a browser derives and a page cannot set.
	// The panel proxy forwards it unchanged, so it survives the server-side hop.
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" {
		return site == "same-origin" || site == "none"
	}
	source := r.Header.Get("Origin")
	if source == "" {
		source = r.Header.Get("Referer")
	}
	if source == "" {
		return true
	}
	parsed, err := url.Parse(source)
	if err != nil || parsed.Host == "" {
		return false
	}
	// The panel forwards /api/v1 from its own server, so the browser's origin
	// names the panel while Host names app-serv; the proxy records the browser's
	// host here. A browser cannot set this header, and a non-browser client that
	// could does not need to pass this guard at all.
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return strings.EqualFold(parsed.Host, host)
}
