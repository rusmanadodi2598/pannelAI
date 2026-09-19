// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_stream_responses_client_calls.go
// @for       The function_call items a Responses client stream reports, from the
//
//	first fragment that names a call to the closer that reports its
//	arguments.
//
// @uses      internal/schema, strconv.
// @reason    A call arrives as fragments spread across deltas, so it needs an
//
//	accumulator keyed by the OpenAI tool-call index that the other item
//	kinds do not. Keeping it separate holds every stream file inside the
//	AGENTS.md §1.1 budget.
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

// callEvents opens a tool-call item for each new call in the delta and feeds the
// argument fragments into it.
func (s *ResponsesStreamState) callEvents(delta object) [][]byte {
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
		frames = append(frames, s.callEvent(call)...)
	}
	return frames
}

// callEvent handles one tool-call entry of a delta: the opening fragment carries
// the id and the name, and later fragments carry the arguments.
func (s *ResponsesStreamState) callEvent(call object) [][]byte {
	key := intField(call, "index")
	function, _ := objectField(call, "function")
	name := stringField(function, "name")

	item, known := s.calls[key]
	if !known {
		callID := stringField(call, "id")
		if callID == "" {
			// A provider that omits the id still needs one the client can
			// correlate the fragments with, so a deterministic placeholder is
			// used rather than a clock-derived one a test could not pin.
			callID = "call_pannelai_" + strconv.Itoa(key)
		}
		// The text that was streaming is closed first: a message item and a call
		// item cannot be the same item.
		frames := s.closeMessage()
		item = s.newItem(schema.ResponsesItemFunctionCall, "fc_"+callID)
		item.callID, item.name = callID, name
		s.calls[key] = item
		frames = append(frames, s.event(EventResponseItemAdded, map[string]any{
			"output_index": item.index, "item": item.opened(),
		}))
		return append(frames, s.callDelta(item, stringField(function, "arguments"))...)
	}
	if name != "" && item.name == "" {
		item.name = name
	}
	return s.callDelta(item, stringField(function, "arguments"))
}

// callDelta feeds one argument fragment into an open call item.
func (s *ResponsesStreamState) callDelta(item *responsesClientItem, fragment string) [][]byte {
	if fragment == "" {
		return nil
	}
	item.arguments += fragment
	return [][]byte{s.event(EventResponseArgumentsDelta, map[string]any{
		"item_id": item.id, "output_index": item.index, "delta": fragment,
	})}
}

// closeCall closes one tool-call item.
func (s *ResponsesStreamState) closeCall(item *responsesClientItem) [][]byte {
	item.done = true
	return [][]byte{
		s.event(EventResponseArgumentsDone, map[string]any{
			"item_id": item.id, "output_index": item.index,
			"arguments": item.callArguments(),
		}),
		s.event(EventResponseItemDone, map[string]any{
			"output_index": item.index, "item": item.render(),
		}),
	}
}
