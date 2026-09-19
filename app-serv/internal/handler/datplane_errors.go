// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/datplane_errors.go
// @for       The OpenAI error envelope writer and the SSE sink the data plane
//
//	responses use.
//
// @uses      internal/dataplane, internal/schema, net/http, log/slog.
// @reason    SPEC-API-001 §4 fixes the data plane envelope
//
//	(`{"error":{"message","type","code"}}`) and §8 makes it the shape a CLI
//	tool reads, so the data plane never borrows the management envelope.
//	Both the writer and the sink are in one file because they are the two
//	ways a data plane response is produced: as a body, or as frames.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-17
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// openAIErrorBody is the §4 data plane error envelope.
type openAIErrorBody struct {
	Error openAIErrorDetail `json:"error"`
}

// openAIErrorDetail carries the message, the type a client groups on, and the
// machine code.
type openAIErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// writeDataPlaneError renders any error in the OpenAI envelope (§4).
//
// The wrapped cause is never written: only the data plane's own message reaches a
// client, so an upstream's raw body or a driver message cannot leak through this
// path (AGENTS.md §1.3).
//
// The access log records the machine code and never the message (register G9): a
// data-plane failure's message can quote an upstream's text back, and that text
// can carry the credential the gateway sent, so only the code is safe to log.
func writeDataPlaneError(w http.ResponseWriter, err error) {
	failure := dataplane.AsError(err)
	if failure == nil {
		failure = dataplane.AsError(dataplane.InternalError("an unexpected error occurred", nil))
	}
	if recorder, ok := w.(schema.ErrorCodeRecorder); ok {
		recorder.SetErrorCode(failure.Code)
	}
	if failure.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(failure.RetryAfter))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(failure.OpenAIStatus())
	if encodeErr := json.NewEncoder(w).Encode(openAIErrorBody{Error: openAIErrorDetail{
		Message: failure.Message, Type: failure.Type, Code: failure.Code,
	}}); encodeErr != nil {
		slog.Error("encoding data plane error failed", "code", failure.Code, "error", encodeErr)
	}
}

// writeDataPlaneBody writes a body the data plane already translated.
//
// The bytes are forwarded rather than a decoded value re-encoded, because the
// answer may carry fields the gateway does not model: parsing and re-rendering it
// would drop exactly the fields a client added a dependency on.
func writeDataPlaneBody(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		slog.Error("writing data plane response failed", "status", status, "error", err)
	}
}

// writeDataPlaneRaw writes a body that is not JSON — the audio bytes a speech
// call answers with, or a transcription upstream's plain-text form. The content
// type is the caller's because only the route knows what it produced.
func writeDataPlaneRaw(w http.ResponseWriter, status int, contentType string, body []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		slog.Error("writing data plane response failed", "status", status, "error", err)
	}
}

// sseSink writes SSE frames to a response writer, flushing each one.
type sseSink struct {
	writer  http.ResponseWriter
	flusher http.Flusher
	wroteH  bool
}

// newSSESink builds the sink, resolving the flusher once so a stream does not
// re-test for it per frame.
func newSSESink(w http.ResponseWriter) *sseSink {
	flusher, _ := w.(http.Flusher)
	return &sseSink{writer: w, flusher: flusher}
}

// WriteFrame writes one complete frame.
func (s *sseSink) WriteFrame(frame []byte) error {
	if _, err := s.writer.Write(frame); err != nil {
		return err
	}
	s.wroteH = true
	return nil
}

// Flush pushes what has been written, so a client sees each frame as it arrives
// rather than at the end of the answer.
func (s *sseSink) Flush() {
	if s.flusher != nil {
		s.flusher.Flush()
	}
}

// wrote reports whether any frame has been written, which is what decides whether
// an error can still be reported as a body or the status line is already
// committed.
func (s *sseSink) wrote() bool { return s.wroteH }
