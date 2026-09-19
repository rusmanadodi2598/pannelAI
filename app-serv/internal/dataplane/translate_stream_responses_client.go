// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_stream_responses_client.go
// @for       Re-framing an upstream stream into the Responses API named-event
//
//	lifecycle, so a client on /api/v1/responses can be served by any
//	provider.
//
// @uses      internal/schema.
// @reason    SPEC-API-001 §7.15 serves POST /api/v1/responses, whose stream is a
//
//	named-event stream rather than a chunk stream. Every upstream format
//	reaches this state in the OpenAI chunk vocabulary the other two client
//	stream states already read, so the lifecycle is written once. The
//	lifecycle events live in translate_stream_responses_client_lifecycle.go
//	and the item mapping in translate_stream_responses_client_items.go,
//	both for the AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import "github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"

// The Responses event names this state emits that the upstream-direction mapper
// does not already declare. The shared names live in translate_stream_responses.go.
const (
	EventResponseInProgress       = "response.in_progress"
	EventResponseContentPartAdded = "response.content_part.added"
	EventResponseContentPartDone  = "response.content_part.done"
	EventResponseOutputTextDone   = "response.output_text.done"
	EventResponseReasoningPartAdd = "response.reasoning_summary_part.added"
	EventResponseReasoningPartEnd = "response.reasoning_summary_part.done"
	EventResponseReasoningDone    = "response.reasoning_summary_text.done"
	EventResponseArgumentsDone    = "response.function_call_arguments.done"
)

// ResponsesStreamState carries one Responses-wire stream's translation state. It
// is owned by the caller and never shared.
type ResponsesStreamState struct {
	// ID is the upstream's response id; the id the client sees is derived from
	// it, so a client correlating its logs with the provider's has one.
	ID string
	// Model is the model id reported to the client.
	Model string
	// Created is the creation instant in Unix seconds. It is a field rather than
	// time.Now() so the translation is testable without a clock.
	Created int64

	// claude maps Anthropic events onto the chunk vocabulary and responses maps
	// Responses events onto it, so all three upstream formats reach the item
	// mapping in one shape.
	claude    StreamState
	responses responsesStreamState

	// seq is the last sequence_number handed out, which the API requires to
	// increase by one across the stream.
	seq int
	// started and completed report whether the opening and closing events were
	// emitted, so each is emitted exactly once.
	started   bool
	completed bool
	// nextIndex is the output index the next item opens at. Every item gets its
	// own index, which is what keeps a reasoning item and the message that
	// follows it from sharing one.
	nextIndex int

	// items holds every item in the order it opened, which is the order the
	// closing events and the completed answer report them in.
	items []*responsesClientItem
	// reasoning and message are the open text items, or nil.
	reasoning *responsesClientItem
	message   *responsesClientItem
	// calls holds the tool-call items by their OpenAI index, so argument
	// fragments land on the call they belong to.
	calls map[int]*responsesClientItem
	// thinking reports whether inline reasoning markup is currently routed to
	// the reasoning item.
	thinking bool

	// usage is the accounting the upstream reported, or nil when it reported
	// none, which is what distinguishes "said zero" from "said nothing".
	usage *schema.Usage
}

// NewResponsesStreamState builds the state for one client stream.
//
// The Anthropic converter is seeded with the id the answer falls back to, so a
// stream that never reported one still produces the documented placeholder
// rather than the chunk wire's own.
func NewResponsesStreamState(id, model string, created int64) *ResponsesStreamState {
	return &ResponsesStreamState{
		ID: id, Model: model, Created: created,
		claude: *NewStreamState("pannelai", model, created, false),
		calls:  make(map[int]*responsesClientItem, 4),
	}
}

// Frames converts one upstream payload into the events the client receives.
func (s *ResponsesStreamState) Frames(upstreamTarget string, payload []byte) [][]byte {
	switch upstreamTarget {
	case TargetClaude:
		chunks := s.claude.claudeChunks(payload)
		if id := s.claude.ID; id != "" {
			s.ID = id
		}
		if usage := s.claude.Usage(); usage != nil {
			s.usage = usage
		}
		return s.chunksFrames(chunks)
	case TargetResponses:
		chunk := s.responses.chunk(payload)
		if chunk == nil {
			return nil
		}
		return s.chunkFrames(chunk)
	default:
		return s.chunkFrames(payload)
	}
}

// chunksFrames feeds each chunk one upstream event produced through the item
// mapping, which is what lets an Anthropic upstream reach the same lifecycle as
// an OpenAI one.
func (s *ResponsesStreamState) chunksFrames(chunks [][]byte) [][]byte {
	frames := make([][]byte, 0, len(chunks))
	for _, chunk := range chunks {
		frames = append(frames, s.chunkFrames(chunk)...)
	}
	return frames
}

// chunkFrames converts one OpenAI chunk into the events it stands for.
func (s *ResponsesStreamState) chunkFrames(payload []byte) [][]byte {
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

	frames := s.startFrames()

	choices, ok := arrayField(chunk, "choices")
	if !ok || len(choices) == 0 {
		// A usage-only frame carries no choice, so there is nothing to emit.
		return frames
	}
	choice, ok := decodeObject(choices[0])
	if !ok {
		return frames
	}
	if delta, ok := objectField(choice, "delta"); ok {
		frames = append(frames, s.deltaEvents(delta)...)
	}
	if reason := stringField(choice, "finish_reason"); reason != "" {
		frames = append(frames, s.closeItems()...)
	}
	return frames
}

// Finish closes the stream: every open item is closed, the closing event reports
// the answer, and the terminal marker releases the client's connection.
func (s *ResponsesStreamState) Finish() [][]byte {
	frames := make([][]byte, 0, 12)
	frames = append(frames, s.startFrames()...)
	frames = append(frames, s.closeItems()...)
	frames = append(frames, s.completedFrame()...)
	return append(frames, []byte(SSEDone))
}

// Usage reports the accounting the stream observed, or nil when the upstream
// reported none, so a streamed call records the same number a non-streamed one
// would.
func (s *ResponsesStreamState) Usage() *schema.Usage { return s.usage }
