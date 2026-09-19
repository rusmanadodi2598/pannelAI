// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/media_provider_test.go
// @for       HTTP tests for the §7.10 media provider routes.
// @uses      net/http, strings, testing.
// @reason    AGENTS.md §2.1 requires a happy path and a validation-failure path
//
//	per route. The kind filter is the interesting one: an unknown kind must
//	be a 400 naming the closed set, never an empty page a panel would render
//	as "nothing configured".
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"net/http"
	"strings"
	"testing"
)

// TestMediaProviderHandler_List pins the list and the kind filter.
func TestMediaProviderHandler_List(t *testing.T) {
	f, _ := newMediaProviderFixture(t)

	rr := do(t, http.MethodGet, "/api/v1/media-providers", "", f.List)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	data, ok := decodeBody(t, rr)["data"].([]any)
	if !ok || len(data) != 2 {
		t.Fatalf("data = %v, want the two media kinds", data)
	}
	first, _ := data[0].(map[string]any)
	if first["provider_id"] != "openai" || first["provider_name"] != "OpenAI" {
		t.Fatalf("first row = %v, want the openai identity", first)
	}
	if first["base_url_source"] != "registry" || first["default_model_source"] != "registry" {
		t.Fatalf("sources = %v/%v, want registry/registry", first["base_url_source"], first["default_model_source"])
	}
	if models, _ := first["models"].([]any); len(models) != 1 {
		t.Fatalf("models = %v, want the declared model list", first["models"])
	}

	rr = do(t, http.MethodGet, "/api/v1/media-providers?kind=image", "", f.List)
	if rr.Code != http.StatusOK {
		t.Fatalf("filtered status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	data, _ = decodeBody(t, rr)["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("filtered data = %v, want only the image kind", data)
	}
}

// TestMediaProviderHandler_ListRejectsAnUnknownKind pins the filter's refusal.
func TestMediaProviderHandler_ListRejectsAnUnknownKind(t *testing.T) {
	f, _ := newMediaProviderFixture(t)
	for _, kind := range []string{"web", "webSearch", "music"} {
		t.Run(kind, func(t *testing.T) {
			rr := do(t, http.MethodGet, "/api/v1/media-providers?kind="+kind, "", f.List)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), `"VALIDATION_ERROR"`) {
				t.Fatalf("body = %s, want the VALIDATION_ERROR code", rr.Body.String())
			}
		})
	}
}

// TestMediaProviderHandler_Get pins the detail and both not-found paths.
func TestMediaProviderHandler_Get(t *testing.T) {
	f, _ := newMediaProviderFixture(t)

	rr := do(t, http.MethodGet, "/api/v1/media-providers/openai", "", withPathValue(f.Get, "provider_id", "openai"))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["provider_id"] != "openai" || body["provider_name"] != "OpenAI" {
		t.Fatalf("identity = %v/%v, want openai/OpenAI", body["provider_id"], body["provider_name"])
	}
	if media, _ := body["media"].([]any); len(media) != 2 {
		t.Fatalf("media = %v, want the two kinds", body["media"])
	}

	rr = do(t, http.MethodGet, "/api/v1/media-providers/ghost", "", withPathValue(f.Get, "provider_id", "ghost"))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown provider status = %d, want 404 (body: %s)", rr.Code, rr.Body.String())
	}
	rr = do(t, http.MethodGet, "/api/v1/media-providers/chatonly", "", withPathValue(f.Get, "provider_id", "chatonly"))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("media-less provider status = %d, want 404 (body: %s)", rr.Code, rr.Body.String())
	}
}

// TestMediaProviderHandler_Patch pins the save, including that the answer
// reports the override as the effective source.
func TestMediaProviderHandler_Patch(t *testing.T) {
	f, repo := newMediaProviderFixture(t)
	body := `{"kind":"tts","base_url":"http://127.0.0.1:8000/v1","default_model":"tts-1"}`

	rr := do(t, http.MethodPatch, "/api/v1/media-providers/openai", body, withPathValue(f.Patch, "provider_id", "openai"))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	answer := decodeBody(t, rr)
	if answer["base_url"] != "http://127.0.0.1:8000/v1" || answer["base_url_source"] != "override" {
		t.Fatalf("base URL = %v (%v), want the override resolved", answer["base_url"], answer["base_url_source"])
	}
	if answer["default_model_source"] != "override" {
		t.Fatalf("default model source = %v, want override", answer["default_model_source"])
	}
	if len(repo.rows) != 1 {
		t.Fatalf("stored rows = %d, want 1", len(repo.rows))
	}
}

// TestMediaProviderHandler_PatchValidation pins every rejection the route owes
// its client.
func TestMediaProviderHandler_PatchValidation(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		body     string
	}{
		{name: "an unknown kind", provider: "openai", body: `{"kind":"music","base_url":"https://x.example.com"}`},
		{name: "a missing kind", provider: "openai", body: `{"base_url":"https://x.example.com"}`},
		{name: "a kind the provider does not offer", provider: "chatonly",
			body: `{"kind":"tts","base_url":"https://x.example.com"}`},
		{name: "a base URL of the wrong scheme", provider: "openai",
			body: `{"kind":"tts","base_url":"ftp://x.example.com"}`},
		{name: "a model the kind does not declare", provider: "openai",
			body: `{"kind":"tts","default_model":"ghost"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, repo := newMediaProviderFixture(t)
			rr := do(t, http.MethodPatch, "/api/v1/media-providers/"+tc.provider, tc.body,
				withPathValue(f.Patch, "provider_id", tc.provider))
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), `"VALIDATION_ERROR"`) {
				t.Fatalf("body = %s, want the VALIDATION_ERROR code", rr.Body.String())
			}
			if len(repo.rows) != 0 {
				t.Fatalf("a refused save wrote %d rows", len(repo.rows))
			}
		})
	}
}

// TestMediaProviderHandler_PatchUnknownProvider pins the not-found mapping.
func TestMediaProviderHandler_PatchUnknownProvider(t *testing.T) {
	f, _ := newMediaProviderFixture(t)
	rr := do(t, http.MethodPatch, "/api/v1/media-providers/ghost",
		`{"kind":"tts","base_url":"https://x.example.com"}`, withPathValue(f.Patch, "provider_id", "ghost"))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body: %s)", rr.Code, rr.Body.String())
	}
}

// TestMediaProviderHandler_MissingID covers the path-value guards.
func TestMediaProviderHandler_MissingID(t *testing.T) {
	f, _ := newMediaProviderFixture(t)
	cases := []struct {
		name   string
		method string
		body   string
		fn     http.HandlerFunc
	}{
		{name: "get", method: http.MethodGet, fn: f.Get},
		{name: "patch", method: http.MethodPatch, body: `{"kind":"tts"}`, fn: f.Patch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := do(t, tc.method, "/api/v1/media-providers/", tc.body, tc.fn)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
			}
		})
	}
}
