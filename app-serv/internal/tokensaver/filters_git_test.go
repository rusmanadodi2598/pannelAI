// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/filters_git_test.go
// @for       Table-driven tests for the three git filters.
// @uses      strings, testing.
// @reason    TDD.md §2.5 requires the table to carry the boundaries, so each
//
//	filter is driven with a realistic dump, a boundary case (an empty
//	input, a single line, an oversized run), and the never-grow rule.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import (
	"strings"
	"testing"
)

// TestGitDiffFilter drives the diff compactor.
func TestGitDiffFilter(t *testing.T) {
	small := strings.Join([]string{
		"diff --git a/main.go b/main.go",
		"index 111..222 100644",
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -1,4 +1,5 @@",
		" package main",
		"",
		"-func main() {}",
		"+func main() { println(\"hi\") }",
		"+// a comment",
	}, "\n")

	big := &strings.Builder{}
	big.WriteString("diff --git a/big.go b/big.go\n@@ -1,1 +1,200 @@\n")
	for i := 0; i < 200; i++ {
		big.WriteString("+added line with some length to it\n")
	}

	cases := []struct {
		name        string
		input       string
		wantContain []string
	}{
		{
			name:  "a small diff keeps the file, the hunk, and the count",
			input: small,
			wantContain: []string{
				"\nmain.go",
				"  @@ -1,4 +1,5 @@",
				"  +func main() { println(\"hi\") }",
				"  +2 -1",
			},
		},
		{
			name:  "a hunk past the cap is truncated",
			input: big.String(),
			wantContain: []string{
				"(100 lines truncated)",
				"[full diff: rtk git diff --no-compact]",
			},
		},
		{name: "an empty input", input: "", wantContain: []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := gitDiffFilter(tc.input)
			for _, want := range tc.wantContain {
				if !strings.Contains(got, want) {
					t.Fatalf("output must contain %q:\n%s", want, got)
				}
			}
			if tc.input != "" && len(got) >= len(tc.input) {
				t.Fatalf("a filter must never grow its input: %d -> %d", len(tc.input), len(got))
			}
		})
	}
}

// TestGitStatusFilter drives the status compactor across the long form, the
// porcelain form, and the clean case.
func TestGitStatusFilter(t *testing.T) {
	long := strings.Join([]string{
		"On branch main",
		"Changes to be committed:",
		"  new file:   internal/new.go",
		"Changes not staged for commit:",
		"  modified:   internal/old.go",
		"Untracked files:",
		"  scratch.txt",
		"",
	}, "\n")

	porcelain := strings.Join([]string{
		"## main...origin/main",
		" M internal/old.go",
		"?? scratch.txt",
	}, "\n")

	cases := []struct {
		name        string
		input       string
		wantContain []string
	}{
		{
			name:  "the long form",
			input: long,
			wantContain: []string{
				"* main",
				"+ Staged: 1 files",
				"   internal/new.go",
				"~ Modified: 1 files",
			},
		},
		{
			name:        "the porcelain form",
			input:       porcelain,
			wantContain: []string{"* main...origin/main", "~ Modified: 1 files", "? Untracked: 1 files"},
		},
		{name: "a clean tree", input: "On branch main\nnothing to commit, working tree clean\n", wantContain: []string{"clean: nothing to commit"}},
		{name: "an empty input", input: "", wantContain: []string{"Clean working tree"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := gitStatusFilter(tc.input)
			for _, want := range tc.wantContain {
				if !strings.Contains(got, want) {
					t.Fatalf("output must contain %q:\n%s", want, got)
				}
			}
		})
	}
}

// TestGitStatusFilter_CapsTheUntrackedList pins the per-group cap: the count
// stays exact while the list stops at ten.
func TestGitStatusFilter_CapsTheUntrackedList(t *testing.T) {
	lines := []string{"## main"}
	for i := 0; i < 12; i++ {
		lines = append(lines, "?? file"+string(rune('a'+i))+".txt")
	}
	got := gitStatusFilter(strings.Join(lines, "\n"))
	if !strings.Contains(got, "? Untracked: 12 files") {
		t.Fatalf("the count must stay exact:\n%s", got)
	}
	if !strings.Contains(got, "... +2 more") {
		t.Fatalf("the list must report what it dropped:\n%s", got)
	}
}

// TestGitLogFilter drives the log compactor across the long form and the
// oneline form.
func TestGitLogFilter(t *testing.T) {
	long := strings.Join([]string{
		"commit 0123456789abcdef0123456789abcdef01234567",
		"Author: Dodi Rusmana <rusmanadodi@kentangtech.com>",
		"Date:   Sat Sep 19 2026",
		"",
		"    feat(app-serv): add the thing",
		"",
		" internal/a.go | 4 ++--",
		" 1 file changed, 2 insertions(+), 2 deletions(-)",
		"",
		"diff --git a/internal/a.go b/internal/a.go",
		"@@ -1 +1 @@",
		"-old",
		"+new",
	}, "\n")

	oneline := strings.Join([]string{
		"0123456 feat(app-serv): add the thing",
		"fedcba9 fix(app-serv): repair the thing",
	}, "\n")

	cases := []struct {
		name        string
		input       string
		wantContain []string
	}{
		{
			name:  "the long form keeps headers, subjects, and stats",
			input: long,
			wantContain: []string{
				"commit 0123456789abcdef0123456789abcdef01234567",
				"Author: Dodi Rusmana",
				"  Subject: feat(app-serv): add the thing",
				"  1 file changed, 2 insertions(+), 2 deletions(-)",
				"  ... diff body omitted",
			},
		},
		{name: "the oneline form", input: oneline, wantContain: []string{"0123456 feat(app-serv): add the thing", "fedcba9 fix(app-serv): repair the thing"}},
		{name: "an empty input", input: "", wantContain: []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := gitLogFilter(tc.input)
			for _, want := range tc.wantContain {
				if !strings.Contains(got, want) {
					t.Fatalf("output must contain %q:\n%s", want, got)
				}
			}
			if tc.input != "" && len(got) > len(tc.input) {
				t.Fatalf("a filter must never grow its input: %d -> %d", len(tc.input), len(got))
			}
		})
	}
}
