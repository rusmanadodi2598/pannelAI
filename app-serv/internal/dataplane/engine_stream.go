// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_stream.go
// @for       Reading an upstream SSE body line by line and writing the client's frames to the sink as they arrive.
// @uses      internal/schema, bufio, bytes, context, io, strings.
// @reason    SPEC-API-001 §4 requires SSE passthrough with per-frame flushing and a usage chunk when the client asked for one. Reading incrementally matters: buffering the whole stream would turn a live answer into one delayed blob, which is the property a CLI tool reports as "hangs".
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-17
package dataplane

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/streamio"
)

// maxEventBytes bounds one upstream SSE event. The bound is enforced inside the
// line read (internal/streamio), because a length check on what the reader
// returns happens after the oversized line is already held.
const maxEventBytes = 1 << 20

// relayStream forwards a streamed answer to the sink, re-framing it into the
// client's wire format and recording the accounting the upstream reported.
func (e *Engine) relayStream(
	ctx context.Context,
	upstream *Upstream,
	resolution Resolution,
	in Request,
	sink FrameSink,
	outcome *Outcome,
) error {
	if sink == nil {
		return internalError("a streamed request needs a sink", nil)
	}
	// The counter is what lets the combo walk tell a member that never started
	// from one the client is already reading, so it wraps the sink for the whole
	// leg and reports on every exit, the error exits included.
	counted := &countedSink{FrameSink: sink}
	sink = counted
	defer func() { outcome.FramesWritten = counted.wrote }()

	switch in.ClientFormat {
	case schema.FormatAnthropic:
		state := NewClaudeStreamState("", answerModel(resolution, resolution.UpstreamID)).
			WithStopSequences(in.stopSequences())
		if err := e.pump(ctx, upstream, sink, resolution.Target, state.Frames); err != nil {
			return err
		}
		if err := writeFrames(sink, state.Finish()); err != nil {
			return err
		}
		outcome.Usage = state.Usage()
	case schema.FormatOpenAIResponses:
		// This surface names the answer by the upstream id rather than the
		// resolved one, which §7.6 records as its own convention. A combo is not
		// that convention's case: the caller addressed a name it can re-send, so
		// the combo name overrides it and a plain model keeps what it had.
		state := NewResponsesStreamState("", answerModel(resolution, resolution.UpstreamID), e.clock().Unix())
		if err := e.pump(ctx, upstream, sink, resolution.Target, state.Frames); err != nil {
			return err
		}
		if err := writeFrames(sink, state.Finish()); err != nil {
			return err
		}
		outcome.Usage = state.Usage()
	default:
		state := NewStreamState("", answerModel(resolution, resolution.UpstreamID), e.clock().Unix(), in.IncludeUsage).
			WithStopSequences(in.stopSequences())
		if err := e.pump(ctx, upstream, sink, resolution.Target, state.Frames); err != nil {
			return err
		}
		if err := writeFrames(sink, state.Finish()); err != nil {
			return err
		}
		outcome.Usage = state.Usage()
	}
	return nil
}

// pump reads the upstream stream event by event and hands each payload to the
// translator, writing whatever frames it produces.
func (e *Engine) pump(
	ctx context.Context,
	upstream *Upstream,
	sink FrameSink,
	upstreamTarget string,
	translate func(string, []byte) [][]byte,
) error {
	reader := bufio.NewReaderSize(upstream.Body, 16<<10)
	var event bytes.Buffer

	for {
		if err := ctx.Err(); err != nil {
			// A client that disconnected stops the upstream read: continuing would
			// bill an answer nobody receives.
			//nolint:nilerr // reason: there is no caller left to report the cancelled context to; a quiet stop is the documented behaviour for a disconnected client.
			return nil
		}
		// The bound is inside the read, not applied to its result: an upstream that
		// never sends a newline must not decide how much memory this process holds.
		line, err := streamio.ReadLine(reader, maxEventBytes)
		if errors.Is(err, streamio.ErrTooLong) {
			return dataPlaneError(CodeUpstreamError, "the upstream stream contained an oversized event")
		}
		if len(line) > 0 {
			if event.Len()+len(line) > maxEventBytes {
				return dataPlaneError(CodeUpstreamError, "the upstream stream contained an oversized event")
			}
			event.Write(line)
		}
		if err != nil {
			// A stream that ends mid-event is normal, so the trailing partial
			// event is processed rather than discarded.
			if frames := translateEvent(event.Bytes(), upstreamTarget, translate); len(frames) > 0 {
				if writeErr := writeFrames(sink, frames); writeErr != nil {
					return writeErr
				}
			}
			if err == io.EOF {
				return nil
			}
			// The idle guard reports a stall as an unexpected EOF, and the
			// request's deadline reports a timeout; both mean the same thing to
			// the client.
			return wrapDataPlaneError(CodeUpstreamTimeout, "the upstream stream ended early", err)
		}
		if !bytes.HasSuffix(line, []byte("\n")) {
			continue
		}

		// An SSE event ends at a blank line, which is what the framing in §4
		// relies on: a frame is only complete when the blank line arrives.
		trimmed := bytes.TrimRight(line, "\r\n")
		if len(trimmed) == 0 {
			if frames := translateEvent(event.Bytes(), upstreamTarget, translate); len(frames) > 0 {
				if writeErr := writeFrames(sink, frames); writeErr != nil {
					return writeErr
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

// countedSink remembers whether any frame reached the client. A stream that has
// spoken is committed to the member that spoke: starting another one through the
// same sink would append a second answer to what the client is already reading,
// and pay a second upstream for one request.
type countedSink struct {
	FrameSink
	wrote bool
}

func (c *countedSink) WriteFrame(frame []byte) error {
	if err := c.FrameSink.WriteFrame(frame); err != nil {
		return err
	}
	c.wrote = true
	return nil
}

// writeFrames writes each frame and flushes, so a client sees output as it is
// produced rather than at the end of the answer.
func writeFrames(sink FrameSink, frames [][]byte) error {
	for _, frame := range frames {
		if len(frame) == 0 {
			continue
		}
		if err := sink.WriteFrame(frame); err != nil {
			return wrapDataPlaneError(CodeUpstreamError, "the response stream could not be written", err)
		}
	}
	sink.Flush()
	return nil
}
