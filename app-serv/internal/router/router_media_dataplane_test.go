// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_media_dataplane_test.go
// @for       Route-level tests for the six §7.10 data-plane media routes.
// @uses      net/http, net/http/httptest, strings, testing.
// @reason    These routes are registered next to the management ones and share
//
//	the /api/v1/media prefix's neighbourhood, so the routing itself is
//	worth pinning: they must be reachable without a dashboard session,
//	and they must answer in the data-plane envelope rather than the
//	management one. The doubles are in router_media_stub_test.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-19
package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mediaDataPlaneRoutes is every §7.10 data-plane route with a body for the
// verbs that take one. The table is the audit against the spec table.
var mediaDataPlaneRoutes = []struct {
	name   string
	method string
	path   string
	body   string
}{
	{name: "speech", method: http.MethodPost, path: "/api/v1/audio/speech",
		body: `{"model":"openai/gpt-4o-mini-tts","input":"hello"}`},
	{name: "voices", method: http.MethodGet, path: "/api/v1/audio/voices?provider=openai"},
	{name: "images", method: http.MethodPost, path: "/api/v1/images/generations",
		body: `{"model":"openai/gpt-image-1","prompt":"a cat"}`},
	{name: "videos", method: http.MethodPost, path: "/api/v1/videos/generations",
		body: `{"model":"openai/sora","prompt":"a cat"}`},
	{name: "search", method: http.MethodPost, path: "/api/v1/search",
		body: `{"provider":"brave-search","query":"golang"}`},
}

// TestMediaDataPlaneRoutesAreNotSessionGated proves the routes answer without a
// dashboard cookie. A session-gated route would refuse with the management
// envelope, which carries no `type` field; the data-plane envelope does.
func TestMediaDataPlaneRoutesAreNotSessionGated(t *testing.T) {
	mux := newManagementRouter(t)
	for _, route := range mediaDataPlaneRoutes {
		t.Run(route.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequest(route.method, route.path, strings.NewReader(route.body)))
			if recorder.Code == http.StatusUnauthorized && !strings.Contains(recorder.Body.String(), `"type"`) {
				t.Fatalf("status = %d, want the route to run: %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

// TestMediaDataPlaneRoutes_HappyPath drives the routes the panel and a CLI tool
// use, through the real mux.
func TestMediaDataPlaneRoutes_HappyPath(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		body   string
		status int
		want   string
	}{
		{
			name: "speech", method: http.MethodPost, path: "/api/v1/audio/speech",
			body: `{"model":"openai/gpt-4o-mini-tts","input":"hello"}`, status: http.StatusOK,
			want: `"url":"https://img.example.com/a.png"`,
		},
		{
			name: "voices", method: http.MethodGet, path: "/api/v1/audio/voices?provider=openai",
			status: http.StatusOK, want: `"object":"list"`,
		},
		{
			name: "images", method: http.MethodPost, path: "/api/v1/images/generations",
			body: `{"model":"openai/gpt-image-1","prompt":"a cat"}`, status: http.StatusOK,
			want: `"data"`,
		},
		{
			// The refusal is the point: no registry provider declares video, so
			// the route answers PROVIDER_NOT_ROUTABLE rather than 404.
			name: "videos", method: http.MethodPost, path: "/api/v1/videos/generations",
			body: `{"model":"openai/sora","prompt":"a cat"}`, status: http.StatusBadRequest,
			want: `"PROVIDER_NOT_ROUTABLE"`,
		},
		{
			name: "search", method: http.MethodPost, path: "/api/v1/search",
			body: `{"provider":"brave-search","query":"golang"}`, status: http.StatusOK,
			want: `"title":"Go"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mux := newManagementRouter(t)
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
			if recorder.Code != tc.status {
				t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, tc.status, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), tc.want) {
				t.Fatalf("body = %s, want it to contain %s", recorder.Body.String(), tc.want)
			}
		})
	}
}

// TestMediaDataPlaneRoutes_ValidationFailure pins the data-plane envelope for a
// bad payload, so a CLI tool reads the shape it already branches on.
func TestMediaDataPlaneRoutes_ValidationFailure(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		body   string
		status int
	}{
		{name: "images without a prompt", path: "/api/v1/images/generations",
			body: `{"model":"openai/gpt-image-1"}`, status: http.StatusBadRequest},
		{name: "search without a query", path: "/api/v1/search",
			body: `{"provider":"brave-search"}`, status: http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mux := newManagementRouter(t)
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body)))
			if recorder.Code != tc.status {
				t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, tc.status, recorder.Body.String())
			}
			body := recorder.Body.String()
			if !strings.Contains(body, `"VALIDATION_ERROR"`) || !strings.Contains(body, `"type"`) {
				t.Fatalf("body = %s, want the data-plane VALIDATION_ERROR envelope", body)
			}
		})
	}
}
