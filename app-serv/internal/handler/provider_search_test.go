// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/provider_search_test.go
// @for       The `?q` parameter of GET /api/v1/providers: the wire behaviour
//
//	of the filter and its refusal for an over-long query.
//
// @uses      internal/registry, internal/service, net/http, net/http/httptest,
//
//	encoding/json, strings, testing.
//
// @reason    SPEC-UI §14 Q13 and PORT 002 §4.1: before `q` existed the route
//
//	answered the unfiltered registry to every spelling, so the failure
//	this file pins is silence rather than an error. The bound is
//	validated here because §1.4 of AGENTS.md puts external input
//	limits at the boundary, and 120 is the tag's own number.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-25
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// searchIndex is the seam double for the registry the list handler reads.
type searchIndex struct {
	entries []registry.Provider
}

func (i searchIndex) Provider(name string) (registry.Provider, bool) {
	for _, entry := range i.entries {
		if entry.ID == name {
			return entry, true
		}
	}
	return registry.Provider{}, false
}

func (i searchIndex) All() []registry.Provider { return append([]registry.Provider(nil), i.entries...) }

func (i searchIndex) Categories() []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(seen))
	for _, entry := range i.entries {
		if !seen[entry.Category] {
			seen[entry.Category] = true
			out = append(out, entry.Category)
		}
	}
	return out
}

// newSearchHandler wires the list handler over three providers whose names and
// ids differ, so a rule that read only one of the two fields fails a case.
func newSearchHandler(t *testing.T) *ProviderHandler {
	t.Helper()
	svc, err := service.NewProviderService(service.ProviderServiceDeps{Index: searchIndex{entries: []registry.Provider{
		{ID: "openai", Priority: 1, Category: "apikey", Display: registry.Display{Name: "OpenAI"}},
		{ID: "azure-openai", Priority: 2, Category: "apikey", Display: registry.Display{Name: "Azure OpenAI"}},
		{ID: "claude-local", Priority: 3, Category: "local", Display: registry.Display{Name: "Claude via Ollama"}},
	}}})
	if err != nil {
		t.Fatalf("NewProviderService() error = %v", err)
	}
	return NewProviderHandler(svc)
}

// listProviders drives the route with a raw query string and decodes the body.
func listProviders(t *testing.T, handler *ProviderHandler, query string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers"+query, nil)
	rr := httptest.NewRecorder()
	handler.List(rr, req)
	return rr
}

// TestProviderHandler_ListQueryParameter pins the parameter's wire contract:
// which spellings are accepted, what they answer, and that an over-long query
// is refused rather than silently dropped.
func TestProviderHandler_ListQueryParameter(t *testing.T) {
	handler := newSearchHandler(t)
	long := strings.Repeat("a", 121)
	atLimit := strings.Repeat("a", 120)

	cases := []struct {
		name   string
		query  string
		status int
		ids    []string
		code   string
	}{
		{name: "absent keeps the whole list", query: "", status: http.StatusOK,
			ids: []string{"openai", "azure-openai", "claude-local"}},
		{name: "an empty value reads as absent", query: "?q=", status: http.StatusOK,
			ids: []string{"openai", "azure-openai", "claude-local"}},
		{name: "a query narrows to its matches", query: "?q=openai", status: http.StatusOK,
			ids: []string{"openai", "azure-openai"}},
		{name: "case does not decide the match", query: "?q=OPENAI", status: http.StatusOK,
			ids: []string{"openai", "azure-openai"}},
		{name: "the value is trimmed like every other parameter", query: "?q=%20azure%20", status: http.StatusOK,
			ids: []string{"azure-openai"}},
		{name: "a name-only match still finds the row", query: "?q=ollama", status: http.StatusOK,
			ids: []string{"claude-local"}},
		{name: "no match answers an empty page", query: "?q=zzzz-nothing", status: http.StatusOK, ids: []string{}},
		{name: "the query composes with the category filter", query: "?q=openai&category=local", status: http.StatusOK, ids: []string{}},
		{name: "a query at the limit is accepted", query: "?q=" + atLimit, status: http.StatusOK, ids: []string{}},
		{name: "an over-long query is refused, not ignored", query: "?q=" + long, status: http.StatusBadRequest, code: "VALIDATION_ERROR"},
		{name: "an unknown routability is still refused beside a query", query: "?q=openai&routability=bogus", status: http.StatusBadRequest, code: "VALIDATION_ERROR"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := listProviders(t, handler, tc.query)
			if rr.Code != tc.status {
				t.Fatalf("status = %d, want %d (body %s)", rr.Code, tc.status, rr.Body.String())
			}
			if tc.status != http.StatusOK {
				var body struct {
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
					t.Fatalf("decode refusal: %v", err)
				}
				if body.Error.Code != tc.code {
					t.Fatalf("error code = %q, want %q", body.Error.Code, tc.code)
				}
				return
			}

			var body struct {
				Data []struct {
					ID string `json:"id"`
				} `json:"data"`
				Meta struct {
					Total int64 `json:"total"`
				} `json:"meta"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode list: %v", err)
			}
			if len(body.Data) != len(tc.ids) {
				t.Fatalf("ids = %d rows, want %d", len(body.Data), len(tc.ids))
			}
			for i, want := range tc.ids {
				if body.Data[i].ID != want {
					t.Fatalf("data[%d].id = %q, want %q", i, body.Data[i].ID, want)
				}
			}
			if body.Meta.Total != int64(len(tc.ids)) {
				t.Fatalf("meta.total = %d, want %d", body.Meta.Total, len(tc.ids))
			}
		})
	}
}

// TestProviderHandler_ListQueryNeverLeaksTheUnfilteredSet is the negative
// control for the failure that motivated the parameter: a query with no match
// must answer zero rows. An implementation that dropped the filter would still
// pass a happy-path case, so this case exists to fail it.
func TestProviderHandler_ListQueryNeverLeaksTheUnfilteredSet(t *testing.T) {
	handler := newSearchHandler(t)
	rr := listProviders(t, handler, "?q=zzzz-nothing")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if strings.Contains(rr.Body.String(), `"openai"`) {
		t.Fatalf("the unfiltered registry leaked into a no-match answer: %s", rr.Body.String())
	}
}
