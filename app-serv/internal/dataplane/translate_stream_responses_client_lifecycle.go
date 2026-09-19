// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_stream_responses_client_lifecycle.go
// @for       The opening, closing, and numbering of the events a Responses client
//
//	stream emits.
//
// @uses      internal/schema.
// @reason    The API numbers every event and requires exactly one opening pair and
//
//	one closing event, so the numbering and the once-only guards are one
//	concern. Keeping it out of the state file holds both inside the
//	AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import "github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"

// startFrames emits the two opening events exactly once.
func (s *ResponsesStreamState) startFrames() [][]byte {
	if s.started {
		return nil
	}
	s.started = true
	id := s.responseID()
	return [][]byte{
		s.event(EventResponseCreated, map[string]any{"response": map[string]any{
			"id": id, "object": schema.ResponsesObjectResponse, "created_at": s.Created,
			"status": schema.ResponsesStatusInProgress, "background": false,
			"error": nil, "output": []any{},
		}}),
		s.event(EventResponseInProgress, map[string]any{"response": map[string]any{
			"id": id, "object": schema.ResponsesObjectResponse,
			"created_at": s.Created, "status": schema.ResponsesStatusInProgress,
		}}),
	}
}

// completedFrame emits the closing event once, reporting the assembled answer.
//
// The reference reports a bare status object here. The gateway also reports the
// items and the accounting, which the official API does and the reference omits:
// a client that reads the final answer from this event would otherwise have to
// rebuild it from the deltas.
func (s *ResponsesStreamState) completedFrame() [][]byte {
	if s.completed {
		return nil
	}
	s.completed = true
	response := map[string]any{
		"id": s.responseID(), "object": schema.ResponsesObjectResponse,
		"created_at": s.Created, "status": schema.ResponsesStatusCompleted,
		"background": false, "error": nil, "output": s.outputItems(),
	}
	if s.Model != "" {
		response["model"] = s.Model
	}
	if s.usage != nil {
		response["usage"] = mustJSON(schema.ResponsesUsageFrom(*s.usage))
	}
	return [][]byte{s.event(EventResponseCompleted, map[string]any{"response": response})}
}

// event renders one named event carrying the next sequence number.
func (s *ResponsesStreamState) event(name string, body map[string]any) []byte {
	s.seq++
	body["type"] = name
	body["sequence_number"] = s.seq
	return EventFrame(name, mustJSON(body))
}

// responseID returns the id reported to the client, derived from the upstream's
// so the two can be correlated.
func (s *ResponsesStreamState) responseID() string { return responsesAnswerID(s.ID) }

// outputItems renders every item the stream produced, in the order it opened.
func (s *ResponsesStreamState) outputItems() []any {
	items := make([]any, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, item.render())
	}
	return items
}
