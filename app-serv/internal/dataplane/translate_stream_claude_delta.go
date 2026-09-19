// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_stream_claude_delta.go
// @for       Converting one OpenAI delta into the Anthropic events it stands for.
// @uses      internal/schema.
// @reason    The delta mapping opens and closes content blocks as the content type
//
//	changes, which is the part of the Anthropic stream that has no OpenAI
//	equivalent. It is split from the stream state so both files stay
//	inside the AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import "github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"

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
