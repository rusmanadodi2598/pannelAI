// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/media_guard_test.go
// @for       The §4 key check and §4 validation envelope every §7.10
//
//	data-plane media route applies.
//
// @uses      net/http, net/http/httptest, strings, testing.
// @reason    AGENTS.md §2.1 requires an auth-failure and a validation-failure
//
//	test per route, and both are cross-cutting here: the same envelope
//	and the same key rule apply to all six, so one parameterized table
//	per rule proves each route instead of six near-identical tests.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestMediaHandler_ValidationFailure pins the §4 envelope for a bad payload on
// each route that takes one.
func TestMediaHandler_ValidationFailure(t *testing.T) {
	cases := []struct {
		name string
		path string
		body string
		fn   func(*MediaHandler) http.HandlerFunc
	}{
		{
			name: "speech without an input", path: "/api/v1/audio/speech",
			body: `{"model":"openai/gpt-4o-mini-tts"}`, fn: func(h *MediaHandler) http.HandlerFunc { return h.Speech },
		},
		{
			name: "speech with an unknown format", path: "/api/v1/audio/speech",
			body: `{"model":"openai/gpt-4o-mini-tts","input":"hi","response_format":"mp4"}`,
			fn:   func(h *MediaHandler) http.HandlerFunc { return h.Speech },
		},
		{
			name: "images without a prompt", path: "/api/v1/images/generations",
			body: `{"model":"openai/gpt-image-1"}`, fn: func(h *MediaHandler) http.HandlerFunc { return h.Images },
		},
		{
			name: "images with an unknown style", path: "/api/v1/images/generations",
			body: `{"model":"openai/gpt-image-1","prompt":"a cat","style":"cubist"}`,
			fn:   func(h *MediaHandler) http.HandlerFunc { return h.Images },
		},
		{
			name: "videos without a prompt", path: "/api/v1/videos/generations",
			body: `{"model":"openai/sora"}`, fn: func(h *MediaHandler) http.HandlerFunc { return h.Videos },
		},
		{
			name: "search without a query", path: "/api/v1/search",
			body: `{"provider":"brave-search"}`, fn: func(h *MediaHandler) http.HandlerFunc { return h.Search },
		},
		{
			name: "search without a provider", path: "/api/v1/search",
			body: `{"query":"golang"}`, fn: func(h *MediaHandler) http.HandlerFunc { return h.Search },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, _ := newMediaHandlerFixture(t)
			rr := do(t, http.MethodPost, tc.path, tc.body, tc.fn(f))
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), `"VALIDATION_ERROR"`) {
				t.Fatalf("body = %s, want the VALIDATION_ERROR code", rr.Body.String())
			}
		})
	}
}

// TestMediaHandler_RequiresAKey pins that every data-plane media route applies
// the §4 gateway-key rule before it does any work.
func TestMediaHandler_RequiresAKey(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "speech", method: http.MethodPost, path: "/api/v1/audio/speech",
			body: `{"model":"openai/gpt-4o-mini-tts","input":"hi"}`},
		{name: "transcriptions", method: http.MethodPost, path: "/api/v1/audio/transcriptions"},
		{name: "voices", method: http.MethodGet, path: "/api/v1/audio/voices?provider=openai"},
		{name: "images", method: http.MethodPost, path: "/api/v1/images/generations",
			body: `{"model":"openai/gpt-image-1","prompt":"a cat"}`},
		{name: "videos", method: http.MethodPost, path: "/api/v1/videos/generations",
			body: `{"model":"openai/sora","prompt":"a cat"}`},
		{name: "search", method: http.MethodPost, path: "/api/v1/search",
			body: `{"provider":"brave-search","query":"golang"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, _ := newMediaHandlerFixture(t)
			f.auth = stubAuthenticator{token: "sk-valid"}

			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.name == "transcriptions" {
				request = transcriptionRequest(t, tc.path, "openai/whisper-1", "clip.mp3", []byte("audio"))
			}
			rr := httptest.NewRecorder()
			mediaRoute(t, f, tc.name)(rr, request)
			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401 (body: %s)", rr.Code, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), `"UNAUTHORIZED"`) {
				t.Fatalf("body = %s, want the UNAUTHORIZED code", rr.Body.String())
			}
		})
	}
}

// mediaRoute returns the handler method one route name addresses.
func mediaRoute(t *testing.T, f *MediaHandler, name string) http.HandlerFunc {
	t.Helper()
	switch name {
	case "speech":
		return f.Speech
	case "transcriptions":
		return f.Transcribe
	case "voices":
		return f.Voices
	case "images":
		return f.Images
	case "videos":
		return f.Videos
	case "search":
		return f.Search
	default:
		t.Fatalf("unknown route %q", name)
		return nil
	}
}
