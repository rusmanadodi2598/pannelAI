// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/model_active_test.go
// @for       The `?active=` query parameter of GET /models/catalog: the two
//
//	spellings the boundary accepts, and the refusal for every other one.
//
// @uses      net/http, net/http/httptest, testing.
// @reason    Draft 025 F5: a boolean parameter that ignores a misspelling
//
//	silently answers the unfiltered catalog under a parameter that
//	promised the opposite — the same failure the usage status filter
//	had before it became a closed set, where `SUCCESS` read as "no
//	failed requests". The house precedent is the provider list's
//	`routability` and the page decoder's `per_page`: a value outside
//	the documented vocabulary is a VALIDATION_ERROR, so the panel's
//	Zod layer and the gateway agree about what the words mean.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-24
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestModelHandler_CatalogActiveParameter pins the wire contract of the
// parameter: `true` and `false` are the accepted vocabulary, absence narrows
// nothing, and anything else is a 400 rather than a silently ignored filter.
//
// The fixture wires no endpoint-status counter, so an accepted `true` reaches
// the service and is answered with the named INTERNAL_ERROR refusal; the
// narrowed answer itself is pinned by the service test, which can wire a
// counter. What this table pins is the boundary's own behaviour: which
// spellings are accepted, which are refused, and that the accepted ones are
// passed through rather than dropped.
func TestModelHandler_CatalogActiveParameter(t *testing.T) {
	f := newManagementFixture(t)
	cases := []struct {
		name   string
		query  string
		status int
		want   []string
	}{
		{name: "absent keeps the whole catalog", query: "", status: http.StatusOK,
			want: []string{"openai/gpt-4o", "openai/gpt-4o-mini"}},
		{name: "false does not narrow", query: "?active=false", status: http.StatusOK,
			want: []string{"openai/gpt-4o", "openai/gpt-4o-mini"}},
		{name: "true reaches the service and is refused without the counter",
			query: "?active=true", status: http.StatusInternalServerError},
		{name: "the value is trimmed like every other parameter",
			query: "?active=%20true%20", status: http.StatusInternalServerError},
		{name: "a misspelling is refused, not ignored", query: "?active=yes", status: http.StatusBadRequest},
		{name: "the integer spellings are refused too", query: "?active=1", status: http.StatusBadRequest},
		// An empty value reads as absent, which is the house rule every other
		// query parameter follows (DecodePage: `v != ""` is the presence
		// test), so the parameter cannot be the one place where a hand-typed
		// `?active=` changes meaning.
		{name: "an empty value reads as absent", query: "?active=", status: http.StatusOK,
			want: []string{"openai/gpt-4o", "openai/gpt-4o-mini"}},
		{name: "uppercase is refused", query: "?active=TRUE", status: http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := do(t, http.MethodGet, "/api/v1/models/catalog"+tc.query, "", f.model.Catalog)
			if rr.Code != tc.status {
				t.Fatalf("status = %d, want %d (body: %s)", rr.Code, tc.status, rr.Body.String())
			}
			if tc.status != http.StatusOK {
				return
			}
			if got := catalogIDs(t, rr); !equalStringSlices(got, tc.want) {
				t.Fatalf("data ids = %v, want %v", got, tc.want)
			}
		})
	}
}

// catalogIDs reads the id list out of a catalog response, in the order the
// answer carried them.
func catalogIDs(t *testing.T, rr *httptest.ResponseRecorder) []string {
	t.Helper()
	body := decodeBody(t, rr)
	data, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("response must carry a data array: %v", body)
	}
	ids := make([]string, 0, len(data))
	for _, row := range data {
		entry, _ := row.(map[string]any)
		id, _ := entry["id"].(string)
		ids = append(ids, id)
	}
	return ids
}

// equalStringSlices is the local order-sensitive comparison, so the table
// asserts the answer's order rather than only its membership.
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if strings.TrimSpace(a[i]) != strings.TrimSpace(b[i]) {
			return false
		}
	}
	return true
}
