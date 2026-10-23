// Package migrate applies numbered SQL migrations.
//
// @file      internal/migrate/parse_test.go
// @for       Filename parsing and pairing contract for the migration set.
// @uses      testing, testing/fstest.
// @reason    A migration set that silently pairs the wrong down file is worse than no migration, so parsing is pinned before the runner exists.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-21
package migrate_test

import (
	"testing"
	"testing/fstest"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/migrate"
)

func TestParse_GivenMigrationSets_ThenVersionsAndPairsAreResolved(t *testing.T) {
	cases := []struct {
		name    string
		files   fstest.MapFS
		want    []int
		wantErr string
	}{
		{
			name: "ordered pair set is parsed",
			files: fstest.MapFS{
				"000002_seed.up.sql":   {Data: []byte("up 2")},
				"000002_seed.down.sql": {Data: []byte("down 2")},
				"000001_init.up.sql":   {Data: []byte("up 1")},
				"000001_init.down.sql": {Data: []byte("down 1")},
			},
			want: []int{1, 2},
		},
		{
			name: "missing down file is refused",
			files: fstest.MapFS{
				"000001_init.up.sql": {Data: []byte("up 1")},
			},
			wantErr: "missing down",
		},
		{
			name: "duplicate version is refused",
			files: fstest.MapFS{
				"000001_init.up.sql":    {Data: []byte("up 1")},
				"000001_init.down.sql":  {Data: []byte("down 1")},
				"000001_again.up.sql":   {Data: []byte("up 1b")},
				"000001_again.down.sql": {Data: []byte("down 1b")},
			},
			wantErr: "duplicate version",
		},
		{
			name: "unversioned file is refused",
			files: fstest.MapFS{
				"init.up.sql": {Data: []byte("up")},
			},
			wantErr: "does not match",
		},
		{
			name:    "empty directory is refused",
			files:   fstest.MapFS{},
			wantErr: "no migrations",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := migrate.Parse(tc.files, ".")
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("Parse() error = nil, want %q", tc.wantErr)
				}
				if !stringsContains(err.Error(), tc.wantErr) {
					t.Fatalf("Parse() error = %q, want %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tc.want))
			}
			for i, version := range tc.want {
				if got[i].Version != version {
					t.Fatalf("version[%d] = %d, want %d", i, got[i].Version, version)
				}
			}
		})
	}
}

func stringsContains(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
