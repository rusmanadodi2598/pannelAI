// Package streamio reads delimited lines from an upstream response body.
//
// @file      internal/streamio/line.go
// @for       One line of a byte stream, bounded by a caller-supplied ceiling.
// @uses      bufio, errors, io
// @reason    bufio.Reader.ReadBytes grows its own buffer until it finds the delimiter, so an upstream that never sends a newline decides how much memory the gateway holds. A length check applied to its return value runs after that decision, which is why the bound has to sit inside the read.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-04
package streamio

import (
	"bufio"
	"errors"
)

// ErrTooLong reports a line past the ceiling the caller set. The bytes read so far
// are returned beside it, because a caller mid-stream may still need what arrived
// before the overrun.
var ErrTooLong = errors.New("streamio: the line exceeds the configured bound")

// ReadLine returns one line, delimiter included, and stops growing at maxBytes.
//
// It reads through the bufio buffer rather than asking it to hold a whole line:
// ReadSlice refuses with ErrBufferFull instead of allocating, and the pieces are
// copied out into a slice the caller owns. The cost is one copy per line, which is
// what a bounded read against an unbounded upstream is worth.
func ReadLine(reader *bufio.Reader, maxBytes int) ([]byte, error) {
	var line []byte
	for {
		chunk, err := reader.ReadSlice('\n')
		line = append(line, chunk...)
		if len(line) > maxBytes {
			return line, ErrTooLong
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if err != nil {
			return line, err
		}
		return line, nil
	}
}
