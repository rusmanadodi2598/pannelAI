// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_stream_openai.go
// @for       Re-framing an upstream stream into OpenAI SSE frames, including the
//
//	usage chunk stream_options.include_usage asks for.
//
// @uses      internal/schema, encoding/json.
// @reason    SPEC-API-001 §4 fixes SSE as the transport and requires a usage chunk
//
//	when the client asked for one. Re-framing needs per-stream state
//	(which content block is open, the next tool-call index, the last
//	usage), and the state is passed in rather than held in a package
//	variable, so a test drives every path directly and no clock decides
//	what a frame contains. The Anthropic-event mapping lives in
//	translate_stream_openai_claude.go and the usage readers in
//	translate_usage_read.go, both for the AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"encoding/json"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// StreamState carries one client stream's translation state towards the OpenAI
// wire. It is owned by the caller and never shared.
type StreamState struct {
	// ID is the response id, taken from the upstream's first frame so a client
	// correlating its logs with the provider's has one identifier.
	ID string
	// Model is the model id reported to the client, which is the one the client
	// asked for rather than the upstream's alias.
	Model string
	// Created is the creation instant in Unix seconds. It is a field rather than
	// time.Now() so the translation is testable without a clock.
	Created int64

	// toolIndex maps an Anthropic block index onto the OpenAI tool-call index it
	// was reported as, which is what keeps streamed argument fragments attached
	// to the right call.
	toolIndex map[int]int
	// toolCalls counts the tool calls reported so far, which is the next OpenAI
	// index.
	toolCalls int

	// responses maps Responses API events onto the OpenAI chunks this state
	// frames, when the upstream speaks that format.
	responses responsesStreamState

	// usage is the last accounting block seen, so the finish frame and the usage
	// chunk can both carry it. It is nil until an upstream reports one, which is
	// what distinguishes "said zero" from "said nothing".
	usage *schema.Usage
	// finishReason is the OpenAI finish reason derived from the upstream's.
	finishReason string
	// finishSent reports whether a finish frame was already emitted, so a stream
	// that ends abruptly still gets exactly one. The upstream's own finish frame
	// counts as emitted the moment it is forwarded: a second, synthetic one is
	// what a client counting finish reasons reads as a second answer.
	finishSent bool
	// includeUsage reports whether the client asked for a usage chunk.
	includeUsage bool
	// usageSent reports whether the usage chunk was already emitted. The upstream
	// may state its numbers on the finish frame or in a frame after it, so the
	// chunk is emitted at most once, from whichever of the two sites sees the
	// numbers last.
	usageSent bool
}

// NewStreamState builds the state for one client stream.
func NewStreamState(id, model string, created int64, includeUsage bool) *StreamState {
	return &StreamState{
		ID: id, Model: model, Created: created, includeUsage: includeUsage,
		toolIndex: make(map[int]int, 4),
	}
}

// Frames re-frames one upstream payload into the frames the client receives.
//
// A nil result means the payload carried nothing a client should see: an empty
// keep-alive, or an event with no OpenAI equivalent. Emitting an empty frame
// instead would make a client that counts frames mis-count the answer.
func (s *StreamState) Frames(upstreamTarget string, payload []byte) [][]byte {
	switch upstreamTarget {
	case TargetClaude:
		return s.claudeFrames(payload)
	case TargetResponses:
		return s.responsesFrames(payload)
	default:
		return s.openAIFrames(payload)
	}
}

// responsesFrames converts one Responses event into the OpenAI frames it stands
// for: the event is mapped into the chunk vocabulary first, and the same
// re-framing every OpenAI chunk takes does the rest.
func (s *StreamState) responsesFrames(payload []byte) [][]byte {
	chunk := s.responses.chunk(payload)
	if chunk == nil {
		return nil
	}
	return s.openAIFrames(chunk)
}

// Finish returns the frames that close the stream: the final content frame when
// the upstream never sent one, the usage chunk when the client asked for it, and
// the terminal marker.
//
// It exists so a stream that ended without a finish event still terminates, which
// is what a client waiting for [DONE] needs in order to release its connection.
func (s *StreamState) Finish() [][]byte {
	frames := make([][]byte, 0, 3)
	if !s.finishSent {
		reason := s.finishReason
		if reason == "" {
			reason = FinishStop
		}
		// The frame is built by the gateway, so it is framed here: a client only
		// flushes what arrives as a complete event, and an unframed frame glued
		// onto the terminal marker is what made the panel read the stream as
		// truncated (draft 021 F1).
		frames = append(frames, Frame(s.chunk(schema.Delta{}, &reason)))
		s.finishSent = true
	}
	if s.includeUsage && s.usage != nil && !s.usageSent {
		frames = append(frames, s.usageChunk())
	}
	frames = append(frames, []byte(SSEDone))
	return frames
}

// Usage reports the accounting the stream observed, or nil when the upstream
// reported none, so a streamed call records the same number a non-streamed one
// would.
func (s *StreamState) Usage() *schema.Usage { return s.usage }

// openAIFrames re-frames an upstream OpenAI chunk, which needs only the identity
// normalisation a client expects.
func (s *StreamState) openAIFrames(payload []byte) [][]byte {
	chunk, ok := decodeObject(payload)
	if !ok {
		return nil
	}
	if id := stringField(chunk, "id"); id != "" && s.ID == "" {
		s.ID = id
	}
	if model := stringField(chunk, "model"); model != "" {
		s.Model = model
	}
	if usage, ok := objectField(chunk, "usage"); ok {
		s.usage = openAIUsageFromObject(usage)
	}
	if choices, ok := arrayField(chunk, "choices"); ok && len(choices) > 0 {
		if first, ok := decodeObject(choices[0]); ok {
			if reason := stringField(first, "finish_reason"); reason != "" {
				s.finishReason = reason
				// The frame about to be forwarded is the client's finish frame,
				// so the stream is finished as of now and Finish must not add a
				// second one (draft 021 F3).
				s.finishSent = true
			}
		}
	}

	// The identity is rewritten in place and every other member is forwarded
	// verbatim: re-encoding a payload the gateway did not change would drop any
	// field the schema does not model.
	chunk["id"] = mustJSON(s.responseID())
	chunk["object"] = mustJSON("chat.completion.chunk")
	chunk["model"] = mustJSON(s.Model)
	if _, ok := chunk["created"]; !ok {
		chunk["created"] = mustJSON(s.Created)
	}
	encoded, err := json.Marshal(chunk)
	if err != nil {
		// reason: the payload decoded once already, so a re-encode failure means a
		// member holds a value json cannot render; forwarding the original bytes is
		// closer to correct than dropping the frame.
		encoded = payload
	}

	// The usage chunk is left to Finish, which runs for every stream: emitting it
	// here as well is what sent two of them, the first priced before the
	// upstream's numbers had arrived (draft 021 F2).
	return [][]byte{Frame(encoded)}
}
