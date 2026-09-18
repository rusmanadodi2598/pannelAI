// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/combo_test.go
// @for       HTTP tests for the §7.7 combo routes.
// @uses      net/http, net/http/httptest, strings, testing.
// @reason    AGENTS.md §2.1 requires a happy path and a validation-failure path
//
//	per route, and §7.7's strategy rules are the rejections a client
//	meets most often; the tests pin both the accepted shapes and the
//	refusals through the real mux-compatible handler methods.
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

// comboBody builds a combo request body.
func comboBody(name, strategy, models, extra string) string {
	body := `{"name":"` + name + `","strategy":"` + strategy + `","models":` + models
	if extra != "" {
		body += "," + extra
	}
	return body + "}"
}

// TestComboHandler_Create_HappyPath covers one accepted shape per strategy and
// per reference form §7.7 allows.
func TestComboHandler_Create_HappyPath(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "fallback on a model", body: comboBody("daily", "fallback", `[{"ref":"openai/gpt-4o","priority":1}]`, "")},
		{name: "fallback on an alias", body: comboBody("aliased", "fallback", `[{"ref":"fast","priority":1}]`, "")},
		{name: "fallback on another combo", body: comboBody("nested", "fallback", `[{"ref":"seeded-combo","priority":1}]`, "")},
		{
			name: "round_robin with a sticky limit",
			body: comboBody("rotating", "round_robin", `[{"ref":"openai/gpt-4o","priority":1},{"ref":"openai/gpt-4o-mini","priority":2}]`, `"sticky_limit":2`),
		},
		{
			name: "fusion with a judge model",
			body: comboBody("panel", "fusion", `[{"ref":"openai/gpt-4o","priority":1}]`, `"judge_model":"openai/gpt-4o-mini"`),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newManagementFixture(t)
			rr := do(t, http.MethodPost, "/api/v1/combos", tc.body, f.combo.Create)
			if rr.Code != http.StatusCreated {
				t.Fatalf("status = %d, want 201 (body: %s)", rr.Code, rr.Body.String())
			}
			body := decodeBody(t, rr)
			if id, _ := body["id"].(string); !strings.HasPrefix(id, "cmb_") {
				t.Fatalf("id = %q, want the cmb_ prefix", id)
			}
			if _, ok := body["models"].([]any); !ok {
				t.Fatalf("response must carry the ordered model list: %v", body)
			}
		})
	}
}

// TestComboHandler_Create_Validation pins every rejection the route owes its
// client: the schema failures and the strategy rules.
func TestComboHandler_Create_Validation(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "a missing name", body: `{"strategy":"fallback","models":[{"ref":"openai/gpt-4o","priority":1}]}`},
		{name: "a missing strategy", body: `{"name":"daily","models":[{"ref":"openai/gpt-4o","priority":1}]}`},
		{name: "an unknown strategy", body: comboBody("daily", "sequential", `[{"ref":"openai/gpt-4o","priority":1}]`, "")},
		{name: "no models", body: `{"name":"empty","strategy":"fallback","models":[]}`},
		{name: "a missing models field", body: `{"name":"empty","strategy":"fallback"}`},
		{name: "a model entry without a ref", body: comboBody("daily", "fallback", `[{"priority":1}]`, "")},
		{name: "a negative priority", body: comboBody("daily", "fallback", `[{"ref":"openai/gpt-4o","priority":-1}]`, "")},
		{name: "an unresolvable ref", body: comboBody("daily", "fallback", `[{"ref":"openai/ghost","priority":1}]`, "")},
		{name: "a disabled ref", body: comboBody("daily", "fallback", `[{"ref":"anthropic/claude-3","priority":1}]`, "")},
		{name: "fusion without a judge", body: comboBody("panel", "fusion", `[{"ref":"openai/gpt-4o","priority":1}]`, "")},
		{name: "fusion with a sticky limit", body: comboBody("panel", "fusion", `[{"ref":"openai/gpt-4o","priority":1}]`, `"judge_model":"openai/gpt-4o","sticky_limit":2`)},
		{name: "round_robin without a sticky limit", body: comboBody("rotating", "round_robin", `[{"ref":"openai/gpt-4o","priority":1}]`, "")},
		{name: "round_robin with a judge", body: comboBody("rotating", "round_robin", `[{"ref":"openai/gpt-4o","priority":1}]`, `"sticky_limit":1,"judge_model":"openai/gpt-4o"`)},
		{name: "fallback with a judge", body: comboBody("daily", "fallback", `[{"ref":"openai/gpt-4o","priority":1}]`, `"judge_model":"openai/gpt-4o"`)},
		{name: "an invalid name", body: comboBody("bad name", "fallback", `[{"ref":"openai/gpt-4o","priority":1}]`, "")},
		{name: "a duplicated ref", body: comboBody("daily", "fallback", `[{"ref":"openai/gpt-4o","priority":1},{"ref":"openai/gpt-4o","priority":2}]`, "")},
		{name: "an unknown field", body: `{"name":"daily","strategy":"fallback","models":[{"ref":"openai/gpt-4o","priority":1}],"extra":1}`},
		{name: "a malformed body", body: `{"name":`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newManagementFixture(t)
			rr := do(t, http.MethodPost, "/api/v1/combos", tc.body, f.combo.Create)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
			}
			body := decodeBody(t, rr)
			errObj, ok := body["error"].(map[string]any)
			if !ok {
				t.Fatalf("response must carry the error envelope: %v", body)
			}
			if code, _ := errObj["code"].(string); code != "VALIDATION_ERROR" {
				t.Fatalf("code = %q, want VALIDATION_ERROR", code)
			}
		})
	}
}

// TestComboHandler_Create_DuplicateName maps the uniqueness violation onto 409.
func TestComboHandler_Create_DuplicateName(t *testing.T) {
	f := newManagementFixture(t)
	body := comboBody("seeded-combo", "fallback", `[{"ref":"openai/gpt-4o","priority":1}]`, "")
	rr := do(t, http.MethodPost, "/api/v1/combos", body, f.combo.Create)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body: %s)", rr.Code, rr.Body.String())
	}
}

// TestComboHandler_Lifecycle runs list, detail, update, and delete through the
// handlers.
func TestComboHandler_Lifecycle(t *testing.T) {
	f := newManagementFixture(t)
	created := do(t, http.MethodPost, "/api/v1/combos",
		comboBody("daily", "fallback", `[{"ref":"openai/gpt-4o","priority":1}]`, ""), f.combo.Create)
	if created.Code != http.StatusCreated {
		t.Fatalf("create = %d (body: %s)", created.Code, created.Body.String())
	}
	id, _ := decodeBody(t, created)["id"].(string)

	listed := do(t, http.MethodGet, "/api/v1/combos", "", f.combo.List)
	if listed.Code != http.StatusOK {
		t.Fatalf("list = %d (body: %s)", listed.Code, listed.Body.String())
	}
	if !strings.Contains(listed.Body.String(), `"total":2`) {
		t.Fatalf("list must report the total: %s", listed.Body.String())
	}

	detail := do(t, http.MethodGet, "/api/v1/combos/"+id, "", withPathID(f.combo.Get, id))
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"daily"`) {
		t.Fatalf("detail = %d (body: %s)", detail.Code, detail.Body.String())
	}

	updated := do(t, http.MethodPatch, "/api/v1/combos/"+id,
		comboBody("daily", "round_robin", `[{"ref":"openai/gpt-4o","priority":1}]`, `"sticky_limit":3`),
		withPathID(f.combo.Update, id))
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), `"round_robin"`) {
		t.Fatalf("update = %d (body: %s)", updated.Code, updated.Body.String())
	}

	deleted := do(t, http.MethodDelete, "/api/v1/combos/"+id, "", withPathID(f.combo.Delete, id))
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete = %d, want 204 (body: %s)", deleted.Code, deleted.Body.String())
	}
	missing := do(t, http.MethodGet, "/api/v1/combos/"+id, "", withPathID(f.combo.Get, id))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("get after delete = %d, want 404", missing.Code)
	}
}

// TestComboHandler_Delete_ConflictWhileAnAliasReferencesIt pins the §7.7 delete
// rule: an alias must be cleaned first and the refusal is CONFLICT.
func TestComboHandler_Delete_ConflictWhileAnAliasReferencesIt(t *testing.T) {
	f := newManagementFixture(t)
	put := do(t, http.MethodPut, "/api/v1/models/aliases",
		`{"aliases":[{"alias":"daily-alias","target":"seeded-combo"}]}`, f.model.AliasesPut)
	if put.Code != http.StatusOK {
		t.Fatalf("seeding the alias = %d (body: %s)", put.Code, put.Body.String())
	}
	rr := do(t, http.MethodDelete, "/api/v1/combos/cmb_seeded", "", withPathID(f.combo.Delete, "cmb_seeded"))
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "CONFLICT") {
		t.Fatalf("body = %s, want the CONFLICT code", rr.Body.String())
	}
}

// TestComboHandler_Update_Validation proves a patch cannot reach a shape a
// create refuses.
func TestComboHandler_Update_Validation(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "fusion without a judge", body: comboBody("seeded-combo", "fusion", `[{"ref":"openai/gpt-4o","priority":1}]`, "")},
		{name: "round_robin without a sticky limit", body: comboBody("seeded-combo", "round_robin", `[{"ref":"openai/gpt-4o","priority":1}]`, "")},
		{name: "an unresolvable ref", body: comboBody("seeded-combo", "fallback", `[{"ref":"openai/ghost","priority":1}]`, "")},
		{name: "no models", body: `{"name":"seeded-combo","strategy":"fallback","models":[]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newManagementFixture(t)
			rr := do(t, http.MethodPatch, "/api/v1/combos/cmb_seeded", tc.body, withPathID(f.combo.Update, "cmb_seeded"))
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
			}
		})
	}
}

// TestComboHandler_MissingID covers the path-value guard on every {id} route.
func TestComboHandler_MissingID(t *testing.T) {
	f := newManagementFixture(t)
	cases := []struct {
		name string
		fn   http.HandlerFunc
	}{
		{"get", f.combo.Get},
		{"delete", f.combo.Delete},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := do(t, http.MethodGet, "/api/v1/combos/", "", tc.fn)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
			}
		})
	}
}

// withPathID wraps a handler method with the {id} path value a Go 1.22 mux
// would populate; these handler-level tests call the method directly.
func withPathID(fn http.HandlerFunc, id string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("id", id)
		fn(w, r)
	}
}
