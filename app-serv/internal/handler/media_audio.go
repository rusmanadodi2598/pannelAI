// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/media_audio.go
// @for       The §7.10 speech, transcription, and voice routes.
// @uses      internal/schema, internal/service, net/http, time.
// @reason    Three of the six media routes answer with something other than a JSON object the service produced: speech answers bytes (or base64 when asked), and a transcription forwards whatever the upstream said. The handler is where that choice belongs, so the service never writes to a response.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-09-19
package handler

import (
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// A 25 MB multipart body outlives the server's 30-second ReadTimeout on a slow
// link, so this route renews the connection deadlines. The write budget is the
// server's own two minutes, set on top of the upload window because the upload
// may have consumed it before the upstream call even starts.
const (
	transcriptionReadBudget  = 5 * time.Minute
	transcriptionWriteBudget = 2 * time.Minute
)

// Speech serves POST /api/v1/audio/speech. The audio is returned as bytes, or
// base64 inside a JSON object when the caller asks for `?response_format=json`.
func (h *MediaHandler) Speech(w http.ResponseWriter, r *http.Request) {
	// The key is checked before the body is read: a request that may not
	// proceed must not cost the gateway a parse.
	keyID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	raw, err := schema.ReadBody(r)
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	req, err := schema.DecodeSpeechRequest(raw)
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		writeDataPlaneError(w, err)
		return
	}

	answer, call, err := h.media.Speech(r.Context(), req, nil, keyID)
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	format := call.SpeechOutputFormat(req)
	if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("response_format")), "json") {
		schema.WriteJSON(w, http.StatusOK, schema.SpeechResponse{
			Audio: base64.StdEncoding.EncodeToString(answer.Body), Format: format,
		})
		return
	}
	writeDataPlaneRaw(w, answer.Status, schema.AudioContentType(format), answer.Body)
}

// Transcribe serves POST /api/v1/audio/transcriptions. The upstream's own answer
// is forwarded: the reference's OpenAI-compatible path returns it verbatim, and
// re-encoding a transcription would drop the fields a caller asked for with
// `response_format`.
func (h *MediaHandler) Transcribe(w http.ResponseWriter, r *http.Request) {
	// Authentication comes before the upload is parsed: a 25 MB multipart body
	// must not be read, or spilled to disk, for a caller without a key.
	keyID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	// A writer that cannot carry a deadline (HTTP/2) reports unsupported exactly
	// like the SSE sink's Flush, and the upload is still read under the server
	// timeout.
	controller := http.NewResponseController(w)
	_ = controller.SetReadDeadline(time.Now().Add(transcriptionReadBudget))
	_ = controller.SetWriteDeadline(time.Now().Add(transcriptionReadBudget + transcriptionWriteBudget))
	r.Body = http.MaxBytesReader(w, r.Body, schema.MaxTranscriptionBytes)
	form, err := schema.ReadTranscriptionForm(r)
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}

	answer, _, err := h.media.Transcribe(r.Context(), form, nil, keyID)
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	if schema.TranscriptionJSON(answer.Body) {
		writeDataPlaneBody(w, answer.Status, answer.Body)
		return
	}
	writeDataPlaneRaw(w, answer.Status, "text/plain; charset=utf-8", answer.Body)
}

// Voices serves GET /api/v1/audio/voices. The provider is required: the catalog
// is per provider, and listing every provider's would answer a question the
// client did not ask.
func (h *MediaHandler) Voices(w http.ResponseWriter, r *http.Request) {
	// The catalog comes from the registry, so no upstream call is made and
	// nothing is recorded beyond the key's own use counter.
	if _, ok := h.authorize(w, r); !ok {
		return
	}
	providerID := strings.TrimSpace(r.URL.Query().Get("provider"))
	if providerID == "" {
		writeDataPlaneError(w, dataplane.ValidationError("query parameter provider is required"))
		return
	}
	voices, err := h.media.Voices(providerID)
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	list := schema.VoiceList{Object: "list", Data: make([]schema.VoiceObject, 0, len(voices))}
	for _, voice := range voices {
		list.Data = append(list.Data, schema.VoiceObject{
			ID: voice.ID, Name: voice.Name, Lang: voice.Lang, Gender: voice.Gender,
		})
	}
	schema.WriteJSON(w, http.StatusOK, list)
}
