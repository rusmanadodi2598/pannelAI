// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/filters_search_test.go
// @for       Table-driven tests for grep, find, search-list, ls, and tree.
// @uses      fmt, strings, testing.
// @reason    TDD.md §2.5 requires the boundaries, so each of these filters is
//
//	driven with a realistic dump and with the cap that decides how much
//	of it survives.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import (
	"fmt"
	"strings"
	"testing"
)

// TestGrepFilter drives the match regroup.
func TestGrepFilter(t *testing.T) {
	input := strings.Join([]string{
		"internal/a.go:12:func main() {",
		"internal/a.go:40:func helper() {",
		"internal/b.go:7:import \"fmt\"",
	}, "\n")

	got := grepFilter(input)
	for _, want := range []string{"3 matches in 2F:", "[file] internal/a.go (2):", "  12: func main() {", "[file] internal/b.go (1):"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output must contain %q:\n%s", want, got)
		}
	}

	if same := grepFilter("no matches here\n"); same != "no matches here\n" {
		t.Fatalf("a blob with no match must pass through unchanged, got %q", same)
	}
}

// TestGrepFilter_CapsTheMatchesPerFile pins the per-file cap.
func TestGrepFilter_CapsTheMatchesPerFile(t *testing.T) {
	lines := make([]string, 0, 14)
	for i := 1; i <= 14; i++ {
		lines = append(lines, fmt.Sprintf("internal/a.go:%d:hit", i))
	}
	got := grepFilter(strings.Join(lines, "\n"))
	if !strings.Contains(got, "[file] internal/a.go (14):") {
		t.Fatalf("the count must stay exact:\n%s", got)
	}
	if !strings.Contains(got, "  +4") {
		t.Fatalf("the list must report what it dropped:\n%s", got)
	}
}

// TestFindFilter drives the directory regroup.
func TestFindFilter(t *testing.T) {
	input := strings.Join([]string{
		"internal/a.go",
		"internal/b.go",
		"cmd/main.go",
	}, "\n")

	got := findFilter(input)
	for _, want := range []string{"3 files in 2 dirs:", "cmd/  (1)", "  main.go", "internal/  (2)"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output must contain %q:\n%s", want, got)
		}
	}

	if same := findFilter("\n\n"); same != "\n\n" {
		t.Fatalf("a blob with no paths must pass through unchanged, got %q", same)
	}
}

// TestSearchListFilter drives the Cursor Glob regroup.
func TestSearchListFilter(t *testing.T) {
	input := strings.Join([]string{
		"Result of search in 'internal' (total 3 files):",
		"- internal/a.go",
		"- internal/b.go",
		"- cmd/main.go",
	}, "\n")

	got := searchListFilter(input)
	for _, want := range []string{"Result of search in 'internal' (total 3 files):", "3 files in 2 dirs:", "internal/ (2):", "  a.go"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output must contain %q:\n%s", want, got)
		}
	}

	if same := searchListFilter("no list here\n"); same != "no list here\n" {
		t.Fatalf("a blob with no entries must pass through unchanged, got %q", same)
	}
}

// TestLsFilter drives the listing compactor.
func TestLsFilter(t *testing.T) {
	input := strings.Join([]string{
		"total 24",
		"drwxr-xr-x  4 user user 4096 Sep 19 10:00 internal",
		"drwxr-xr-x  2 user user 4096 Sep 19 10:00 node_modules",
		"-rw-r--r--  1 user user 2048 Sep 19 10:00 main.go",
		"-rw-r--r--  1 user user 1024 Sep 19 10:00 README.md",
	}, "\n")

	got := lsFilter(input)
	for _, want := range []string{"internal/\n", "main.go  2.0K", "README.md  1.0K", "Summary: 2 files, 1 dirs", "1 .go, 1 .md"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output must contain %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "node_modules") {
		t.Fatalf("a noise directory must be dropped:\n%s", got)
	}

	if same := lsFilter("not a listing\n"); same != "not a listing\n" {
		t.Fatalf("a blob that is not a listing must pass through unchanged, got %q", same)
	}
}

// TestTreeFilter drives the tree compactor.
func TestTreeFilter(t *testing.T) {
	input := strings.Join([]string{
		"",
		".",
		"├── internal",
		"│   └── a.go",
		"└── main.go",
		"",
		"5 directories, 23 files",
		"",
	}, "\n")

	got := treeFilter(input)
	if !strings.Contains(got, "├── internal") {
		t.Fatalf("the tree must survive:\n%s", got)
	}
	if strings.Contains(got, "directories, 23 files") {
		t.Fatalf("the counts line must be dropped:\n%s", got)
	}
	if strings.HasPrefix(got, "\n") || strings.HasSuffix(got, "\n") {
		t.Fatalf("the blank edges must be dropped:\n%q", got)
	}
}

// TestTreeFilter_CapsAnOversizedTree pins the line cap.
func TestTreeFilter_CapsAnOversizedTree(t *testing.T) {
	lines := make([]string, 0, 260)
	for i := 0; i < 260; i++ {
		lines = append(lines, fmt.Sprintf("│   └── file%d.go", i))
	}
	got := treeFilter(strings.Join(lines, "\n"))
	if !strings.Contains(got, "... +60 more lines") {
		t.Fatalf("the cap must report what it dropped:\n%s", got)
	}
}
