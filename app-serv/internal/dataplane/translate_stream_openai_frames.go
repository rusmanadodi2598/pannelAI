// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_stream_openai_frames.go
// @for       The OpenAI chunk layer: the frames the gateway builds itself (the
//
//	content frame, the usage chunk, the stream identity, the marshal
//	guard) and the re-framing of one upstream OpenAI chunk.
//
// @uses      encoding/json, internal/schema.
// @reason    The frame builders are the half of the OpenAI client wire the
//
//	gateway authors rather than forwards, and they are what draft 021's
//	framing findings are about. The upstream re-framing joined them
//	here for the AGENTS.md §1.1 budget: the translator keeps the state
//	and this file owns what a frame is.
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
// every frame must not read content here. Emitting it marks usageSent, and so
// does forwarding an upstream frame that already carries usage, so one stream
// carries at most one delivery of the numbers (draft 021 F2, 034 F1).
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

// openAIFrames re-frames an upstream OpenAI chunk, which needs only the identity
// normalisation a client expects.
func (s *StreamState) openAIFrames(payload []byte) [][]byte {
	chunk, ok := decodeObject(payload)
	if !ok {
		return nil
	}
	if id := stringField(chunk, "id"); id != "" && s.ID == "" {
		s.ID = id
	}
	// The name the client is given is the one it asked for, which is what this
	// field's own comment has always claimed. Adopting the upstream's `model`
	// member broke that: Qoder answers every model it serves as `auto`, so a
	// client that asked `qoder/qfmodel` was told its answer came from `auto` —
	// a name it cannot re-send, and one that routes to a pool the vendor
	// answers 429 for. The echo is only used when nothing was asked for, where
	// it is the sole name available.
	if s.Model == "" {
		s.Model = stringField(chunk, "model")
	}
	if usage, ok := objectField(chunk, "usage"); ok {
		s.usage = openAIUsageFromObject(usage)
		// A forwarded frame that itself carries usage is the delivery the client
		// asked for, so it marks usageSent and Finish appends nothing (draft 034
		// F1). A null or empty member is not numbers on the wire, which leaves
		// the decision to Finish's own guard.
		if len(usage) > 0 {
			s.usageSent = true
		}
	}
	if choices, ok := arrayField(chunk, "choices"); ok && len(choices) > 0 {
		if first, ok := decodeObject(choices[0]); ok {
			if reason := stringField(first, "finish_reason"); reason != "" {
				// A second frame that closes again repeats the finish reason,
				// which a client counting finish reasons reads as a second
				// answer (draft 034 F2). The duplicate member is nulled and
				// every other member, the usage it may carry included, still
				// forwards; the first reason stays the stream's finish.
				if s.finishSent {
					first["finish_reason"] = mustJSON(nil)
					choices[0] = mustJSON(first)
					chunk["choices"] = mustJSON(choices)
				} else {
					// The frame about to be forwarded is the client's finish
					// frame, so the stream is finished as of now and Finish
					// must not add a second one (draft 021 F3).
					s.finishReason = reason
					s.finishSent = true
				}
			}
		}
	}

	// The vendor's own members are shaped last, after the stream's finish and
	// usage have been read from what actually arrived: the guard decides what the
	// client sees, not what this call is billed for.
	if !s.sanitizeChunk(chunk) {
		return nil
	}

	// The identity is rewritten in place and every other member is forwarded
	// verbatim: re-encoding a payload the gateway did not change would drop any
	// field the schema does not model.
	chunk["id"] = mustJSON(s.responseID())
	chunk["object"] = mustJSON("chat.completion.chunk")
	chunk["model"] = mustJSON(s.Model)
	if _, ok := chunk["created"]; !ok {
		chunk["created"] = mustJSON(s.Created)
	}
	encoded, err := json.Marshal(chunk)
	if err != nil {
		// reason: the payload decoded once already, so a re-encode failure means a
		// member holds a value json cannot render; forwarding the original bytes is
		// closer to correct than dropping the frame.
		encoded = payload
	}

	// The usage chunk is left to Finish, which runs for every stream: emitting it
	// here as well is what sent two of them, the first priced before the
	// upstream's numbers had arrived (draft 021 F2).
	return [][]byte{Frame(encoded)}
}
