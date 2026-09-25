// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_stream_openai_frames.go
// @for       The OpenAI chunks the gateway builds itself: one content frame, the
//
//	usage chunk, the stream's identity, and the marshal guard.
//
// @uses      encoding/json, internal/schema.
// @reason    The frame builders are the half of the OpenAI client wire the
//
//	gateway authors rather than forwards, and they are what draft 021's
//	framing findings are about. Keeping them beside the type definition
//	crossed the AGENTS.md §1.1 split warning, so they live here while the
//	translator keeps the state and the upstream re-framing.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-25
package dataplane

import (
	"encoding/json"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// chunk builds one OpenAI chunk from the stream's identity. It is the raw JSON,
// not an SSE frame: the Anthropic and Responses client wires build their own
// events out of these chunks, and only the OpenAI wire frames them.
func (s *StreamState) chunk(delta schema.Delta, finishReason *string) []byte {
	return mustFrame(schema.ChatCompletionChunk{
		ID:      s.responseID(),
		Object:  "chat.completion.chunk",
		Created: s.Created,
		Model:   s.Model,
		Choices: []schema.ChunkChoice{{Index: 0, Delta: delta, FinishReason: finishReason}},
	})
}

// usageChunk builds the usage chunk a client that asked for one receives. It is
// its own frame with an empty choices array, which is the shape OpenAI documents
// for stream_options.include_usage: a client that reads the first choice from
// every frame must not read content here. Emitting it marks usageSent, so one
// stream carries at most one (draft 021 F2).
func (s *StreamState) usageChunk() []byte {
	s.usageSent = true
	return Frame(mustFrame(schema.UsageChunk(s.ID, s.Created, s.Model, *s.usage)))
}

// responseID returns the id reported to the client, falling back to a stable
// placeholder when the upstream reported none.
func (s *StreamState) responseID() string {
	if s.ID == "" {
		return "chatcmpl-pannelai"
	}
	return s.ID
}

// mustFrame marshals a frame the translator built itself, reporting a failure as
// a JSON null rather than panicking on the request path.
func mustFrame(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		return []byte("null")
	}
	return encoded
}
