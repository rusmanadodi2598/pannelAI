// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_stream_claude.go
// @for       Re-framing an upstream stream into Anthropic SSE events, so a client
//
//	on /api/v1/messages can be served by any provider.
//
// @uses      internal/schema.
// @reason    SPEC-API-001 §7.15 serves POST /api/v1/messages on the Anthropic wire,
//
//	and the resolved provider may speak OpenAI or the Responses API, so the
//	framing has to be produced, not forwarded. Anthropic's stream is
//	stricter than OpenAI's: every content block must be opened, fed, and
//	closed in order, so this is a small state machine whose state the
//	caller owns. Its delta and block-closing helpers live in
//	translate_stream_claude_delta.go and translate_stream_claude_blocks.go,
//	for the AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import "github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"

// ClaudeStreamState carries one Anthropic-wire stream's translation state.
type ClaudeStreamState struct {
	// ID and Model are the identifiers a client sees; the upstream's id is kept
	// when it reported one.
	ID    string
	Model string

	// started reports whether message_start was emitted. Anthropic requires it
	// to be the first event, so it is emitted lazily on the first upstream frame
	// and again by Finish when nothing ever arrived.
	started bool
	// nextIndex is the Anthropic content-block index the next block gets; the
	// indices must be contiguous because a client pairs each start with a stop.
	nextIndex int
	// textIndex and thinkingIndex are the open blocks, or -1 when closed.
	textIndex     int
	thinkingIndex int
	// toolIndex maps an OpenAI tool-call index onto the Anthropic block index it
	// was opened as, so argument fragments land on the right block.
	toolIndex map[int]int

	// responses maps Responses API events onto the OpenAI chunks this state
	// frames, when the upstream speaks that format.
	responses responsesStreamState

	// usage is the accounting folded into the closing message_delta, which is
	// where Anthropic carries it.
	usage *schema.Usage
	// stopReason is the Anthropic stop reason derived from the upstream's.
	stopReason string
}

// NewClaudeStreamState builds the state for one client stream.
func NewClaudeStreamState(id, model string) *ClaudeStreamState {
	return &ClaudeStreamState{
		ID: id, Model: model, textIndex: -1, thinkingIndex: -1,
		toolIndex: make(map[int]int, 4),
	}
}

// Frames converts one upstream payload into Anthropic events.
func (s *ClaudeStreamState) Frames(upstreamTarget string, payload []byte) [][]byte {
	switch upstreamTarget {
	case TargetClaude:
		chunk, ok := decodeObject(payload)
		if !ok {
			return nil
		}
		// The upstream already speaks Anthropic, so its events are forwarded after
		// the identity fields the client expects are set.
		return s.forwardClaude(chunk)
	case TargetResponses:
		return s.fromResponses(payload)
	default:
		chunk, ok := decodeObject(payload)
		if !ok {
			return nil
		}
		return s.fromOpenAI(chunk)
	}
}

// fromResponses converts one Responses event by mapping it into the OpenAI chunk
// vocabulary first, then framing that chunk: the same path an OpenAI upstream
// takes, so the event mapping lives in one place.
func (s *ClaudeStreamState) fromResponses(payload []byte) [][]byte {
	chunk := s.responses.chunk(payload)
	if chunk == nil {
		return nil
	}
	decoded, ok := decodeObject(chunk)
	if !ok {
		return nil
	}
	return s.fromOpenAI(decoded)
}

// Finish closes the stream: any open block is stopped, the closing
// message_delta carries the stop reason and the usage, and message_stop ends it.
func (s *ClaudeStreamState) Finish() [][]byte {
	frames := make([][]byte, 0, 4)
	frames = append(frames, s.startFrame())
	frames = append(frames, s.closeBlocks()...)

	usage := schema.MessagesUsage{}
	if s.usage != nil {
		usage = OpenAIToClaudeUsage(*s.usage)
	}
	stopReason := s.stopReason
	if stopReason == "" {
		stopReason = StopEndTurn
	}
	frames = append(frames,
		eventFrame(schema.EventMessageDelta, mustFrame(map[string]any{
			"type":  schema.EventMessageDelta,
			"delta": map[string]any{"stop_reason": stopReason, "stop_sequence": nil},
			"usage": usage,
		})),
		eventFrame(schema.EventMessageStop, mustFrame(map[string]any{"type": schema.EventMessageStop})),
	)
	return frames
}

// Usage reports the accounting the stream observed, or nil when the upstream
// reported none.
func (s *ClaudeStreamState) Usage() *schema.Usage { return s.usage }

// forwardClaude re-emits an Anthropic event the upstream already framed, keeping
// the client's identity for the opening message.
func (s *ClaudeStreamState) forwardClaude(chunk object) [][]byte {
	if eventType := stringField(chunk, "type"); eventType != "" {
		if message, ok := objectField(chunk, "message"); ok {
			if id := stringField(message, "id"); id != "" && s.ID == "" {
				s.ID = id
			}
		}
		if usage, ok := objectField(chunk, "usage"); ok {
			parsed := ClaudeUsageToOpenAI(claudeUsageFromObject(usage))
			s.usage = &parsed
		}
		s.started = true
		return [][]byte{eventFrame(eventType, mustJSON(chunk))}
	}
	return nil
}

// fromOpenAI converts one OpenAI chunk into the Anthropic events it stands for.
func (s *ClaudeStreamState) fromOpenAI(chunk object) [][]byte {
	if id := stringField(chunk, "id"); id != "" && s.ID == "" {
		s.ID = id
	}
	if model := stringField(chunk, "model"); model != "" {
		s.Model = model
	}
	if usage, ok := objectField(chunk, "usage"); ok {
		s.usage = openAIUsageFromObject(usage)
	}

	frames := make([][]byte, 0, 3)
	frames = append(frames, s.startFrame())

	choices, ok := arrayField(chunk, "choices")
	if !ok || len(choices) == 0 {
		// A usage-only frame carries no choice, so nothing else to emit.
		return frames
	}
	choice, ok := decodeObject(choices[0])
	if !ok {
		return frames
	}
	delta, hasDelta := objectField(choice, "delta")
	if hasDelta {
		frames = append(frames, s.deltaFrames(delta)...)
	}

	if reason := stringField(choice, "finish_reason"); reason != "" {
		s.stopReason = claudeStopReason(reason)
	}
	return frames
}

// startFrame emits message_start exactly once, which Anthropic requires as the
// first event of a stream.
func (s *ClaudeStreamState) startFrame() []byte {
	if s.started {
		return nil
	}
	s.started = true
	return eventFrame(schema.EventMessageStart, mustFrame(map[string]any{
		"type": schema.EventMessageStart,
		"message": map[string]any{
			"id": responseID(s.ID, "msg_pannelai"), "type": "message", "role": RoleAssistant,
			"model": s.Model, "content": []schema.Block{}, "stop_reason": nil, "stop_sequence": nil,
			"usage": schema.MessagesUsage{},
		},
	}))
}
