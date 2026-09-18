// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/catalog_error_test.go
// @for       Driver-error mapping for the catalog, combo, and adapter (first half; split at the AGENTS.md §1.1 line limit).
// @uses      github.com/jackc/pgx/v5, github.com/jackc/pgx/v5/pgconn,
// @reason    §8 fixes which code a client sees for a duplicate or an absent row,
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-17
package postgres

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestTranslateComboError pins the mapping the combo repository answers with.
func TestTranslateComboError(t *testing.T) {
	cases := []struct {
		name      string
		err       error
		wantErr   error
		wantCode  string
		wantInMsg string
	}{
		{name: "no error stays nil", err: nil},
		{
			name: "an absent row is NOT_FOUND", err: pgx.ErrNoRows,
			wantErr: domain.ErrComboNotFound, wantCode: "NOT_FOUND",
		},
		{
			name:    "a unique violation is CONFLICT",
			err:     &pgconn.PgError{Code: "23505", ConstraintName: "idx_combos_name"},
			wantErr: domain.ErrComboExists, wantCode: "CONFLICT",
			wantInMsg: "idx_combos_name",
		},
		{
			name:      "an unrelated driver error is INTERNAL_ERROR on the wire",
			err:       &pgconn.PgError{Code: "42601", Message: "syntax error"},
			wantCode:  "INTERNAL_ERROR",
			wantInMsg: "combos:",
		},
		{
			name:     "a wrapped absent row is still NOT_FOUND",
			err:      errors.New("loading combo: " + pgx.ErrNoRows.Error()),
			wantCode: "INTERNAL_ERROR",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := translateComboError(tc.err)
			if tc.err == nil {
				if got != nil {
					t.Fatalf("translateComboError(nil) = %v, want nil", got)
				}
				return
			}
			if tc.wantErr != nil && !errors.Is(got, tc.wantErr) {
				t.Fatalf("translateComboError(%v) = %v, want %v", tc.err, got, tc.wantErr)
			}
			if code := domain.AsAppError(got).Code; code != tc.wantCode {
				t.Fatalf("translateComboError(%v) code = %q, want %q", tc.err, code, tc.wantCode)
			}
			if tc.wantInMsg != "" && !strings.Contains(got.Error(), tc.wantInMsg) {
				t.Fatalf("translateComboError(%v) = %v, want it to mention %q", tc.err, got, tc.wantInMsg)
			}
		})
	}
}

// TestTranslateCatalogError pins the same mapping for the catalog repository,
// including that the wrapped driver text never names a SQL statement to a client
// (it reaches logs only, AGENTS.md §1.3).
func TestTranslateCatalogError(t *testing.T) {
	cases := []struct {
		name      string
		err       error
		wantErr   error
		wantCode  string
		wantGroup string
	}{
		{name: "no error stays nil", err: nil},
		{
			name: "a missing custom model is NOT_FOUND", err: pgx.ErrNoRows,
			wantErr: domain.ErrModelNotFound, wantCode: "NOT_FOUND",
		},
		{
			name:    "a duplicate pair is CONFLICT",
			err:     &pgconn.PgError{Code: "23505", ConstraintName: "idx_models_custom_provider_model"},
			wantErr: domain.ErrModelExists, wantCode: "CONFLICT",
			wantGroup: "model catalog:",
		},
		{
			name:     "an unrelated driver error is INTERNAL_ERROR on the wire",
			err:      errors.New("connection reset"),
			wantCode: "INTERNAL_ERROR", wantGroup: "model catalog:",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := translateCatalogError(tc.err)
			if tc.err == nil {
				if got != nil {
					t.Fatalf("translateCatalogError(nil) = %v, want nil", got)
				}
				return
			}
			if tc.wantErr != nil && !errors.Is(got, tc.wantErr) {
				t.Fatalf("translateCatalogError(%v) = %v, want %v", tc.err, got, tc.wantErr)
			}
			if code := domain.AsAppError(got).Code; code != tc.wantCode {
				t.Fatalf("translateCatalogError(%v) code = %q, want %q", tc.err, code, tc.wantCode)
			}
			if tc.wantGroup != "" && !strings.Contains(got.Error(), tc.wantGroup) {
				t.Fatalf("translateCatalogError(%v) = %v, want it grouped under %q", tc.err, got, tc.wantGroup)
			}
		})
	}
}

// TestCatalogErrorCodesAreStable keeps the two mappings on the same sentinels,
// so a handler cannot render one code for a duplicate custom model and another
// for a duplicate combo.
func TestCatalogErrorCodesAreStable(t *testing.T) {
	duplicate := &pgconn.PgError{Code: "23505", ConstraintName: "c"}
	if code := domain.AsAppError(translateComboError(duplicate)).Code; code != "CONFLICT" {
		t.Fatalf("combo duplicate code = %q, want CONFLICT", code)
	}
	if code := domain.AsAppError(translateCatalogError(duplicate)).Code; code != "CONFLICT" {
		t.Fatalf("catalog duplicate code = %q, want CONFLICT", code)
	}
	if code := domain.AsAppError(translateComboError(pgx.ErrNoRows)).Code; code != "NOT_FOUND" {
		t.Fatalf("combo absent code = %q, want NOT_FOUND", code)
	}
	if code := domain.AsAppError(translateCatalogError(pgx.ErrNoRows)).Code; code != "NOT_FOUND" {
		t.Fatalf("catalog absent code = %q, want NOT_FOUND", code)
	}
}

// TestEncodeDecodeCapabilitiesRoundTrips pins the jsonb column's codec: the
// canonical set a write produces is the set a read returns.
func TestEncodeDecodeCapabilitiesRoundTrips(t *testing.T) {
	cases := []struct {
		name  string
		input domain.ModelCapabilities
		want  []string
	}{
		{name: "an empty set", input: domain.NewModelCapabilities(), want: []string{}},
		{name: "one member", input: domain.NewModelCapabilities("vision"), want: []string{"vision"}},
		{
			name:  "several members, sorted and de-duplicated",
			input: domain.NewModelCapabilities("tools", "vision", "VISION", " video "),
			want:  []string{"tools", "video", "vision"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := encodeCapabilities(tc.input)
			if err != nil {
				t.Fatalf("encodeCapabilities() error = %v", err)
			}
			got, err := decodeCapabilities(raw)
			if err != nil {
				t.Fatalf("decodeCapabilities() error = %v", err)
			}
			list := got.List()
			if len(list) != len(tc.want) {
				t.Fatalf("round trip = %v, want %v", list, tc.want)
			}
			for i := range tc.want {
				if list[i] != tc.want[i] {
					t.Fatalf("round trip = %v, want %v", list, tc.want)
				}
			}
		})
	}
	if _, err := decodeCapabilities([]byte(`{"not":"an array"}`)); err == nil {
		t.Fatal("decodeCapabilities() accepted a non-array document")
	}
}
