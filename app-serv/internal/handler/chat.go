// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/chat.go
// @for       The chat data plane routes: POST /api/v1/chat/completions,
//
//	POST /api/v1/messages, and GET /api/v1/models.
//
// @uses      internal/schema, internal/service, net/http.
// @reason    SPEC-API-001 §7.15 serves three routes on the OpenAI and Anthropic
//
//	wires, and §4 fixes the data plane auth header. AGENTS.md §1.5 keeps
//	SQL and Redis out of here and validation in the schema layer, so this
//	file only decodes, authenticates, calls, and encodes.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-17
package handler

import (
	"net/http"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

const tokenSaverBypassHeader = "X-Token-Saver"

// ChatHandler serves the chat side of the data plane (§7.15).
type ChatHandler struct {
	chat *service.ChatService
}

// NewChatHandler validates deps and returns the handler.
func NewChatHandler(chat *service.ChatService) *ChatHandler {
	return &ChatHandler{chat: chat}
}

// Completions serves POST /api/v1/chat/completions on the OpenAI wire.
func (h *ChatHandler) Completions(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, dataplane.RouteChatCompletions, schema.FormatOpenAI)
}

// Messages serves POST /api/v1/messages on the Anthropic wire.
func (h *ChatHandler) Messages(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, dataplane.RouteMessages, schema.FormatAnthropic)
}

// Responses serves POST /api/v1/responses on the Responses API wire.
func (h *ChatHandler) Responses(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, dataplane.RouteResponses, schema.FormatOpenAIResponses)
}

// serve runs the shared pipeline for both chat routes: read the body, decode and
// validate it against the route's contract, authenticate, then relay.
func (h *ChatHandler) serve(w http.ResponseWriter, r *http.Request, route dataplane.Route, format schema.DataPlaneFormat) {
	raw, err := schema.ReadBody(r)
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}

	request := dataplane.Request{
		Route: route, ClientFormat: format, Raw: raw,
		TokenSaverBypass: tokenSaverBypass(r),
	}
	switch format {
	case schema.FormatAnthropic:
		decoded, decodeErr := schema.DecodeMessagesRequest(raw)
		if decodeErr != nil {
			writeDataPlaneError(w, decodeErr)
			return
		}
		if validateErr := schema.ValidateStruct(decoded); validateErr != nil {
			writeDataPlaneError(w, validateErr)
			return
		}
		request.Messages = &decoded
		request.Model = decoded.Model
		request.Stream = decoded.Stream
	case schema.FormatOpenAIResponses:
		decoded, decodeErr := schema.DecodeResponsesRequest(raw)
		if decodeErr != nil {
			writeDataPlaneError(w, decodeErr)
			return
		}
		if validateErr := schema.ValidateStruct(decoded); validateErr != nil {
			writeDataPlaneError(w, validateErr)
			return
		}
		request.Responses = &decoded
		request.Model = decoded.Model
		request.Stream = decoded.Stream
	default:
		decoded, decodeErr := schema.DecodeChatRequest(raw)
		if decodeErr != nil {
			writeDataPlaneError(w, decodeErr)
			return
		}
		if validateErr := schema.ValidateStruct(decoded); validateErr != nil {
			writeDataPlaneError(w, validateErr)
			return
		}
		request.Chat = &decoded
		request.Model = decoded.Model
		request.Stream = decoded.Stream
		request.IncludeUsage = decoded.IncludeUsage()
	}

	key, err := h.chat.Authenticate(r.Context(), bearerToken(r))
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}

	if request.Stream {
		h.stream(w, r, request, key.ID())
		return
	}
	outcome, err := h.chat.Relay(r.Context(), request, nil, key.ID())
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	writeDataPlaneBody(w, http.StatusOK, outcome.Body)
}

// stream relays a streamed answer, flushing each frame as it is produced so a CLI
// tool sees output rather than one delayed blob (§4).
func (h *ChatHandler) stream(w http.ResponseWriter, r *http.Request, request dataplane.Request, keyID string) {
	sink := newSSESink(w)
	dataplane.DataPlaneHeaders(w.Header())
	w.WriteHeader(http.StatusOK)

	outcome, err := h.chat.Relay(r.Context(), request, sink, keyID)
	if err != nil && !sink.wrote() {
		// A failure before the first frame can still be reported as an error body;
		// after one, the status line is committed and the stream simply ends, which
		// is the only honest answer a client can act on.
		writeDataPlaneError(w, err)
		return
	}
	_ = outcome
}

// Models serves GET /api/v1/models in the OpenAI list shape (§7.15).
func (h *ChatHandler) Models(w http.ResponseWriter, r *http.Request) {
	key, err := h.chat.Authenticate(r.Context(), bearerToken(r))
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	_ = key
	list, err := h.chat.Models(r.Context())
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, list)
}

// tokenSaverBypass reports whether this request opted out of all token savers.
// Only the exact case-insensitive value "off" is a bypass; every other value
// keeps the stored configuration active.
func tokenSaverBypass(r *http.Request) bool {
	return strings.EqualFold(strings.TrimSpace(r.Header.Get(tokenSaverBypassHeader)), "off")
}

// bearerToken reads the §4 data plane credential. A bare token is accepted too,
// because that is what several CLI tools send, and the header is the whole
// contract the spec fixes.
func bearerToken(r *http.Request) string {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "Bearer "
	if len(header) > len(prefix) && strings.EqualFold(header[:len(prefix)], prefix) {
		return strings.TrimSpace(header[len(prefix):])
	}
	return strings.TrimSpace(r.Header.Get("X-Api-Key"))
}
