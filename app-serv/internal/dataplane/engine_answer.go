// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_answer.go
// @for       Reading one upstream answer into the outcome, and converting the non-streamed shapes into the client's wire format.
// @uses      internal/schema, context, io, time.
// @reason    SPEC-API-001 §7.15 serves three client wires from three upstream formats, so the non-streamed answer has its own pair of decisions.
//
//	Keeping it out of the stream file holds both inside the AGENTS.md
//	§1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-17
package dataplane

import (
	"context"
	"io"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// translateFolded converts a streamed answer that was folded back into one body into the
// client's wire format and reads its accounting. The fold produced the upstream's own
// non-streamed wire, so the non-streamed translation is reused rather than every mapping
// duplicated: only the body it reads differs, and a matching format is forwarded as
// written apart from the model name a combo re-claims.
func (e *Engine) translateFolded(upstream *Upstream, resolution Resolution, in Request, outcome *Outcome) ([]byte, *schema.Usage, error) {
	raw, usage, err := foldStream(upstream, resolution, foldStopSequences(in))
	if err != nil {
		return nil, nil, err
	}
	if usage == nil {
		usage = readUsage(raw, resolution.Target)
	}
	outcome.Truncated = emptyTruncatedAnswer(raw, resolution)

	switch in.ClientFormat {
	case schema.FormatAnthropic:
		if resolution.Target == TargetClaude {
			return stampAnswerModel(raw, resolution.ClientModel()), usage, nil
		}
		translated, err := claudeAnswer(raw, resolution)
		if err != nil {
			return nil, nil, err
		}
		body, encodeErr := encode(translated)
		return body, usage, encodeErr
	case schema.FormatOpenAIResponses:
		translated, err := responsesClientAnswer(raw, resolution, e.clock().Unix())
		if err != nil {
			return nil, nil, err
		}
		body, encodeErr := encode(translated)
		return body, usage, encodeErr
	default:
		if resolution.Target == TargetOpenAI {
			return stampAnswerModel(raw, resolution.ClientModel()), usage, nil
		}
		translated, err := openAIAnswer(raw, resolution, e.clock().Unix())
		if err != nil {
			return nil, nil, err
		}
		body, encodeErr := encode(translated)
		return body, usage, encodeErr
	}
}

// foldStopSequences answers whether the fold itself should cut the answer to the caller's
// stop sequences. An Anthropic client is cut after translation instead: the fold produces
// the upstream's OpenAI-shaped body, so a marker removed there is gone before the
// Anthropic answer exists and cannot report `stop_reason: "stop_sequence"`.
func foldStopSequences(in Request) []string {
	if in.ClientFormat == schema.FormatAnthropic {
		return nil
	}
	return in.stopSequences()
}

// translateAnswer converts a non-streamed upstream answer into the client's wire
// format and reads its accounting.
func (e *Engine) translateAnswer(upstream *Upstream, resolution Resolution, in Request, outcome *Outcome) ([]byte, *schema.Usage, error) {
	raw, err := io.ReadAll(upstream.Body)
	if err != nil {
		return nil, nil, wrapDataPlaneError(CodeUpstreamError, "the upstream response could not be read", err)
	}
	usage := readUsage(raw, resolution.Target)
	outcome.Truncated = emptyTruncatedAnswer(raw, resolution)

	switch in.ClientFormat {
	case schema.FormatAnthropic:
		if resolution.Target == TargetClaude {
			return stampAnswerModel(raw, resolution.ClientModel()), usage, nil
		}
		translated, err := claudeAnswer(raw, resolution)
		if err != nil {
			return nil, nil, err
		}
		body, encodeErr := encode(translated)
		return body, usage, encodeErr
	case schema.FormatOpenAIResponses:
		translated, err := responsesClientAnswer(raw, resolution, e.clock().Unix())
		if err != nil {
			return nil, nil, err
		}
		body, encodeErr := encode(translated)
		return body, usage, encodeErr
	default:
		if resolution.Target == TargetOpenAI {
			return stampAnswerModel(raw, resolution.ClientModel()), usage, nil
		}
		translated, err := openAIAnswer(raw, resolution, e.clock().Unix())
		if err != nil {
			return nil, nil, err
		}
		body, encodeErr := encode(translated)
		return body, usage, encodeErr
	}
}

// answer reads one upstream answer into the outcome, in whichever of the three shapes the
// call produced: a stream the client asked for, a stream folded back for a client that
// asked for one body, or a plain non-streamed body. Failure bookkeeping happens once here.
func (e *Engine) answer(
	ctx context.Context,
	upstream *Upstream,
	resolution Resolution,
	in Request,
	sink FrameSink,
	selection Selection,
	started time.Time,
	outcome *Outcome,
) error {
	body, usage, err := e.readAnswer(ctx, upstream, resolution, in, sink, outcome)
	if err != nil {
		e.recordFailure(ctx, selection, err)
		outcome.LatencyMS = e.elapsedMS(started)
		return err
	}
	outcome.Body = body
	// A streamed answer reports its usage through the outcome inside relayStream,
	// and hands back no separate body: overwriting here would discard the numbers
	// the upstream sent and record the call as 0/0.
	if usage != nil {
		outcome.Usage = usage
	}
	return nil
}

// readAnswer performs the translation the call's shape calls for.
func (e *Engine) readAnswer(
	ctx context.Context,
	upstream *Upstream,
	resolution Resolution,
	in Request,
	sink FrameSink,
	outcome *Outcome,
) ([]byte, *schema.Usage, error) {
	if in.Stream {
		return nil, nil, e.relayStream(ctx, upstream, resolution, in, sink, outcome)
	}
	body, usage, err := e.readOneBody(upstream, resolution, in, outcome)
	if err != nil {
		return nil, nil, err
	}
	// A stream is cut frame by frame at the translator; a single body is cut here, after
	// translation, because only the served shape knows the text the client will read and
	// reports a cut as `stop_reason: "stop_sequence"`.
	if in.ClientFormat == schema.FormatAnthropic {
		return cutClaudeAnswer(body, in.stopSequences()), usage, nil
	}
	return body, usage, nil
}

// readOneBody folds or translates the upstream answer into the single body a
// non-streamed client receives, whichever of the two shapes it came in.
func (e *Engine) readOneBody(
	upstream *Upstream,
	resolution Resolution,
	in Request,
	outcome *Outcome,
) ([]byte, *schema.Usage, error) {
	if e.transport.ForcesStream(resolution.Provider) {
		// The provider only answers a stream, so the single body the client
		// asked for is folded back out of it. The fold returns the upstream's
		// own non-streamed wire, so the same answer translator serves it.
		return e.translateFolded(upstream, resolution, in, outcome)
	}
	return e.translateAnswer(upstream, resolution, in, outcome)
}
