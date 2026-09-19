// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_stream_responses_client_items.go
// @for       The output items a Responses client stream reports: how one is
//
//	allocated, named, and rendered, and how all of them close.
//
// @uses      internal/schema, strconv.
// @reason    The API requires every item to be opened before its content and
//
//	closed after it, and pairs the two by id, so the item shape and the
//	closing order are one concern. The reasoning, message, and call
//	lifecycles live in the sibling files, for the AGENTS.md §1.1 budget.
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

// responsesClientItem is one output item a stream opened, with the buffers its
// closing event reports.
type responsesClientItem struct {
	// kind is the item type, and index the output index it opened at.
	kind  string
	index int
	id    string
	// done reports whether the closing events were emitted, so a stream that
	// ends after a finish reason does not close the same item twice.
	done bool
	// text accumulates a reasoning item's summary or a message item's text.
	text string
	// callID, name, and arguments describe a function_call item.
	callID    string
	name      string
	arguments string
}

// newItem allocates the next output item. An empty name lets the id be derived
// from the response id and the item's own index, which is the shape the API gives
// reasoning and message items.
func (s *ResponsesStreamState) newItem(kind, name string) *responsesClientItem {
	item := &responsesClientItem{kind: kind, index: s.nextIndex}
	if name == "" {
		item.id = itemPrefix(kind) + s.responseID() + "_" + strconv.Itoa(s.nextIndex)
	} else {
		item.id = name
	}
	s.nextIndex++
	s.items = append(s.items, item)
	return item
}

// itemPrefix is the id prefix the API gives each item kind.
func itemPrefix(kind string) string {
	switch kind {
	case schema.ResponsesItemReasoning:
		return "rs_"
	case schema.ResponsesItemMessage:
		return "msg_"
	default:
		return "fc_"
	}
}

// opened builds the item object an opening event reports, which carries none of
// the content the deltas and the closing event fill in. The empty members are
// present rather than omitted, because a client reads them as the baseline it
// appends the deltas to.
func (i *responsesClientItem) opened() map[string]any {
	switch i.kind {
	case schema.ResponsesItemReasoning:
		return map[string]any{"id": i.id, "type": i.kind, "summary": []any{}}
	case schema.ResponsesItemFunctionCall:
		return map[string]any{
			"id": i.id, "type": i.kind, "arguments": "",
			"call_id": i.callID, "name": i.name,
		}
	default:
		return map[string]any{
			"id": i.id, "type": i.kind, "content": []any{}, "role": schema.RoleAssistant,
		}
	}
}

// render builds the item object a closing event reports.
func (i *responsesClientItem) render() map[string]any {
	switch i.kind {
	case schema.ResponsesItemReasoning:
		return map[string]any{
			"id": i.id, "type": i.kind,
			"summary": []any{map[string]any{"type": schema.ResponsesSummaryText, "text": i.text}},
		}
	case schema.ResponsesItemFunctionCall:
		return map[string]any{
			"id": i.id, "type": i.kind, "call_id": i.callID,
			"name": i.name, "arguments": i.callArguments(),
		}
	default:
		return map[string]any{
			"id": i.id, "type": i.kind, "role": schema.RoleAssistant,
			"content": []any{map[string]any{
				"type": schema.ResponsesPartOutputText, "text": i.text,
				"annotations": []any{}, "logprobs": []any{},
			}},
		}
	}
}

// callArguments reports a call's arguments, defaulting to an empty object: a
// client that parses the field would fail on an empty string.
func (i *responsesClientItem) callArguments() string {
	if i.arguments == "" {
		return "{}"
	}
	return i.arguments
}

// closeItems closes every open item, in the order the items opened, so a client
// pairing each opening event with a closing one sees a consistent order whatever
// sequence the upstream sent.
func (s *ResponsesStreamState) closeItems() [][]byte {
	frames := make([][]byte, 0, len(s.items)*3)
	for _, item := range s.items {
		if item.done {
			continue
		}
		switch item.kind {
		case schema.ResponsesItemReasoning:
			frames = append(frames, s.closeReasoning()...)
		case schema.ResponsesItemFunctionCall:
			frames = append(frames, s.closeCall(item)...)
		default:
			frames = append(frames, s.closeMessage()...)
		}
	}
	return frames
}
