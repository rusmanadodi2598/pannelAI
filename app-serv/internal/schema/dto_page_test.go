// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/dto_page_test.go
// @for       The shared page and per_page query decoder every list route reads.
// @uses      net/http, net/http/httptest, testing.
// @reason    Draft 010 F6: DecodePage clamped per_page above 100 silently, so
//
//	the three layers disagreed (the panel refuses 1..100 in Zod, the
//	gateway clamped, the contract said nothing) and a caller could
//	not tell a narrowed answer from the page it asked for. Owner
//	decision D3 chose refusal, so the boundary now rejects what the
//	contract will declare: page >= 1 and 1 <= per_page <= 100, with
//	absent parameters keeping their documented defaults.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-22
package schema

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestDecodePage_BoundsIsRefusalNotClamp covers the decoder every paginated
// management route shares. The benign controls prove the legal values and the
// defaults still pass; the refusals are one behaviour: a page request outside
// the documented range is an error the caller can see, never a silently
// narrowed answer.
func TestDecodePage_BoundsIsRefusalNotClamp(t *testing.T) {
	cases := []struct {
		name        string
		query       string
		wantPage    int
		wantPerPage int
		wantErr     bool
	}{
		{name: "absent parameters keep the defaults", query: "", wantPage: 1, wantPerPage: DefaultPerPage},
		{name: "the smallest legal page and per_page", query: "?page=1&per_page=1", wantPage: 1, wantPerPage: 1},
		{name: "the largest legal per_page", query: "?per_page=100", wantPage: 1, wantPerPage: MaxPerPage},
		{name: "a mid-range page", query: "?page=3&per_page=25", wantPage: 3, wantPerPage: 25},
		{name: "page zero is refused", query: "?page=0", wantErr: true},
		{name: "a negative page is refused", query: "?page=-2", wantErr: true},
		{name: "a non-numeric page is refused", query: "?page=abc", wantErr: true},
		{name: "per_page zero is refused", query: "?per_page=0", wantErr: true},
		{name: "per_page one over the cap is refused", query: "?per_page=101", wantErr: true},
		{name: "a far-over per_page is refused, not clamped", query: "?per_page=99999", wantErr: true},
		{name: "a negative per_page is refused", query: "?per_page=-5", wantErr: true},
		{name: "a non-numeric per_page is refused", query: "?per_page=x", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/records"+tc.query, nil)
			page, perPage, err := DecodePage(req)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("DecodePage(%q) = (%d, %d, nil), want a validation error", tc.query, page, perPage)
				}
				if appErr := domain.AsAppError(err); appErr.Code != "VALIDATION_ERROR" {
					t.Fatalf("DecodePage(%q) error = %v, want a VALIDATION_ERROR", tc.query, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("DecodePage(%q) error = %v, want none", tc.query, err)
			}
			if page != tc.wantPage || perPage != tc.wantPerPage {
				t.Fatalf("DecodePage(%q) = (%d, %d), want (%d, %d)",
					tc.query, page, perPage, tc.wantPage, tc.wantPerPage)
			}
		})
	}
}
