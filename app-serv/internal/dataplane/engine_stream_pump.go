// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_stream_pump.go
// @for       Reading one upstream SSE body: the bounded event loop, its framing helpers, and the terminal marker it reports.
// @uses      internal/streamio, bufio, bytes, context, errors, io.
// @reason    The pump is the half that talks to the upstream; keeping it apart from the client-format switch in engine_stream.go leaves both files inside the AGENTS.md §1.1 budget. It also reports whether the upstream sent its terminal marker, which is what tells a clean EOF from a truncated answer.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-22
package dataplane

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/streamio"
)

// maxEventBytes bounds one upstream SSE event. The bound is enforced inside the
// line read (internal/streamio), because a length check on what the reader
// returns happens after the oversized line is already held.
const maxEventBytes = 1 << 20

// pump reads the upstream stream event by event and hands each payload to the
// translator, writing whatever frames it produces. The bool reports whether the
// upstream sent its terminal marker, so a clean EOF can be told from a cut.
func (e *Engine) pump(
	ctx context.Context,
	upstream *Upstream,
	sink FrameSink,
	upstreamTarget string,
	translate func(string, []byte) [][]byte,
) (bool, error) {
	reader := bufio.NewReaderSize(upstream.Body, 16<<10)
	var event bytes.Buffer
	var sawDone bool

	for {
		if err := ctx.Err(); err != nil {
			// A client that disconnected stops the upstream read: continuing would
			// bill an answer nobody receives.
			//nolint:nilerr // reason: there is no caller left to report the cancelled context to; a quiet stop is the documented behaviour for a disconnected client.
			return sawDone, nil
		}
		// The bound is inside the read, not applied to its result: an upstream that
		// never sends a newline must not decide how much memory this process holds.
		line, err := streamio.ReadLine(reader, maxEventBytes)
		if errors.Is(err, streamio.ErrTooLong) {
			return sawDone, dataPlaneError(CodeUpstreamError, "the upstream stream contained an oversized event")
		}
		if len(line) > 0 {
			if event.Len()+len(line) > maxEventBytes {
				return sawDone, dataPlaneError(CodeUpstreamError, "the upstream stream contained an oversized event")
			}
			event.Write(line)
		}
		if err != nil {
			// A stream that ends mid-event is normal, so the trailing partial
			// event is processed rather than discarded.
			sawDone = sawDone || sseTerminal(event.Bytes())
			if frames := translateEvent(event.Bytes(), upstreamTarget, translate); len(frames) > 0 {
				if writeErr := writeFrames(sink, frames); writeErr != nil {
					return sawDone, writeErr
				}
			}
			if err == io.EOF {
				return sawDone, nil
			}
			// The idle guard reports a stall as an unexpected EOF, and the
			// request's deadline reports a timeout; both mean the same thing to
			// the client.
			return sawDone, wrapDataPlaneError(CodeUpstreamTimeout, "the upstream stream ended early", err)
		}
		if !bytes.HasSuffix(line, []byte("\n")) {
			continue
		}

		// An SSE event ends at a blank line, which is what the framing in §4
		// relies on: a frame is only complete when the blank line arrives.
		trimmed := bytes.TrimRight(line, "\r\n")
		if len(trimmed) == 0 {
			sawDone = sawDone || sseTerminal(event.Bytes())
			if frames := translateEvent(event.Bytes(), upstreamTarget, translate); len(frames) > 0 {
				if writeErr := writeFrames(sink, frames); writeErr != nil {
					return sawDone, writeErr
				}
			}
			event.Reset()
		}
	}
}

// translateEvent extracts the payload from one complete SSE event and translates
// it. An event with no data line, or the terminal [DONE] marker, produces nothing.
func translateEvent(raw []byte, upstreamTarget string, translate func(string, []byte) [][]byte) [][]byte {
	payload := sseData(raw)
	if len(payload) == 0 {
		return nil
	}
	return translate(upstreamTarget, payload)
}

// sseData returns an event's data payload, joining multiple data lines with the
// newline SSE specifies between them. It returns nil for the terminal marker and
// for an event that carries no data at all.
func sseData(raw []byte) []byte {
	var payload bytes.Buffer
	for _, line := range bytes.Split(raw, []byte("\n")) {
		line = bytes.TrimRight(line, "\r")
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		value := bytes.TrimSpace(line[len("data:"):])
		if len(value) == 0 {
			continue
		}
		if bytes.Equal(value, []byte("[DONE]")) {
			return nil
		}
		if payload.Len() > 0 {
			payload.WriteByte('\n')
		}
		payload.Write(value)
	}
	if payload.Len() == 0 {
		return nil
	}
	return payload.Bytes()
}

// sseTerminal reports whether an event carries the OpenAI terminal marker, which
// sseData deliberately hides from the translators.
func sseTerminal(raw []byte) bool {
	for _, line := range bytes.Split(raw, []byte("\n")) {
		line = bytes.TrimRight(line, "\r")
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		if bytes.Equal(bytes.TrimSpace(line[len("data:"):]), []byte("[DONE]")) {
			return true
		}
	}
	return false
}
