// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_stream_claude.go
// @for       Re-framing an upstream stream into Anthropic SSE events, so a client
//
//	on /api/v1/messages can be served by any provider.
//
// @uses      internal/schema, encoding/json.
// @reason    SPEC-API-001 §7.15 serves POST /api/v1/messages on the Anthropic wire,
//
//	and the resolved provider may speak OpenAI — so the framing has to be
//	produced, not forwarded. Anthropic's stream is stricter than OpenAI's:
//	every content block must be opened, fed, and closed in order, so this
//	is a small state machine whose state the caller owns.
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
	chunk, ok := decodeObject(payload)
	if !ok {
		return nil
	}
	if upstreamTarget == TargetClaude {
		// The upstream already speaks Anthropic, so its events are forwarded after
		// the identity fields the client expects are set.
		return s.forwardClaude(chunk)
	}
	return s.fromOpenAI(chunk)
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

// deltaFrames converts one OpenAI delta, opening and closing Anthropic blocks as
// the content type changes: Anthropic has no equivalent of a single delta that
// carries text and a tool call at once.
func (s *ClaudeStreamState) deltaFrames(delta object) [][]byte {
	frames := make([][]byte, 0, 2)

	if thinking := stringField(delta, "reasoning_content"); thinking != "" {
		frames = append(frames, s.closeText()...)
		if s.thinkingIndex < 0 {
			s.thinkingIndex = s.nextIndex
			s.nextIndex++
			frames = append(frames, eventFrame(schema.EventContentBlockStart, mustFrame(map[string]any{
				"type":  schema.EventContentBlockStart,
				"index": s.thinkingIndex,
				"content_block": map[string]any{
					"type": schema.BlockThinking, "thinking": "",
				},
			})))
		}
		frames = append(frames, eventFrame(schema.EventContentBlockDelta, mustFrame(map[string]any{
			"type":  schema.EventContentBlockDelta,
			"index": s.thinkingIndex,
			"delta": map[string]any{"type": schema.DeltaThinking, "thinking": thinking},
		})))
	}

	if content := stringField(delta, "content"); content != "" {
		frames = append(frames, s.closeThinking()...)
		if s.textIndex < 0 {
			s.textIndex = s.nextIndex
			s.nextIndex++
			frames = append(frames, eventFrame(schema.EventContentBlockStart, mustFrame(map[string]any{
				"type":  schema.EventContentBlockStart,
				"index": s.textIndex,
				"content_block": map[string]any{
					"type": schema.BlockText, "text": "",
				},
			})))
		}
		frames = append(frames, eventFrame(schema.EventContentBlockDelta, mustFrame(map[string]any{
			"type":  schema.EventContentBlockDelta,
			"index": s.textIndex,
			"delta": map[string]any{"type": schema.DeltaText, "text": content},
		})))
	}

	frames = append(frames, s.toolCallFrames(delta)...)
	return frames
}

// toolCallFrames opens a tool_use block for a new tool call and feeds argument
// fragments into it.
func (s *ClaudeStreamState) toolCallFrames(delta object) [][]byte {
	calls, ok := arrayField(delta, "tool_calls")
	if !ok {
		return nil
	}
	frames := make([][]byte, 0, len(calls)*2)
	for _, raw := range calls {
		call, ok := decodeObject(raw)
		if !ok {
			continue
		}
		openAIIndex := intField(call, "index")
		function, hasFunction := objectField(call, "function")

		if id := stringField(call, "id"); id != "" {
			frames = append(frames, s.closeText()...)
			frames = append(frames, s.closeThinking()...)
			blockIndex := s.nextIndex
			s.nextIndex++
			s.toolIndex[openAIIndex] = blockIndex
			name := ""
			if hasFunction {
				name = stringField(function, "name")
			}
			frames = append(frames, eventFrame(schema.EventContentBlockStart, mustFrame(map[string]any{
				"type":  schema.EventContentBlockStart,
				"index": blockIndex,
				"content_block": map[string]any{
					"type": schema.BlockToolUse, "id": id, "name": name, "input": map[string]any{},
				},
			})))
		}

		if !hasFunction {
			continue
		}
		fragment := stringField(function, "arguments")
		blockIndex, known := s.toolIndex[openAIIndex]
		if fragment == "" || !known {
			continue
		}
		frames = append(frames, eventFrame(schema.EventContentBlockDelta, mustFrame(map[string]any{
			"type":  schema.EventContentBlockDelta,
			"index": blockIndex,
			"delta": map[string]any{"type": schema.DeltaInputJSON, "partial_json": fragment},
		})))
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

// closeText stops the open text block, if one is open.
func (s *ClaudeStreamState) closeText() [][]byte {
	if s.textIndex < 0 {
		return nil
	}
	index := s.textIndex
	s.textIndex = -1
	return [][]byte{blockStopFrame(index)}
}

// closeThinking stops the open thinking block, if one is open.
func (s *ClaudeStreamState) closeThinking() [][]byte {
	if s.thinkingIndex < 0 {
		return nil
	}
	index := s.thinkingIndex
	s.thinkingIndex = -1
	return [][]byte{blockStopFrame(index)}
}

// closeBlocks stops every open block, ordered by index so the client sees them
// close in the order they opened.
func (s *ClaudeStreamState) closeBlocks() [][]byte {
	frames := make([][]byte, 0, 3)
	frames = append(frames, s.closeThinking()...)
	frames = append(frames, s.closeText()...)
	for index := 0; index < s.nextIndex; index++ {
		if isToolBlock(s.toolIndex, index) {
			frames = append(frames, blockStopFrame(index))
		}
	}
	s.toolIndex = make(map[int]int, 0)
	return frames
}

// isToolBlock reports whether a block index was opened as a tool_use block.
func isToolBlock(index map[int]int, blockIndex int) bool {
	for _, value := range index {
		if value == blockIndex {
			return true
		}
	}
	return false
}

// blockStopFrame builds a content_block_stop event.
func blockStopFrame(index int) []byte {
	return eventFrame(schema.EventContentBlockStop, mustFrame(map[string]any{
		"type": schema.EventContentBlockStop, "index": index,
	}))
}

// eventFrame renders a JSON payload as a named Anthropic SSE event.
func eventFrame(event string, payload []byte) []byte {
	return EventFrame(event, payload)
}
