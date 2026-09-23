// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/model_test.go
// @for       HTTP tests for the §7.6 model catalog routes. (first half; split at the AGENTS.md §1.1 line limit).
// @uses      internal/schema, net/http, net/http/httptest, strings, testing.
// @reason    AGENTS.md §2.1 requires a happy path and a validation-failure path
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-17
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// handlerNow is the instant the handler fixtures date rows with.
func handlerNow() time.Time { return time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC) }

// do runs one request against a handler method and returns the recorder.
func do(t *testing.T, method, target, body string, handler http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

// TestModelHandler_Catalog_HappyPath serves the merged catalog with filters.
func TestModelHandler_Catalog_HappyPath(t *testing.T) {
	f := newManagementFixture(t)
	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{name: "no filter", query: "", want: []string{"openai/gpt-4o", "openai/gpt-4o-mini"}},
		{name: "by provider", query: "?provider_id=openai", want: []string{"openai/gpt-4o", "openai/gpt-4o-mini"}},
		// The reference marks gpt-4o-mini vision-capable too (`*gpt-4o*`
		// matches it), so the filter answers both. The old expectation of one
		// row came from a fixture that wrote "vision" into its own data.
		{name: "by capability", query: "?capability=vision", want: []string{"openai/gpt-4o", "openai/gpt-4o-mini"}},
		{name: "by a media capability the document declares", query: "?capability=edit", want: []string{}},
		{name: "by free text", query: "?q=mini", want: []string{"openai/gpt-4o-mini"}},
		{name: "a disabled model is excluded", query: "?provider_id=anthropic", want: []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := do(t, http.MethodGet, "/api/v1/models/catalog"+tc.query, "", f.model.Catalog)
			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
			}
			body := decodeBody(t, rr)
			data, ok := body["data"].([]any)
			if !ok {
				t.Fatalf("response must carry a data array: %v", body)
			}
			if len(data) != len(tc.want) {
				t.Fatalf("data = %v, want %v", body["data"], tc.want)
			}
			for i, want := range tc.want {
				row, _ := data[i].(map[string]any)
				if id, _ := row["id"].(string); id != want {
					t.Fatalf("data[%d].id = %q, want %q", i, id, want)
				}
			}
		})
	}
}

// TestModelHandler_Catalog_EmptyMatchesNothing proves an unmatched filter is an
// empty array, not a null, so the panel renders its empty state.
func TestModelHandler_Catalog_EmptyMatchesNothing(t *testing.T) {
	f := newManagementFixture(t)
	rr := do(t, http.MethodGet, "/api/v1/models/catalog?q=nonexistent", "", f.model.Catalog)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"data":[]`) {
		t.Fatalf("body = %s, want an empty data array", rr.Body.String())
	}
}

// TestModelHandler_CustomCreate covers the happy path and every validation
// failure the schema layer can reject.
func TestModelHandler_CustomCreate(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		status int
	}{
		{name: "happy path", body: `{"provider_id":"openai","model_id":"gpt-5","display_name":"GPT-5"}`, status: http.StatusCreated},
		{name: "with capabilities", body: `{"provider_id":"openai","model_id":"gpt-5-vision","display_name":"GPT-5 Vision","capabilities":["vision"]}`, status: http.StatusCreated},
		{name: "missing provider", body: `{"model_id":"x","display_name":"X"}`, status: http.StatusBadRequest},
		{name: "empty model id", body: `{"provider_id":"openai","model_id":"","display_name":"X"}`, status: http.StatusBadRequest},
		{name: "missing display name", body: `{"provider_id":"openai","model_id":"x"}`, status: http.StatusBadRequest},
		{name: "unknown field", body: `{"provider_id":"openai","model_id":"x","display_name":"X","extra":1}`, status: http.StatusBadRequest},
		{name: "malformed json", body: `{"provider_id":}`, status: http.StatusBadRequest},
		{name: "an unknown provider", body: `{"provider_id":"nope","model_id":"x","display_name":"X"}`, status: http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newManagementFixture(t)
			rr := do(t, http.MethodPost, "/api/v1/models/custom", tc.body, f.model.CustomCreate)
			if rr.Code != tc.status {
				t.Fatalf("status = %d, want %d (body: %s)", rr.Code, tc.status, rr.Body.String())
			}
			if tc.status == http.StatusBadRequest {
				body := decodeBody(t, rr)
				errObj, ok := body["error"].(map[string]any)
				if !ok {
					t.Fatalf("response must carry the error envelope: %v", body)
				}
				if code, _ := errObj["code"].(string); code != "VALIDATION_ERROR" {
					t.Fatalf("code = %q, want VALIDATION_ERROR", code)
				}
			}
		})
	}
}

// TestModelHandler_CustomLifecycle runs create, list, and delete through the
// handlers, which is the path the panel drives.
func TestModelHandler_CustomLifecycle(t *testing.T) {
	f := newManagementFixture(t)
	created := do(t, http.MethodPost, "/api/v1/models/custom",
		`{"provider_id":"openai","model_id":"gpt-5","display_name":"GPT-5"}`, f.model.CustomCreate)
	if created.Code != http.StatusCreated {
		t.Fatalf("create = %d (body: %s)", created.Code, created.Body.String())
	}
	id, _ := decodeBody(t, created)["id"].(string)
	if !strings.HasPrefix(id, "mdl_") {
		t.Fatalf("created id = %q, want the mdl_ prefix", id)
	}

	listed := do(t, http.MethodGet, "/api/v1/models/custom", "", f.model.CustomList)
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), `"gpt-5"`) {
		t.Fatalf("list = %d (body: %s)", listed.Code, listed.Body.String())
	}

	deleted := do(t, http.MethodDelete, "/api/v1/models/custom/"+id, "", func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("id", id)
		f.model.CustomDelete(w, r)
	})
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete = %d, want 204 (body: %s)", deleted.Code, deleted.Body.String())
	}
	after := do(t, http.MethodGet, "/api/v1/models/custom", "", f.model.CustomList)
	if strings.Contains(after.Body.String(), `"gpt-5"`) {
		t.Fatalf("the deleted model is still listed: %s", after.Body.String())
	}
}

// TestModelHandler_CustomDelete_UnknownModel maps a missing row onto 404.
func TestModelHandler_CustomDelete_UnknownModel(t *testing.T) {
	f := newManagementFixture(t)
	rr := do(t, http.MethodDelete, "/api/v1/models/custom/mdl_missing", "", func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("id", "mdl_missing")
		f.model.CustomDelete(w, r)
	})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body: %s)", rr.Code, rr.Body.String())
	}
}

// TestModelHandler_AliasesPutReplacesTheWholeSet pins PUT semantics: the stored
// set equals the request, with nothing merged from before.
func TestModelHandler_AliasesPutReplacesTheWholeSet(t *testing.T) {
	f := newManagementFixture(t)
	put := do(t, http.MethodPut, "/api/v1/models/aliases",
		`{"aliases":[{"alias":"quick","target":"openai/gpt-4o-mini"}]}`, f.model.AliasesPut)
	if put.Code != http.StatusOK {
		t.Fatalf("put = %d (body: %s)", put.Code, put.Body.String())
	}
	got := do(t, http.MethodGet, "/api/v1/models/aliases", "", f.model.AliasesGet)
	if got.Code != http.StatusOK {
		t.Fatalf("get = %d", got.Code)
	}
	body := got.Body.String()
	if !strings.Contains(body, `"quick"`) {
		t.Fatalf("the new alias is missing: %s", body)
	}
	if strings.Contains(body, `"fast"`) {
		t.Fatalf("the previous alias survived a PUT: %s", body)
	}

	cleared := do(t, http.MethodPut, "/api/v1/models/aliases", `{"aliases":[]}`, f.model.AliasesPut)
	if cleared.Code != http.StatusOK || !strings.Contains(cleared.Body.String(), `"data":[]`) {
		t.Fatalf("clearing = %d (body: %s)", cleared.Code, cleared.Body.String())
	}
}
