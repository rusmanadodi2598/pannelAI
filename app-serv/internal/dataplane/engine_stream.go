// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_stream.go
// @for       The client-format switch for a streamed answer: translate the upstream, pump it to the sink, and synthesize the finish the client needs.
// @uses      internal/schema, context.
// @reason    SPEC-API-001 §4 requires SSE passthrough with per-frame flushing and a usage chunk when the client asked for one. The reading loop lives in engine_stream_pump.go for the §1.1 budget; this file owns the per-format state and the commitment a written frame makes.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-17
package dataplane

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

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
	//
	// The abort flag is read the same way on every exit: a cancelled caller means
	// the answer stopped arriving, and an answer that stopped arriving is not
	// evidence the credential works.
	counted := &countedSink{FrameSink: sink}
	sink = counted
	defer func() {
		outcome.FramesWritten = counted.wrote
		outcome.StreamAborted = ctx.Err() != nil
	}()

	switch in.ClientFormat {
	case schema.FormatAnthropic:
		state := NewClaudeStreamState("", answerModel(resolution, resolution.UpstreamID)).
			WithStopSequences(in.stopSequences())
		translate := restoreToolNamesStream(upstream.ToolAliases, state.Frames)
		sawDone, err := e.pump(ctx, upstream, sink, resolution.Target, translate)
		if err != nil {
			return err
		}
		outcome.StreamTruncated = !sawDone && !state.terminalSeen() && ctx.Err() == nil
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
		translate := restoreToolNamesStream(upstream.ToolAliases, state.Frames)
		sawDone, err := e.pump(ctx, upstream, sink, resolution.Target, translate)
		if err != nil {
			return err
		}
		outcome.StreamTruncated = !sawDone && !state.terminalSeen() && ctx.Err() == nil
		if err := writeFrames(sink, state.Finish()); err != nil {
			return err
		}
		outcome.Usage = state.Usage()
	default:
		state := NewStreamState("", answerModel(resolution, resolution.UpstreamID), e.clock().Unix(), in.IncludeUsage).
			WithStopSequences(in.stopSequences())
		translate := restoreToolNamesStream(upstream.ToolAliases, state.Frames)
		sawDone, err := e.pump(ctx, upstream, sink, resolution.Target, translate)
		if err != nil {
			return err
		}
		outcome.StreamTruncated = !sawDone && !state.terminalSeen() && ctx.Err() == nil
		if err := writeFrames(sink, state.Finish()); err != nil {
			return err
		}
		outcome.Usage = state.Usage()
	}
	return nil
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
