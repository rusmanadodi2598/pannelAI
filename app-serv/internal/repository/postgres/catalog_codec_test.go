// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/catalog_codec_test.go
// @for       The jsonb codec tests for the catalog and combo repositories.
// @uses      github.com/jackc/pgx/v5, github.com/jackc/pgx/v5/pgconn,
// @reason    The driver-error mapping tests and the codec tests are separate groups; AGENTS.md §1.1 caps a file at 250 lines, so the codec cases moved here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-17
package postgres

import (
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestEncodeDecodeComboModelsRoundTrips pins the combos.models codec, including
// that the ordered list survives with its priorities.
func TestEncodeDecodeComboModelsRoundTrips(t *testing.T) {
	cases := []struct {
		name  string
		input []domain.ComboModel
		want  []string
	}{
		{name: "an empty list", input: []domain.ComboModel{}, want: []string{}},
		{
			name:  "one entry",
			input: []domain.ComboModel{domain.RehydrateComboModel("openai/gpt-4o", 1)},
			want:  []string{"openai/gpt-4o"},
		},
		{
			name: "several entries keep their order",
			input: []domain.ComboModel{
				domain.RehydrateComboModel("openai/gpt-4o", 1),
				domain.RehydrateComboModel("seeded-combo", 2),
				domain.RehydrateComboModel("fast", 3),
			},
			want: []string{"openai/gpt-4o", "seeded-combo", "fast"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := encodeComboModels(tc.input)
			if err != nil {
				t.Fatalf("encodeComboModels() error = %v", err)
			}
			got, err := decodeComboModels(raw)
			if err != nil {
				t.Fatalf("decodeComboModels() error = %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("round trip = %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i].Ref() != tc.want[i] {
					t.Fatalf("round trip = %v, want %v", got, tc.want)
				}
				if got[i].Priority() != tc.input[i].Priority() {
					t.Fatalf("round trip priority = %d, want %d", got[i].Priority(), tc.input[i].Priority())
				}
			}
		})
	}
	if _, err := decodeComboModels([]byte(`"not an array"`)); err == nil {
		t.Fatal("decodeComboModels() accepted a non-array document")
	}
}

// TestNullString pins the optional judge column: "no judge" is SQL NULL, so it
// never round-trips as an empty string that reads like a set value.
func TestNullString(t *testing.T) {
	if got := nullString(""); got != nil {
		t.Fatalf("nullString(\"\") = %v, want nil", *got)
	}
	got := nullString("openai/gpt-4o")
	if got == nil || *got != "openai/gpt-4o" {
		t.Fatalf("nullString() = %v, want the value", got)
	}
}
