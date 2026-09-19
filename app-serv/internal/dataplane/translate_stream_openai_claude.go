// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_stream_openai_claude.go
// @for       Converting Anthropic stream events into OpenAI chunks, which is the
//
//	half of the OpenAI client stream that a Claude provider needs.
//
// @uses      internal/schema, strings.
// @reason    The OpenAI client state and its Anthropic-event mapping are two
//
//	concerns: one keeps the stream's identity and usage, the other maps
//	event types. Splitting them keeps both files inside the AGENTS.md
//	§1.1 budget, and keeping the mapping unframed lets a client state on
//	another wire reuse it.
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
	chunks := s.claudeChunks(payload)
	if len(chunks) == 0 {
		return nil
	}
	frames := make([][]byte, 0, len(chunks))
	for _, chunk := range chunks {
		frames = append(frames, Frame(chunk))
	}
	return frames
}

// claudeChunks converts one Anthropic stream event into the OpenAI chunks it
// stands for, unframed.
//
// It is separate from the framing because a client state whose own wire is not
// the OpenAI chunk wire still needs this event mapping: the Responses client
// state reads the chunks rather than the frames.
func (s *StreamState) claudeChunks(payload []byte) [][]byte {
	event, ok := decodeObject(payload)
	if !ok {
		return nil
	}
	chunks := make([][]byte, 0, 2)

	switch stringField(event, "type") {
	case schema.EventMessageStart:
		if message, ok := objectField(event, "message"); ok {
			if id := stringField(message, "id"); id != "" {
				s.ID = id
			}
			if usage, ok := objectField(message, "usage"); ok {
				s.mergeUsage(usage)
			}
		}
		chunks = append(chunks, s.chunk(schema.Delta{Role: RoleAssistant}, nil))

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
		chunks = append(chunks, s.chunk(schema.Delta{ToolCalls: []schema.ToolCallDelta{{
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
			chunks = append(chunks, frame)
		}

	case schema.EventMessageDelta:
		if delta, ok := objectField(event, "delta"); ok {
			if reason := stringField(delta, "stop_reason"); reason != "" {
				s.finishReason = openAIFinishReason(reason, TargetClaude)
			}
		}
		if usage, ok := objectField(event, "usage"); ok {
			s.mergeUsage(usage)
		}
		if s.finishReason != "" && !s.finishSent {
			reason := s.finishReason
			chunks = append(chunks, s.chunk(schema.Delta{}, &reason))
			s.finishSent = true
		}

	case schema.EventMessageStop:
		if !s.finishSent {
			reason := s.finishReason
			if reason == "" {
				reason = FinishStop
			}
			chunks = append(chunks, s.chunk(schema.Delta{}, &reason))
			s.finishSent = true
		}

	default:
		// A ping, or an event type this translator does not map, carries nothing a
		// client acts on.
		return nil
	}
	return chunks
}

// mergeUsage folds one Anthropic usage block into the stream's accounting.
//
// Anthropic reports the prompt side once, on message_start, and the output side
// cumulatively, on message_delta, so a later block replaces only the members it
// actually reports. Replacing the whole block would report a prompt of zero for
// every Claude upstream, which is what the reference avoids by accumulating
// (open-sse/translator/response/claude-to-openai.js).
func (s *StreamState) mergeUsage(usage object) {
	parsed := ClaudeUsageToOpenAI(claudeUsageFromObject(usage))
	if s.usage == nil {
		s.usage = &parsed
		return
	}
	if parsed.PromptTokens > 0 {
		s.usage.PromptTokens = parsed.PromptTokens
		s.usage.PromptTokensDetails = parsed.PromptTokensDetails
	}
	if parsed.CompletionTokens > 0 {
		s.usage.CompletionTokens = parsed.CompletionTokens
	}
	s.usage.TotalTokens = s.usage.PromptTokens + s.usage.CompletionTokens
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
