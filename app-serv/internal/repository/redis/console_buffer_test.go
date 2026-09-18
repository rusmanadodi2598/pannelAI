//go:build integration

// Package redisrepo implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/console_buffer_test.go
// @for       Integration tests for the bounded console ring's eviction.
// @uses      github.com/redis/go-redis/v9, context, os, testing.
// @reason    The ring's whole contract is its bound: SPEC-API-001 §7.13 calls it
//
//	"the last N lines" and §9 forbids unbounded growth. An in-memory
//	double would prove the trim happens but not that Redis applies it
//	to the list, and the trim is the one operation whose failure is
//	silent — the buffer simply grows. These tests run against a real
//	server reached through PANNELAI_TEST_REDIS_ADDR.
//
//	The file carries an `integration` build tag, so the default
//	`go test ./...` stays hermetic on a machine with no Redis (AGENTS.md §2.1
//	forbids t.Skip as a way to sidestep a test, and a tagged file is not
//	compiled rather than skipped at runtime). With the tag active the
//	address is required, not optional: a missing value fails the test
//	rather than passing silently.
//
//	  PANNELAI_TEST_REDIS_ADDR='[user:password@]host:port' \
//	    go test -race -tags=integration ./internal/repository/redis/
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-18
package redisrepo

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"
)

// testRedisEnv names the variable that enables these tests. It carries a Redis
// address, optionally with credentials as `user:password@host:port`, so a server
// that requires AUTH is reachable without a second variable that a caller could
// forget to set.
const testRedisEnv = "PANNELAI_TEST_REDIS_ADDR"

// newTestBuffer connects to the configured server and returns an empty ring.
func newTestBuffer(t *testing.T) *ConsoleBuffer {
	t.Helper()

	raw := os.Getenv(testRedisEnv)
	if raw == "" {
		t.Fatalf("%s must be set to run the console ring tests", testRedisEnv)
	}
	client := redis.NewClient(redisOptions(t, raw))
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("closing redis client: %v", err)
		}
	})
	buffer := NewConsoleBuffer(client)
	if err := buffer.Clear(context.Background()); err != nil {
		t.Fatalf("clearing the ring: %v", err)
	}
	return buffer
}

// redisOptions parses the test address. An address without credentials is the
// common case; `user:password@host:port` is accepted so a password-protected
// server needs no extra variable.
func redisOptions(t *testing.T, raw string) *redis.Options {
	t.Helper()
	options := &redis.Options{Addr: raw}
	at := strings.LastIndex(raw, "@")
	if at < 0 {
		return options
	}
	options.Addr = raw[at+1:]
	credentials := raw[:at]
	user, password, hasUser := strings.Cut(credentials, ":")
	if hasUser {
		options.Username = user
		options.Password = password
		return options
	}
	options.Password = credentials
	return options
}

// TestConsoleBuffer_EvictionAtTheBound walks the ring across its boundary: below
// it, exactly at it, and over it, asserting the contents rather than only the
// length so a trim that keeps the wrong end is caught.
func TestConsoleBuffer_EvictionAtTheBound(t *testing.T) {
	cases := []struct {
		name      string
		bound     int
		appended  []string
		wantLines []string
		wantErr   bool
	}{
		{
			name:  "a single line under a bound of two",
			bound: 2, appended: []string{"one"},
			wantLines: []string{"one"},
		},
		{
			name:  "exactly at the bound keeps every line",
			bound: 3, appended: []string{"one", "two", "three"},
			wantLines: []string{"one", "two", "three"},
		},
		{
			name:  "one over the bound drops the oldest",
			bound: 3, appended: []string{"one", "two", "three", "four"},
			wantLines: []string{"two", "three", "four"},
		},
		{
			name:  "far over the bound keeps only the newest",
			bound: 2, appended: []string{"a", "b", "c", "d", "e", "f"},
			wantLines: []string{"e", "f"},
		},
		{
			name:  "a bound of one keeps only the last line",
			bound: 1, appended: []string{"first", "second", "third"},
			wantLines: []string{"third"},
		},
		{
			name:  "a zero bound stores nothing rather than everything",
			bound: 0, appended: []string{"one", "two"},
			wantLines: nil,
		},
		{
			name:  "a negative bound stores nothing",
			bound: -5, appended: []string{"one"},
			wantLines: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buffer := newTestBuffer(t)
			ctx := context.Background()
			for _, line := range tc.appended {
				if err := buffer.Append(ctx, line, tc.bound); err != nil {
					t.Fatalf("Append(%q) error = %v", line, err)
				}
			}

			lines, err := buffer.Lines(ctx, tc.bound)
			if tc.wantErr {
				if err == nil {
					t.Fatal("Lines = nil error, want one")
				}
				return
			}
			if err != nil {
				t.Fatalf("Lines error = %v", err)
			}
			assertLines(t, lines, tc.wantLines)

			size, err := buffer.Size(ctx)
			if err != nil {
				t.Fatalf("Size error = %v", err)
			}
			if int(size) != len(tc.wantLines) {
				t.Fatalf("Size = %d, want %d: the bound must hold in Redis, not only in the returned slice", size, len(tc.wantLines))
			}
		})
	}
}

// TestConsoleBuffer_ClearEmptiesEveryPanel asserts a clear is server-side: a
// second reader sees the same empty buffer, which is what SPEC-UI §6.16
// promises when the confirmation says the buffer is removed.
func TestConsoleBuffer_ClearEmptiesEveryPanel(t *testing.T) {
	buffer := newTestBuffer(t)
	ctx := context.Background()
	for _, line := range []string{"alpha", "beta"} {
		if err := buffer.Append(ctx, line, 10); err != nil {
			t.Fatalf("Append error = %v", err)
		}
	}
	before, err := buffer.Size(ctx)
	if err != nil {
		t.Fatalf("Size error = %v", err)
	}
	if before != 2 {
		t.Fatalf("Size before clear = %d, want 2", before)
	}

	if err := buffer.Clear(ctx); err != nil {
		t.Fatalf("Clear error = %v", err)
	}

	lines, err := buffer.Lines(ctx, 10)
	if err != nil {
		t.Fatalf("Lines error = %v", err)
	}
	if len(lines) != 0 {
		t.Fatalf("Lines after clear = %v, want none", lines)
	}
	size, err := buffer.Size(ctx)
	if err != nil {
		t.Fatalf("Size error = %v", err)
	}
	if size != 0 {
		t.Fatalf("Size after clear = %d, want 0", size)
	}
}

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
