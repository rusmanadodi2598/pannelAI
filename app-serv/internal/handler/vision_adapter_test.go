// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/vision_adapter_test.go
// @for       HTTP tests for the §7.8 vision adapter routes.
// @uses      net/http, strings, testing.
// @reason    AGENTS.md §2.1 requires a happy path and a validation-failure path
//
//	per route. §7.8's write is a whole replacement whose models come
//	from the catalog, and the capability table is not in the registry
//	yet, so the tests pin the exact refusals an operator sees today.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-17
package handler

import (
	"net/http"
	"strings"
	"testing"
)

// TestVisionAdapterHandler_GetServesTheDefault documents the fresh-install
// payload: a typed shape with an empty model list, never a 404.
func TestVisionAdapterHandler_GetServesTheDefault(t *testing.T) {
	f := newManagementFixture(t)
	rr := do(t, http.MethodGet, "/api/v1/vision-adapter", "", f.vision.Get)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if enabled, _ := body["enabled"].(bool); enabled {
		t.Fatalf("enabled = true, want the disabled default: %v", body)
	}
	models, ok := body["models"].([]any)
	if !ok || len(models) != 0 {
		t.Fatalf("models = %v, want an empty array", body["models"])
	}
}

// TestVisionAdapterHandler_Put covers every rejection the route owes its client.
//
// The capability predicate is the service's placeholder until the registry
// carries capability data, so every model is refused as not vision-capable; the
// reachable failures are therefore pinned here exactly as an operator meets
// them, and the seam's future state is covered by the service tests.
func TestVisionAdapterHandler_Put(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		status int
	}{
		{name: "disabling with no models", body: `{"enabled":false,"round_robin":false,"models":[]}`, status: http.StatusOK},
		{name: "a missing models field", body: `{"enabled":false,"round_robin":false}`, status: http.StatusBadRequest},
		{name: "a model outside the catalog", body: `{"enabled":true,"round_robin":false,"models":["openai/ghost"]}`, status: http.StatusBadRequest},
		{name: "a model that is not provider/model", body: `{"enabled":true,"round_robin":false,"models":["gpt-4o"]}`, status: http.StatusBadRequest},
		{name: "a model the capability check refuses", body: `{"enabled":true,"round_robin":false,"models":["openai/gpt-4o"]}`, status: http.StatusBadRequest},
		{name: "a disabled catalog model", body: `{"enabled":true,"round_robin":false,"models":["anthropic/claude-3"]}`, status: http.StatusBadRequest},
		{name: "an unknown field", body: `{"enabled":false,"round_robin":false,"models":[],"extra":1}`, status: http.StatusBadRequest},
		{name: "a malformed body", body: `{"enabled":`, status: http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newManagementFixture(t)
			rr := do(t, http.MethodPut, "/api/v1/vision-adapter", tc.body, f.vision.Put)
			if rr.Code != tc.status {
				t.Fatalf("status = %d, want %d (body: %s)", rr.Code, tc.status, rr.Body.String())
			}
			if tc.status != http.StatusBadRequest {
				return
			}
			body := decodeBody(t, rr)
			errObj, ok := body["error"].(map[string]any)
			if !ok {
				t.Fatalf("response must carry the error envelope: %v", body)
			}
			if code, _ := errObj["code"].(string); code != "VALIDATION_ERROR" {
				t.Fatalf("code = %q, want VALIDATION_ERROR", code)
			}
			if msg, _ := errObj["message"].(string); msg == "" {
				t.Fatal("error message must not be empty")
			}
		})
	}
}

// TestVisionAdapterHandler_PutRoundTrip proves a configuration the service
// accepts comes back out of GET unchanged, which the panel's save relies on.
func TestVisionAdapterHandler_PutRoundTrip(t *testing.T) {
	f := newManagementFixture(t)
	// Round-trip the disabled state, which is the shape this build can accept
	// without capability data.
	put := do(t, http.MethodPut, "/api/v1/vision-adapter",
		`{"enabled":false,"round_robin":true,"models":[]}`, f.vision.Put)
	if put.Code != http.StatusOK {
		t.Fatalf("put = %d (body: %s)", put.Code, put.Body.String())
	}
	if !strings.Contains(put.Body.String(), `"round_robin":true`) {
		t.Fatalf("put response = %s, want round_robin:true", put.Body.String())
	}
	got := do(t, http.MethodGet, "/api/v1/vision-adapter", "", f.vision.Get)
	if got.Code != http.StatusOK {
		t.Fatalf("get = %d (body: %s)", got.Code, got.Body.String())
	}
	if !strings.Contains(got.Body.String(), `"round_robin":true`) {
		t.Fatalf("get response = %s, want the stored round_robin:true", got.Body.String())
	}
	if !strings.Contains(got.Body.String(), `"updated_at"`) {
		t.Fatalf("get response = %s, want updated_at after a write", got.Body.String())
	}
}
