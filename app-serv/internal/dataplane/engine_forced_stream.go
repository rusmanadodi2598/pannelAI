// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_forced_stream.go
// @for       Serving a non-streaming client from a provider that only answers a
//
//	stream.
//
// @uses      internal/schema, bufio, bytes, io.
// @reason    A provider may refuse a non-streaming request (OpenCode Free answers
//
//	403 to one), so a client that asked for a single JSON body has to be
//	served from the stream the provider does send. The fold belongs here
//	rather than in the connector: a connector owns the outbound shape, and
//	turning an answer back into the client's wire is what the translation
//	layer already does. The result is the upstream's own non-streamed
//	wire, so every existing answer translator is reused unchanged.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-21
package dataplane

import (
	"bufio"
	"bytes"
	"errors"
	"io"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/streamio"
)

// maxFoldEvents bounds how many SSE events one folded answer may contain. A
// single event is bounded by maxEventBytes, and every event of an endless stream
// is accumulated into one slice, so the count needs its own ceiling.
const maxFoldEvents = 1 << 16

// foldStream reads a forced stream to its end and returns the single upstream
// answer it carried, in the upstream's own wire format, so the caller's existing
// non-streamed translation applies unchanged. The resolved model is passed to the
// chat fold because a chat stream reports no answer object to name, and an
// upstream's own label (Qoder answers every model as `auto`) is not one the caller
// can send back. A Responses stream folds from its terminal event, a chat stream by
// accumulating deltas, because the two wires carry the answer differently.
func foldStream(upstream *Upstream, resolution Resolution, stop []string) ([]byte, *schema.Usage, error) {
	events, err := readFoldEvents(upstream.Body)
	if err != nil {
		return nil, nil, err
	}
	if resolution.Target == TargetResponses {
		return foldResponsesEvents(events)
	}
	return foldChatEvents(events, answerModel(resolution, resolution.ModelID), stop)
}

// readFoldEvents reads every data payload of an SSE body, in order. The terminal
// [DONE] marker and events with no data are skipped, exactly as the streaming
// reader skips them.
func readFoldEvents(body io.Reader) ([][]byte, error) {
	reader := bufio.NewReaderSize(body, 16<<10)
	var event bytes.Buffer
	events := make([][]byte, 0, 16)

	for {
		// Bounded inside the read: an upstream that never sends a newline must not
		// be the one deciding how much memory the fold holds.
		line, err := streamio.ReadLine(reader, maxEventBytes)
		if errors.Is(err, streamio.ErrTooLong) {
			return nil, dataPlaneError(CodeUpstreamError, "the upstream stream contained an oversized event")
		}
		if len(line) > 0 {
			if event.Len()+len(line) > maxEventBytes {
				return nil, dataPlaneError(CodeUpstreamError, "the upstream stream contained an oversized event")
			}
			event.Write(line)
		}
		if err != nil {
			// A stream that ends mid-event is normal, so the trailing partial
			// event is processed rather than discarded.
			if payload := sseData(event.Bytes()); len(payload) > 0 {
				events = append(events, payload)
			}
			if err == io.EOF {
				return events, nil
			}
			return nil, wrapDataPlaneError(CodeUpstreamTimeout, "the upstream stream ended early", err)
		}
		if len(events) > maxFoldEvents {
			return nil, dataPlaneError(CodeUpstreamError, "the upstream stream contained too many events")
		}

		// An SSE event ends at a blank line, which is what the framing relies
		// on: a frame is only complete when the blank line arrives.
		if len(bytes.TrimRight(line, "\r\n")) == 0 {
			if payload := sseData(event.Bytes()); len(payload) > 0 {
				events = append(events, payload)
			}
			event.Reset()
		}
	}
}

// foldResponsesEvents returns the complete response the terminal event carries. The
// Responses API states the whole answer in `response.completed`, and in
// `response.incomplete` when the model hit its ceiling, so that event's `response`
// member is the non-streamed body verbatim. Reading it rather than re-assembling the
// deltas is what keeps a reasoning item, a tool call, and the accounting from being
// reconstructed by hand.
func foldResponsesEvents(events [][]byte) ([]byte, *schema.Usage, error) {
	for index := len(events) - 1; index >= 0; index-- {
		event, ok := decodeObject(events[index])
		if !ok {
			continue
		}
		switch stringField(event, "type") {
		case EventResponseCompleted, EventResponseDone, EventResponseIncomplete, EventResponseFailed:
			if response, ok := objectField(event, "response"); ok {
				return mustJSON(response), foldUsage(response, TargetResponses), nil
			}
		}
	}
	return nil, nil, dataPlaneError(CodeUpstreamError, "the upstream stream ended without a final answer")
}

// foldUsage reads the accounting a terminal response object reported, or nil when
// it reported none, so a folded answer records the same number a non-streamed one
// would rather than a fabricated zero.
func foldUsage(response object, target string) *schema.Usage {
	usage, ok := objectField(response, "usage")
	if !ok {
		return nil
	}
	if target == TargetResponses {
		return responsesUsageFromObject(usage)
	}
	return openAIUsageFromObject(usage)
}

// foldChatEvents accumulates a chat stream into the non-streamed completion it
// stands for: content and reasoning fragments join, tool-call fragments attach to
// the call index they belong to, the last finish reason wins, the usage frame is
// taken as reported, and the caller's stop sequences cut the text the upstream
// answered past.
func foldChatEvents(events [][]byte, model string, stop []string) ([]byte, *schema.Usage, error) {
	answer := foldedChat{
		id:    "chatcmpl-pannelai",
		model: model,
		calls: map[int]*schema.ToolCall{},
		stop:  stop,
	}
	for _, payload := range events {
		chunk, ok := decodeObject(payload)
		if !ok {
			continue
		}
		answer.absorb(chunk)
	}
	if answer.seen == 0 {
		return nil, nil, dataPlaneError(CodeUpstreamError, "the upstream stream carried no answer")
	}
	return mustJSON(answer.response()), answer.usage, nil
}
