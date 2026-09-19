// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/autodetect_test.go
// @for       Table-driven tests for the detection order and the allowlist.
// @uses      internal/domain, reflect, strings, testing.
// @reason    SPEC-API-002 §5 makes the detection order observable, so the table
//
//	pins the two claims that are order-dependent: build output before
//	the porcelain check, and a log before a diff. The registry test is
//	what keeps the twelve names and their implementations in step.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import (
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestRegistry_MatchesTheDomainVocabulary pins the two lists against each other:
// a name the configuration accepts must have an implementation, and an
// implementation must be reachable by a configuration value.
func TestRegistry_MatchesTheDomainVocabulary(t *testing.T) {
	if len(registry) != len(domain.TokenSaverFilters) {
		t.Fatalf("registry carries %d filters, want the %d canonical names", len(registry), len(domain.TokenSaverFilters))
	}
	for _, name := range domain.TokenSaverFilters {
		if _, ok := registry[name]; !ok {
			t.Fatalf("registry has no implementation for %q", name)
		}
	}
	for name := range registry {
		if !domain.ValidTokenSaverFilter(name) {
			t.Fatalf("registry implements %q, which no configuration may name", name)
		}
	}
}

// TestDetect drives the detector over each shape and over the order-sensitive
// pairs.
func TestDetect(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "a diff",
			input: "diff --git a/a.go b/a.go\n@@ -1 +1 @@\n-old\n+new\n",
			want:  "git-diff",
		},
		{
			name:  "a bare hunk",
			input: "@@ -1,3 +1,4 @@\n context\n",
			want:  "git-diff",
		},
		{
			name:  "a status dump",
			input: "On branch main\nChanges not staged for commit:\n  modified:   a.go\n",
			want:  "git-status",
		},
		{
			name:  "a porcelain dump",
			input: "## main\n M a.go\n?? b.go\n M c.go\n",
			want:  "git-status",
		},
		{
			name:  "a log with a graph",
			input: "* commit 0123456789abcdef0123456789abcdef01234567\n* commit fedcba9876543210fedcba9876543210fedcba98\n",
			want:  "git-log",
		},
		{
			name:  "build output wins over the porcelain check",
			input: "## main\n M a.go\n M b.go\n M c.go\n   Compiling serde v1.0.0\n",
			want:  "build-output",
		},
		{
			name:  "a log wins over a diff it embeds",
			input: "commit 0123456789abcdef0123456789abcdef01234567\n\n    subject\n\ndiff --git a/a.go b/a.go\n",
			want:  "git-log",
		},
		{
			name:  "a grep list",
			input: "a.go:12:hit\nb.go:7:hit\nc.go:3:hit\n",
			want:  "grep",
		},
		{
			name:  "a path list",
			input: "internal/a.go\ninternal/b.go\ncmd/main.go\n",
			want:  "find",
		},
		{
			name:  "a tree",
			input: ".\n├── internal\n│   └── a.go\n",
			want:  "tree",
		},
		{
			name:  "an ls listing",
			input: "total 8\ndrwxr-xr-x 2 u u 4096 Sep 19 10:00 internal\n-rw-r--r-- 1 u u 12 Sep 19 10:00 a.go\n",
			want:  "ls",
		},
		{
			name:  "a search list",
			input: "Result of search in 'internal' (total 2 files):\n- internal/a.go\n",
			want:  "search-list",
		},
		{
			name:  "a line-numbered dump",
			input: strings.Repeat("  1|package main\n", 300),
			want:  "read-numbered",
		},
		{
			name:  "generic multi-line noise",
			input: "one\ntwo\nthree\nfour\nfive\n",
			want:  "dedup-log",
		},
		{
			name:  "a blob nothing claims",
			input: "just one line with no shape at all",
			want:  "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, got, ok := detect(tc.input, allowedSet(nil))
			if !ok {
				got = ""
			}
			if got != tc.want {
				t.Fatalf("detect() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestDetect_RespectsTheAllowlist pins the configured filter list's meaning: a
// filter outside it is skipped even when it would claim the blob.
func TestDetect_RespectsTheAllowlist(t *testing.T) {
	diff := "diff --git a/a.go b/a.go\n@@ -1 +1 @@\n-old\n+new\n"

	if _, name, ok := detect(diff, allowedSet([]string{"git-diff"})); !ok || name != "git-diff" {
		t.Fatalf("an allowed filter must claim the blob, got %q ok=%v", name, ok)
	}
	if _, name, ok := detect(diff, allowedSet([]string{"ls", "tree"})); ok {
		t.Fatalf("a filter outside the allowlist must not claim the blob, got %q", name)
	}
	if _, name, ok := detect(diff, allowedSet([]string{"git-diff", "git-status"})); !ok || name != "git-diff" {
		t.Fatalf("the first matching allowed filter must win, got %q ok=%v", name, ok)
	}
}

// TestSafeApply_RecoversFromAPanic pins the fail-open rule: a filter that panics
// on a malformed blob returns the blob, not an error and not an empty string.
func TestSafeApply_RecoversFromAPanic(t *testing.T) {
	panicking := filter(func(string) string { panic("filter bug") })
	got := safeApply(panicking, "original text")
	if got != "original text" {
		t.Fatalf("safeApply() = %q, want the original text", got)
	}
	if !allowedSet(nil)("anything") {
		t.Fatal("an empty allowlist must admit every filter")
	}
}
