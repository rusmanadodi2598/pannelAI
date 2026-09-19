// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_stream_claude_blocks.go
// @for       Closing the Anthropic content blocks a stream opened, and framing an
//
//	event for the wire.
//
// @uses      internal/schema.
// @reason    Anthropic requires every opened block to be closed in index order, so
//
//	the closing rules are one small concern that both Finish and the delta
//	mapping call. Splitting them keeps both files inside the AGENTS.md
//	§1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import "github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"

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
