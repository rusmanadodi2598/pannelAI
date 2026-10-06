// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_sse.go
// @for       Reading one SSE event at a time from a provider stream.
// @uses      bufio, bytes, errors, io, strings.
// @reason    The peek has to hand back every byte it took, or the first frame of the answer is lost before the client reads it. So the reader here returns the raw event beside its payload rather than consuming it silently, and it bounds one event the way the core's own stream reader does, a provider that never sends a newline cannot grow a buffer without limit.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-27
package provider

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/streamio"
)

// qoderMaxEventBytes bounds one event, matching the ceiling the core puts on an
// upstream SSE event so a connector cannot be the reason a stream is unbounded.
const qoderMaxEventBytes = 1 << 20

// nextSSEEvent reads one event and returns its joined `data:` payload beside the raw
// bytes it consumed. A nil payload with no error means the event carried no data at
// all, a comment or a keepalive, and the caller moves on to the next one.
func nextSSEEvent(reader *bufio.Reader) (payload []byte, consumed []byte, err error) {
	var data bytes.Buffer
	var taken bytes.Buffer
	sawData := false

	for {
		// The bound is applied while the line is being read, not after it is
		// accumulated: a ReadBytes of an unbounded line allocates the whole thing
		// first, which is the exact memory an upstream controls.
		line, readErr := streamio.ReadLine(reader, qoderMaxEventBytes-taken.Len())
		if len(line) > 0 {
			taken.Write(line)
			trimmed := strings.TrimRight(string(line), "\r\n")
			if trimmed == "" && sawData {
				return data.Bytes(), taken.Bytes(), nil
			}
			if strings.HasPrefix(trimmed, "data:") {
				if sawData {
					data.WriteByte('\n')
				}
				data.WriteString(strings.TrimPrefix(trimmed, "data:"))
				sawData = true
			}
		}
		if errors.Is(readErr, streamio.ErrTooLong) {
			return nil, taken.Bytes(), errors.New("provider: the upstream stream contained an oversized event")
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				if sawData {
					return data.Bytes(), taken.Bytes(), io.EOF
				}
				return nil, taken.Bytes(), io.EOF
			}
			return nil, taken.Bytes(), readErr
		}
	}
}

// readFirstSSEData peeks the first data-bearing event of a stream and returns its
// payload with every byte the peek took, so a caller can replay them.
func readFirstSSEData(reader *bufio.Reader) (*[]byte, []byte, error) {
	for {
		payload, taken, err := nextSSEEvent(reader)
		if payload != nil {
			found := payload
			return &found, taken, nil
		}
		if err != nil {
			return nil, taken, err
		}
	}
}

// readSSEData returns one event's payload, or io.EOF when the stream has no event
// left. The distinction matters: an event that carried no data is skipped by the
// caller, and only the end of the stream stops it, so swallowing EOF here would turn
// a keepalive-terminated stream into a loop that never ends.
func readSSEData(reader *bufio.Reader) ([]byte, error) {
	payload, _, err := nextSSEEvent(reader)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if payload == nil && errors.Is(err, io.EOF) {
		return nil, io.EOF
	}
	return payload, nil
}
