// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_stream_responses.go
// @for       Mapping Responses API SSE events onto the OpenAI chunk vocabulary
//
//	the two client stream states already consume.
//
// @uses      encoding/json, strconv.
// @reason    SPEC-API-001 §10 makes the Responses API a P3 deliverable, and its
//
//	stream is a named-event stream rather than a chunk stream. Mapping
//	each event onto the chunk shape both client states already frame is
//	what keeps this from being two more translators, one per client wire.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"strconv"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// The Responses stream event names this mapper reads. Every other event carries
// nothing a chat client shows, so it produces no frame.
const (
	EventResponseCreated          = "response.created"
	EventResponseOutputTextDelta  = "response.output_text.delta"
	EventResponseReasoningDelta   = "response.reasoning_summary_text.delta"
	EventResponseItemAdded        = "response.output_item.added"
	EventResponseItemDone         = "response.output_item.done"
	EventResponseArgumentsDelta   = "response.function_call_arguments.delta"
	EventResponseCustomInputDelta = "response.custom_tool_call_input.delta"
	EventResponseCompleted        = "response.completed"
	EventResponseDone             = "response.done"
	EventResponseFailed           = "response.failed"
	EventResponseError            = "error"
)

// responsesStreamState carries the per-stream state a Responses source needs:
// the upstream's response id and the tool-call index the next call gets.
type responsesStreamState struct {
	// responseID is the id every frame reports, read from the first event that
	// carries one.
	responseID string
	// toolIndex is the OpenAI tool-call index the next call gets. The Responses
	// stream names one call at a time, so the index advances with each call.
	toolIndex int
	// sawToolCall makes the finish reason tool_calls, which is what a client
	// dispatches on.
	sawToolCall bool
}

// chunk maps one Responses event onto the OpenAI chunk it stands for, or nil
// when the event carries nothing a client shows. A nil result is deliberate:
// emitting an empty frame would make a client that counts frames mis-count the
// answer.
func (r *responsesStreamState) chunk(payload []byte) []byte {
	event, ok := decodeObject(payload)
	if !ok {
		return nil
	}
	response, _ := objectField(event, "response")
	if id := stringField(response, "id"); id != "" {
		r.responseID = id
	}

	switch stringField(event, "type") {
	case EventResponseOutputTextDelta:
		if delta := stringField(event, "delta"); delta != "" {
			return r.chunkWith(map[string]any{"content": delta}, "")
		}
	case EventResponseReasoningDelta:
		if delta := stringField(event, "delta"); delta != "" {
			return r.chunkWith(map[string]any{"reasoning_content": delta}, "")
		}
	case EventResponseItemAdded:
		return r.itemAdded(event)
	case EventResponseArgumentsDelta, EventResponseCustomInputDelta:
		if delta := stringField(event, "delta"); delta != "" {
			return r.argumentsDelta(delta)
		}
	case EventResponseItemDone:
		if item, ok := objectField(event, "item"); ok && isResponsesCallItem(item) {
			r.toolIndex++
			return nil
		}
	case EventResponseCompleted, EventResponseDone:
		return r.completed(response)
	case EventResponseFailed, EventResponseError:
		return r.failure(event, response)
	}
	return nil
}

// itemAdded opens a tool call when the added item is a function call, and
// produces nothing for the message and reasoning items: their text arrives as
// deltas.
func (r *responsesStreamState) itemAdded(event object) []byte {
	item, ok := objectField(event, "item")
	if !ok || !isResponsesCallItem(item) {
		return nil
	}
	callID := stringField(item, "call_id")
	if callID == "" {
		// A provider that omits the id still needs one the client can correlate
		// with the argument fragments, so a deterministic placeholder is used
		// rather than a clock-derived one a test could not pin.
		callID = "call_pannelai_" + strconv.Itoa(r.toolIndex)
	}
	r.sawToolCall = true
	return r.chunkWith(map[string]any{"tool_calls": []any{map[string]any{
		"index": r.toolIndex,
		"id":    callID,
		"type":  schema.BlockFunction,
		"function": map[string]any{
			"name": stringField(item, "name"), "arguments": "",
		},
	}}}, "")
}

// argumentsDelta feeds one argument fragment into the open tool call.
func (r *responsesStreamState) argumentsDelta(delta string) []byte {
	return r.chunkWith(map[string]any{"tool_calls": []any{map[string]any{
		"index":    r.toolIndex,
		"function": map[string]any{"arguments": delta},
	}}}, "")
}

// completed closes the answer: the finish reason the stream implies and the
// accounting the upstream reported, in the shape the client states read.
func (r *responsesStreamState) completed(response object) []byte {
	finish := FinishStop
	if r.sawToolCall {
		finish = FinishToolCalls
	}
	chunk := r.chunkWith(map[string]any{}, finish)
	body, ok := decodeObject(chunk)
	if !ok {
		return chunk
	}
	if usage, ok := objectField(response, "usage"); ok {
		body["usage"] = mustJSON(*responsesUsageFromObject(usage))
	}
	return mustJSON(body)
}

// failure surfaces an upstream error as the frame the reference emits: a chunk
// carrying the message as content. A client mid-stream has no other channel for
// it, and dropping the event would leave it waiting for an answer that ended.
func (r *responsesStreamState) failure(event, response object) []byte {
	message := "the upstream stream failed"
	if detail, ok := objectField(event, "error"); ok {
		if text := stringField(detail, "message"); text != "" {
			message = text
		}
	}
	if response != nil {
		if detail, ok := objectField(response, "error"); ok {
			if text := stringField(detail, "message"); text != "" {
				message = text
			}
		}
	}
	r.sawToolCall = false
	return r.chunkWith(map[string]any{"content": "[Error] " + message}, FinishStop)
}

// chunkWith builds one OpenAI chunk from the stream's identity. An empty finish
// reason renders as null, which is what a client expects mid-answer.
func (r *responsesStreamState) chunkWith(delta map[string]any, finishReason string) []byte {
	choice := map[string]any{"index": 0, "delta": delta, "finish_reason": nil}
	if finishReason != "" {
		choice["finish_reason"] = finishReason
	}
	return mustJSON(map[string]any{
		"id":      r.responseID,
		"object":  "chat.completion.chunk",
		"choices": []any{choice},
	})
}

// isResponsesCallItem reports whether an output item is a function call, in
// either the standard or the custom-tool spelling the reference maps the same.
func isResponsesCallItem(item object) bool {
	switch stringField(item, "type") {
	case ItemFunctionCall, "custom_tool_call":
		return true
	default:
		return false
	}
}
