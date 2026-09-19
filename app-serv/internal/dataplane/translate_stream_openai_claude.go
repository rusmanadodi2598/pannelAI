// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_stream_openai_claude.go
// @for       Converting Anthropic stream events into OpenAI frames, which is the
//
//	half of the OpenAI client stream that a Claude provider needs.
//
// @uses      internal/schema, strings.
// @reason    The OpenAI client state and its Anthropic-event mapping are two
//
//	concerns: one keeps the stream's identity and usage, the other maps
//	event types. Splitting them keeps both files inside the AGENTS.md
//	§1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// claudeFrames converts one Anthropic stream event into OpenAI frames, keeping the
// tool-call indices OpenAI clients expect.
func (s *StreamState) claudeFrames(payload []byte) [][]byte {
	event, ok := decodeObject(payload)
	if !ok {
		return nil
	}
	frames := make([][]byte, 0, 2)

	switch stringField(event, "type") {
	case schema.EventMessageStart:
		if message, ok := objectField(event, "message"); ok {
			if id := stringField(message, "id"); id != "" {
				s.ID = id
			}
			if usage, ok := objectField(message, "usage"); ok {
				// A message_start usage block only carries the prompt side, so the
				// stream keeps the last one it sees rather than the first.
				parsed := ClaudeUsageToOpenAI(claudeUsageFromObject(usage))
				s.usage = &parsed
			}
		}
		frames = append(frames, s.chunk(schema.Delta{Role: RoleAssistant}, nil))

	case schema.EventContentBlockStart:
		block, ok := objectField(event, "content_block")
		if !ok {
			break
		}
		if stringField(block, "type") != schema.BlockToolUse {
			break
		}
		index := intField(event, "index")
		s.toolIndex[index] = s.toolCalls
		s.toolCalls++
		frames = append(frames, s.chunk(schema.Delta{ToolCalls: []schema.ToolCallDelta{{
			Index: s.toolIndex[index],
			ID:    stringField(block, "id"),
			Type:  schema.BlockFunction,
			Function: &schema.FunctionDelta{
				Name: strings.TrimPrefix(stringField(block, "name"), ClaudeToolPrefix),
			},
		}}}, nil))

	case schema.EventContentBlockDelta:
		delta, ok := objectField(event, "delta")
		if !ok {
			break
		}
		if frame := s.blockDeltaFrame(intField(event, "index"), delta); frame != nil {
			frames = append(frames, frame)
		}

	case schema.EventMessageDelta:
		if delta, ok := objectField(event, "delta"); ok {
			if reason := stringField(delta, "stop_reason"); reason != "" {
				s.finishReason = openAIFinishReason(reason, TargetClaude)
			}
		}
		if usage, ok := objectField(event, "usage"); ok {
			parsed := ClaudeUsageToOpenAI(claudeUsageFromObject(usage))
			s.usage = &parsed
		}
		if s.finishReason != "" && !s.finishSent {
			reason := s.finishReason
			frames = append(frames, s.chunk(schema.Delta{}, &reason))
			s.finishSent = true
		}

	case schema.EventMessageStop:
		if !s.finishSent {
			reason := s.finishReason
			if reason == "" {
				reason = FinishStop
			}
			frames = append(frames, s.chunk(schema.Delta{}, &reason))
			s.finishSent = true
		}

	default:
		// A ping, or an event type this translator does not map, carries nothing a
		// client acts on.
		return nil
	}
	return frames
}

// blockDeltaFrame builds the frame one Anthropic content delta stands for, or nil
// when the delta is empty or belongs to an unknown block.
func (s *StreamState) blockDeltaFrame(index int, delta object) []byte {
	switch stringField(delta, "type") {
	case schema.DeltaText:
		if text := stringField(delta, "text"); text != "" {
			return s.chunk(schema.Delta{Content: text}, nil)
		}
	case schema.DeltaThinking:
		if thinking := stringField(delta, "thinking"); thinking != "" {
			return s.chunk(schema.Delta{ReasoningContent: thinking}, nil)
		}
	case schema.DeltaInputJSON:
		fragment := stringField(delta, "partial_json")
		if fragment == "" {
			return nil
		}
		toolIndex, ok := s.toolIndex[index]
		if !ok {
			return nil
		}
		return s.chunk(schema.Delta{ToolCalls: []schema.ToolCallDelta{{
			Index:    toolIndex,
			Function: &schema.FunctionDelta{Arguments: fragment},
		}}}, nil)
	}
	return nil
}
