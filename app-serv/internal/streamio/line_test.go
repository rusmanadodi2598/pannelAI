// Package streamio reads delimited lines from an upstream response body.
//
// @file      internal/streamio/line_test.go
// @for       The bound a caller sets on one line, and what happens when an upstream exceeds it.
// @uses      bufio, errors, io, strings, testing
// @reason    The gateway reads SSE lines from providers it does not control. A reader that grows until a delimiter arrives lets the upstream choose the process's memory, so the ceiling has to bite before the bytes are held, not after.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-04
package streamio

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestReadLineKeepsTheDelimiterAndTheBytes(t *testing.T) {
	reader := bufio.NewReaderSize(strings.NewReader("first\nsecond\n"), 8)

	first, err := ReadLine(reader, 64)
	if err != nil || string(first) != "first\n" {
		t.Fatalf("ReadLine() = %q, %v, want \"first\\n\" with no error", first, err)
	}
	second, err := ReadLine(reader, 64)
	if err != nil || string(second) != "second\n" {
		t.Fatalf("ReadLine() = %q, %v, want \"second\\n\" with no error", second, err)
	}
}

func TestReadLineAssemblesALineLargerThanTheBuffer(t *testing.T) {
	body := strings.Repeat("a", 200) + "\n"
	reader := bufio.NewReaderSize(strings.NewReader(body), 16)

	line, err := ReadLine(reader, 4096)
	if err != nil {
		t.Fatalf("ReadLine() error = %v", err)
	}
	if string(line) != body {
		t.Fatalf("line = %d bytes, want the whole 201-byte line", len(line))
	}
}

func TestReadLineRefusesBeyondTheBound(t *testing.T) {
	// No newline at all: the upstream that decides the gateway holds a megabyte.
	reader := bufio.NewReaderSize(strings.NewReader(strings.Repeat("b", 1<<20)), 4096)

	line, err := ReadLine(reader, 1024)
	if !errors.Is(err, ErrTooLong) {
		t.Fatalf("ReadLine() error = %v, want ErrTooLong", err)
	}
	if len(line) > 1024+4096 {
		t.Fatalf("ReadLine() held %d bytes before refusing, want the bound plus at most one buffer", len(line))
	}
}

func TestReadLineReportsAPartialTailAtEOF(t *testing.T) {
	reader := bufio.NewReaderSize(strings.NewReader("tail"), 16)

	line, err := ReadLine(reader, 64)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("ReadLine() error = %v, want io.EOF for a stream that ended mid-line", err)
	}
	if string(line) != "tail" {
		t.Fatalf("line = %q, want the bytes that did arrive", line)
	}
}
