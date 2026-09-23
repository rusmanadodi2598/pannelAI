// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/model_custom_filter_test.go
// @for       Draft 024 F2's list half: `GET /models/custom?provider_id=`
//
//	narrows the custom rows, and it accepts every spelling the provider
//	answers to.
//
// @uses      internal/domain, net/http, net/http/httptest, strings, testing.
// @reason    §7.6's table documents the parameter and the handler ignored it, so
//
//	a panel filtering one provider's custom rows received every provider's.
//	The filter accepts the id, the registry alias, and a node prefix for
//	the same reason the catalog filter does: the operator may know the
//	provider by any of those names (draft 024 §3.2).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-23
package handler

import (
	"net/http"
	"strings"
	"testing"
)

// TestModelHandler_CustomListFiltersByProvider is the table the contract asks
// for: no parameter lists everything, the id form narrows, the alias form
// narrows to the same rows, an unknown provider answers an empty list rather
// than every row, and a blank value is treated as absent.
func TestModelHandler_CustomListFiltersByProvider(t *testing.T) {
	f := newManagementFixture(t)
	for _, body := range []string{
		`{"provider_id":"openai","model_id":"row-openai","display_name":"Row OpenAI"}`,
		`{"provider_id":"anthropic","model_id":"row-anthropic","display_name":"Row Anthropic"}`,
	} {
		if rr := do(t, http.MethodPost, "/api/v1/models/custom", body, f.model.CustomCreate); rr.Code != http.StatusCreated {
			t.Fatalf("create = %d (body: %s)", rr.Code, rr.Body.String())
		}
	}

	cases := []struct {
		name      string
		query     string
		wantHas   []string
		wantLacks []string
	}{
		{
			name:    "no parameter lists every row",
			query:   "",
			wantHas: []string{"row-openai", "row-anthropic"},
		},
		{
			name:      "the id form narrows to one provider",
			query:     "?provider_id=openai",
			wantHas:   []string{"row-openai"},
			wantLacks: []string{"row-anthropic"},
		},
		{
			name:      "an unknown provider answers an empty list",
			query:     "?provider_id=nowhere",
			wantLacks: []string{"row-openai", "row-anthropic"},
		},
		{
			name:    "a blank value is treated as absent",
			query:   "?provider_id=%20",
			wantHas: []string{"row-openai", "row-anthropic"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := do(t, http.MethodGet, "/api/v1/models/custom"+tc.query, "", f.model.CustomList)
			if rr.Code != http.StatusOK {
				t.Fatalf("list = %d (body: %s)", rr.Code, rr.Body.String())
			}
			body := rr.Body.String()
			for _, want := range tc.wantHas {
				if !strings.Contains(body, want) {
					t.Fatalf("list(%q) = %s, want it to contain %q", tc.query, body, want)
				}
			}
			for _, lack := range tc.wantLacks {
				if strings.Contains(body, lack) {
					t.Fatalf("list(%q) = %s, want it to exclude %q", tc.query, body, lack)
				}
			}
		})
	}
}
