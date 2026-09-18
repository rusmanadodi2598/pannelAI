// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/model_writes_test.go
// @for       The alias and disabled set route tests for §7.6.
// @uses      internal/schema, net/http, net/http/httptest, strings, testing.
// @reason    The catalog/custom tests and the alias/disabled tests are separate groups; AGENTS.md §1.1 caps a file at 250 lines, so the set-replacement cases moved here.
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

// TestModelHandler_AliasesPut_Validation pins every rejection the alias route
// owes its client, including a target that does not resolve.
func TestModelHandler_AliasesPut_Validation(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "a missing alias", body: `{"aliases":[{"target":"openai/gpt-4o"}]}`},
		{name: "a missing target", body: `{"aliases":[{"alias":"quick"}]}`},
		{name: "an unresolvable target", body: `{"aliases":[{"alias":"quick","target":"openai/ghost"}]}`},
		{name: "a disabled target", body: `{"aliases":[{"alias":"quick","target":"anthropic/claude-3"}]}`},
		{name: "an alias shadowing a combo", body: `{"aliases":[{"alias":"seeded-combo","target":"openai/gpt-4o"}]}`},
		{name: "an unknown field", body: `{"aliases":[{"alias":"quick","target":"openai/gpt-4o","extra":1}]}`},
		{name: "a malformed body", body: `{"aliases":`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newManagementFixture(t)
			rr := do(t, http.MethodPut, "/api/v1/models/aliases", tc.body, f.model.AliasesPut)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
			}
		})
	}
}

// TestModelHandler_DisabledPutReplacesTheWholeSet pins the same PUT semantics
// for the disabled set.
func TestModelHandler_DisabledPutReplacesTheWholeSet(t *testing.T) {
	f := newManagementFixture(t)
	put := do(t, http.MethodPut, "/api/v1/models/disabled",
		`{"models":[{"provider_id":"openai","model_id":"gpt-4o-mini"}]}`, f.model.DisabledPut)
	if put.Code != http.StatusOK {
		t.Fatalf("put = %d (body: %s)", put.Code, put.Body.String())
	}
	body := put.Body.String()
	if !strings.Contains(body, `"gpt-4o-mini"`) || strings.Contains(body, `"claude-3"`) {
		t.Fatalf("the set was not replaced: %s", body)
	}

	// The newly disabled model disappears from the catalog.
	catalog := do(t, http.MethodGet, "/api/v1/models/catalog", "", f.model.Catalog)
	if strings.Contains(catalog.Body.String(), `"gpt-4o-mini"`) {
		t.Fatalf("a disabled model is still in the catalog: %s", catalog.Body.String())
	}
}

// TestModelHandler_DisabledPut_Validation pins the disabled route's rejections.
func TestModelHandler_DisabledPut_Validation(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "a missing provider", body: `{"models":[{"model_id":"gpt-4o"}]}`},
		{name: "a missing model", body: `{"models":[{"provider_id":"openai"}]}`},
		{name: "an unknown model", body: `{"models":[{"provider_id":"openai","model_id":"ghost"}]}`},
		{name: "a malformed body", body: `{"models":[`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newManagementFixture(t)
			rr := do(t, http.MethodPut, "/api/v1/models/disabled", tc.body, f.model.DisabledPut)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
			}
		})
	}
}

// TestModelHandler_DisabledGet_PreservesTheSeededSet keeps the read route honest
// about the set the fixture wrote.
func TestModelHandler_DisabledGet_PreservesTheSeededSet(t *testing.T) {
	f := newManagementFixture(t)
	rr := do(t, http.MethodGet, "/api/v1/models/disabled", "", f.model.DisabledGet)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"claude-3"`) {
		t.Fatalf("the seeded disabled set is missing: %s", rr.Body.String())
	}
}
