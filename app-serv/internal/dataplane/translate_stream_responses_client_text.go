// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_stream_responses_client_text.go
// @for       The reasoning and message items a Responses client stream reports,
//
//	including the inline reasoning markup an upstream may wrap its
//	thinking in.
//
// @uses      internal/schema, strings.
// @reason    Text is the only item kind that changes destination mid-delta, so the
//
//	routing between the reasoning item and the message item is one
//	concern. Keeping it out of the item shape holds both files inside the
//	AGENTS.md §1.1 budget.
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

// The inline reasoning markup an upstream may wrap its thinking in. The reference
// routes it to the reasoning item rather than the answer's text.
const (
	thinkOpen  = "<think>"
	thinkClose = "</think>"
)

// deltaEvents converts one OpenAI delta into the events it stands for. The order
// is the order a client renders them in: reasoning, then the answer's text, then
// the calls the text announced.
func (s *ResponsesStreamState) deltaEvents(delta object) [][]byte {
	frames := make([][]byte, 0, 4)
	if thinking := stringField(delta, "reasoning_content"); thinking != "" {
		frames = append(frames, s.openReasoning()...)
		frames = append(frames, s.reasoningDelta(thinking)...)
	}
	if content := stringField(delta, "content"); content != "" {
		frames = append(frames, s.contentEvents(content)...)
	}
	return append(frames, s.callEvents(delta)...)
}

// contentEvents routes a content delta to the reasoning item or to the message
// item, honouring the inline markup an upstream may wrap its reasoning in.
func (s *ResponsesStreamState) contentEvents(content string) [][]byte {
	frames := make([][]byte, 0, 4)
	if strings.Contains(content, thinkOpen) {
		s.thinking = true
		content = strings.Replace(content, thinkOpen, "", 1)
		frames = append(frames, s.openReasoning()...)
	}
	if close := strings.Index(content, thinkClose); close >= 0 {
		tail := content[close+len(thinkClose):]
		if s.thinking {
			if head := content[:close]; head != "" {
				frames = append(frames, s.reasoningDelta(head)...)
			}
			frames = append(frames, s.closeReasoning()...)
			s.thinking = false
		} else {
			// A closing tag with no opening one is not a reasoning boundary, so
			// what preceded it stays answer text. The tag itself is markup either
			// way, and leaving it would show it to the user.
			tail = content[:close] + tail
		}
		content = tail
	}
	if s.thinking {
		if content != "" {
			frames = append(frames, s.reasoningDelta(content)...)
		}
		return frames
	}
	if content == "" {
		return frames
	}
	frames = append(frames, s.openMessage()...)
	return append(frames, s.messageDelta(content)...)
}

// openReasoning opens a reasoning item and its one summary part. A reasoning
// block that follows a closed one gets a fresh item rather than deltas against a
// finished one.
func (s *ResponsesStreamState) openReasoning() [][]byte {
	if s.reasoning != nil && !s.reasoning.done {
		return nil
	}
	item := s.newItem(schema.ResponsesItemReasoning, "")
	s.reasoning = item
	return [][]byte{
		s.event(EventResponseItemAdded, map[string]any{
			"output_index": item.index, "item": item.opened(),
		}),
		s.event(EventResponseReasoningPartAdd, map[string]any{
			"item_id": item.id, "output_index": item.index, "summary_index": 0,
			"part": map[string]any{"type": schema.ResponsesSummaryText, "text": ""},
		}),
	}
}

// reasoningDelta feeds one fragment into the open reasoning item.
func (s *ResponsesStreamState) reasoningDelta(text string) [][]byte {
	if s.reasoning == nil || s.reasoning.done {
		return nil
	}
	s.reasoning.text += text
	return [][]byte{s.event(EventResponseReasoningDelta, map[string]any{
		"item_id": s.reasoning.id, "output_index": s.reasoning.index,
		"summary_index": 0, "delta": text,
	})}
}

// closeReasoning closes the open reasoning item, if one is open.
func (s *ResponsesStreamState) closeReasoning() [][]byte {
	item := s.reasoning
	if item == nil || item.done {
		return nil
	}
	item.done = true
	return [][]byte{
		s.event(EventResponseReasoningDone, map[string]any{
			"item_id": item.id, "output_index": item.index,
			"summary_index": 0, "text": item.text,
		}),
		s.event(EventResponseReasoningPartEnd, map[string]any{
			"item_id": item.id, "output_index": item.index, "summary_index": 0,
			"part": map[string]any{"type": schema.ResponsesSummaryText, "text": item.text},
		}),
		s.event(EventResponseItemDone, map[string]any{
			"output_index": item.index, "item": item.render(),
		}),
	}
}

// openMessage opens a message item, unless the current one is still open. Text
// that arrives after a tool call opens a fresh item rather than feeding deltas to
// one the client already saw close.
func (s *ResponsesStreamState) openMessage() [][]byte {
	if s.message != nil && !s.message.done {
		return nil
	}
	item := s.newItem(schema.ResponsesItemMessage, "")
	s.message = item
	return [][]byte{
		s.event(EventResponseItemAdded, map[string]any{
			"output_index": item.index, "item": item.opened(),
		}),
		s.event(EventResponseContentPartAdded, map[string]any{
			"item_id": item.id, "output_index": item.index, "content_index": 0,
			"part": map[string]any{
				"type": schema.ResponsesPartOutputText, "annotations": []any{},
				"logprobs": []any{}, "text": "",
			},
		}),
	}
}

// messageDelta feeds one text fragment into the open message item.
func (s *ResponsesStreamState) messageDelta(text string) [][]byte {
	if s.message == nil || s.message.done {
		return nil
	}
	s.message.text += text
	return [][]byte{s.event(EventResponseOutputTextDelta, map[string]any{
		"item_id": s.message.id, "output_index": s.message.index,
		"content_index": 0, "delta": text, "logprobs": []any{},
	})}
}

// closeMessage closes the open message item, if one is open.
func (s *ResponsesStreamState) closeMessage() [][]byte {
	item := s.message
	if item == nil || item.done {
		return nil
	}
	item.done = true
	return [][]byte{
		s.event(EventResponseOutputTextDone, map[string]any{
			"item_id": item.id, "output_index": item.index,
			"content_index": 0, "text": item.text, "logprobs": []any{},
		}),
		s.event(EventResponseContentPartDone, map[string]any{
			"item_id": item.id, "output_index": item.index, "content_index": 0,
			"part": map[string]any{
				"type": schema.ResponsesPartOutputText, "annotations": []any{},
				"logprobs": []any{}, "text": item.text,
			},
		}),
		s.event(EventResponseItemDone, map[string]any{
			"output_index": item.index, "item": item.render(),
		}),
	}
}
