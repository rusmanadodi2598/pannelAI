// Package redis is the Redis access layer of app-serv.
//
// @file      internal/repository/redis/console_buffer_reads_test.go
// @for       The read bound and the empty-line rule of the console buffer.
// @uses      context, strings, testing, github.com/redis/go-redis/v9.
// @reason    A reader asks for at most N lines and a blank write is not a line of output, so both are read-side limits the eviction test does not reach.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-04

//go:build integration

package redisrepo

import (
	"context"
	"fmt"
	"testing"
)

// TestConsoleBuffer_EmptyLineIsIgnored covers the degenerate append: an empty
// line is not a console line and must not consume a slot in the ring.
func TestConsoleBuffer_EmptyLineIsIgnored(t *testing.T) {
	buffer := newTestBuffer(t)
	ctx := context.Background()
	if err := buffer.Append(ctx, "", 5); err != nil {
		t.Fatalf("Append(\"\") error = %v", err)
	}
	size, err := buffer.Size(ctx)
	if err != nil {
		t.Fatalf("Size error = %v", err)
	}
	if size != 0 {
		t.Fatalf("Size = %d, want 0 after appending an empty line", size)
	}
}

// TestConsoleBuffer_ReadBoundIsRespected asserts a read asks for at most the
// bound, so a caller with a stale bound cannot read more than the ring holds.
func TestConsoleBuffer_ReadBoundIsRespected(t *testing.T) {
	buffer := newTestBuffer(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if err := buffer.Append(ctx, fmt.Sprintf("line-%d", i), 10); err != nil {
			t.Fatalf("Append error = %v", err)
		}
	}
	lines, err := buffer.Lines(ctx, 3)
	if err != nil {
		t.Fatalf("Lines error = %v", err)
	}
	if len(lines) != 3 {
		t.Fatalf("Lines = %v, want the 3 newest", lines)
	}
	// The newest three are the tail, oldest first.
	assertLines(t, lines, []string{"line-2", "line-3", "line-4"})
}

// assertLines compares a line slice, treating nil and empty as equal because
// both mean "no lines".
func assertLines(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("lines = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d = %q, want %q (got %v, want %v)", i, got[i], want[i], got, want)
		}
	}
}
