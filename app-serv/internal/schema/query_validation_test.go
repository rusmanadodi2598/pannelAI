// Package schema defines the typed request and response contracts of the API.
//
// @file      internal/schema/query_validation_test.go
// @for       The bounds on the list filters that arrive as query parameters.
// @uses      strings, testing
// @reason    These values were passed to the service trimmed only, so an over-long or nonsense capability read as a narrowed filter while narrowing nothing (the failure draft 025 F5 recording for `?active=`).
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability stable
// @since     2026-10-04
package schema

import (
	"strings"
	"testing"
)

func TestValidateStruct_CatalogListQuery(t *testing.T) {
	cases := []struct {
		name    string
		query   CatalogListQuery
		wantErr bool
	}{
		{name: "an empty filter is accepted", query: CatalogListQuery{}},
		{
			name:  "a normal filter is accepted",
			query: CatalogListQuery{ProviderID: "openai", Capability: "vision", Query: "flash"},
		},
		{
			name:    "a provider id past the bound is refused",
			query:   CatalogListQuery{ProviderID: strings.Repeat("p", 65)},
			wantErr: true,
		},
		{
			name:    "a capability past the bound is refused",
			query:   CatalogListQuery{Capability: strings.Repeat("c", 33)},
			wantErr: true,
		},
		{
			name:    "a search longer than any model name is refused",
			query:   CatalogListQuery{Query: strings.Repeat("q", 201)},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateStruct(tc.query)
			if tc.wantErr && err == nil {
				t.Fatalf("ValidateStruct() accepted %+v", tc.query)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("ValidateStruct() = %v, want the filter accepted", err)
			}
		})
	}
}
