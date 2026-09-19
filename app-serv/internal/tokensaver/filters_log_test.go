// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/filters_log_test.go
// @for       Table-driven tests for dedup-log, smart-truncate, read-numbered,
//
//	and build-output.
//
// @uses      fmt, strings, testing.
// @reason    TDD.md §2.5 requires the boundaries, and these four filters are the
//
//	ones whose caps decide the output, so each test drives both a small
//	input (pass-through) and one past its cap.
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

// TestDedupLogFilter drives the duplicate collapse.
func TestDedupLogFilter(t *testing.T) {
	input := strings.Join([]string{
		"starting",
		"retrying",
		"retrying",
		"retrying",
		"",
		"",
		"done",
	}, "\n")

	got := dedupLogFilter(input)
	if !strings.Contains(got, "... (2 duplicate lines)") {
		t.Fatalf("the run must be reported:\n%s", got)
	}
	if strings.Count(got, "\n\n") > 1 {
		t.Fatalf("a blank run must collapse:\n%q", got)
	}
	if !strings.Contains(got, "done") {
		t.Fatalf("the tail must survive:\n%s", got)
	}
}

// TestDedupLogFilter_TruncatesAtTheCap pins the line cap.
func TestDedupLogFilter_TruncatesAtTheCap(t *testing.T) {
	lines := make([]string, 0, 2100)
	for i := 0; i < 2100; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	got := dedupLogFilter(strings.Join(lines, "\n"))
	if !strings.Contains(got, "... (truncated at 2000 lines)") {
		t.Fatalf("the cap must be reported:\n%s", got[len(got)-120:])
	}
}

// TestSmartTruncateFilter drives the head/tail truncation and its floor.
func TestSmartTruncateFilter(t *testing.T) {
	small := strings.Join([]string{"a", "b", "c"}, "\n")
	if got := smartTruncateFilter(small); got != small {
		t.Fatalf("a blob under the floor must pass through, got %q", got)
	}

	lines := make([]string, 0, 300)
	for i := 0; i < 300; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	got := smartTruncateFilter(strings.Join(lines, "\n"))
	if !strings.Contains(got, "... +120 lines truncated") {
		t.Fatalf("the dropped middle must be counted:\n%s", got)
	}
	if !strings.HasPrefix(got, "line 0\n") || !strings.HasSuffix(got, "line 299") {
		t.Fatalf("the head and the tail must survive:\n%s", got)
	}
}

// TestReadNumberedFilter drives the line-numbered truncation.
func TestReadNumberedFilter(t *testing.T) {
	lines := make([]string, 0, 300)
	for i := 1; i <= 300; i++ {
		lines = append(lines, fmt.Sprintf("%3d|package main", i))
	}
	got := readNumberedFilter(strings.Join(lines, "\n"))
	if !strings.Contains(got, "... +120 lines truncated (file continues)") {
		t.Fatalf("the marker must name the shape:\n%s", got)
	}
	if !strings.Contains(got, "  1|package main") {
		t.Fatalf("the head must survive:\n%s", got)
	}
}

// TestBuildOutputFilter drives the build-log compactor.
func TestBuildOutputFilter(t *testing.T) {
	input := strings.Join([]string{
		"npm warn deprecated left-pad@1.0.0: use pad-start",
		"npm warn deprecated request@2.0.0: deprecated",
		"npm warn deprecated mkdirp@1.0.0: deprecated",
		"npm warn deprecated fsevents@2.0.0: deprecated",
		"npm warn deprecated node-gyp@9.0.0: deprecated",
		"Compiling serde v1.0.0",
		"Compiling tokio v1.0.0",
		"npm ERR! code ELIFECYCLE",
		"npm ERR! errno 1",
		"npm warn some other warning",
		"added 2 packages, and audited 3 packages in 4s",
	}, "\n")

	got := buildOutputFilter(input)
	for _, want := range []string{
		"npm warn deprecated left-pad@1.0.0",
		"... +2 more deprecated packages",
		"Compiled 2 packages",
		"npm ERR! code ELIFECYCLE",
		"npm ERR! errno 1",
		"added 2 packages, and audited 3 packages in 4s",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output must contain %q:\n%s", want, got)
		}
	}
	if len(got) >= len(input) {
		t.Fatalf("the filter must not grow its input: %d -> %d", len(input), len(got))
	}
}

// TestBuildOutputFilter_KeepsACargoErrorBlock pins the continuation rule: the
// lines under a cargo error heading are kept verbatim.
func TestBuildOutputFilter_KeepsACargoErrorBlock(t *testing.T) {
	input := strings.Join([]string{
		"   Compiling pannelai v0.1.0",
		"error[E0308]: mismatched types",
		" --> internal/a.go:12:9",
		"  |",
		"12 |     let x: u8 = \"s\";",
		"  |         ^^ expected u8",
		"",
		"error: could not compile pannelai",
	}, "\n")

	got := buildOutputFilter(input)
	for _, want := range []string{"error[E0308]: mismatched types", "--> internal/a.go:12:9", "expected u8", "could not compile"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output must contain %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Compiling pannelai") {
		t.Fatalf("the progress line must be dropped:\n%s", got)
	}
}
