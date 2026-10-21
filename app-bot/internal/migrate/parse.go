// Package migrate applies numbered SQL migrations.
//
// @file      internal/migrate/parse.go
// @for       Reading and pairing the embedded .up.sql and .down.sql files.
// @uses      fmt, io/fs, path, regexp, sort, strconv.
// @reason    The runner must refuse an incomplete pair or a duplicate version before it opens a transaction, so parsing is a separate, tested step.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-21
package migrate

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
)

// Migration is one numbered pair. Both halves are read at parse time so a
// missing rollback fails before anything is applied.
type Migration struct {
	Version int
	Name    string
	UpSQL   []byte
	DownSQL []byte
}

var namePattern = regexp.MustCompile(`^(\d{6})_([a-z0-9_]+)\.(up|down)\.sql$`)

// Parse reads dir from fsys and returns the migrations in ascending version
// order. Statuses: empty set, missing pair half, duplicate version, bad name.
func Parse(fsys fs.FS, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("reading migration directory: %w", err)
	}

	type pair struct {
		name string
		up   []byte
		down []byte
	}
	pairs := make(map[int]*pair)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		match := namePattern.FindStringSubmatch(entry.Name())
		if match == nil {
			return nil, fmt.Errorf("migration file %q does not match NNNNNN_name.up.sql", entry.Name())
		}
		version, err := strconv.Atoi(match[1])
		if err != nil {
			return nil, fmt.Errorf("migration file %q has an unreadable version", entry.Name())
		}
		content, err := fs.ReadFile(fsys, path.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading migration %q: %w", entry.Name(), err)
		}

		current, exists := pairs[version]
		if !exists {
			current = &pair{name: match[2]}
			pairs[version] = current
		}
		if current.name != match[2] {
			return nil, fmt.Errorf("duplicate version %06d: %s and %s", version, current.name, match[2])
		}
		if match[3] == "up" {
			if current.up != nil {
				return nil, fmt.Errorf("duplicate version %06d: two up files for %s", version, current.name)
			}
			current.up = content
			continue
		}
		if current.down != nil {
			return nil, fmt.Errorf("duplicate version %06d: two down files for %s", version, current.name)
		}
		current.down = content
	}

	if len(pairs) == 0 {
		return nil, fmt.Errorf("no migrations found in %s", dir)
	}

	versions := make([]int, 0, len(pairs))
	for version := range pairs {
		versions = append(versions, version)
	}
	sort.Ints(versions)

	migrations := make([]Migration, 0, len(versions))
	for _, version := range versions {
		current := pairs[version]
		if current.up == nil {
			return nil, fmt.Errorf("missing up migration for version %06d", version)
		}
		if current.down == nil {
			return nil, fmt.Errorf("missing down migration for version %06d", version)
		}
		migrations = append(migrations, Migration{
			Version: version,
			Name:    current.name,
			UpSQL:   current.up,
			DownSQL: current.down,
		})
	}
	return migrations, nil
}
