// Package router maps HTTP routes to handlers.
//
// @file      internal/router/middleware.go
// @for       Cross-cutting HTTP middleware: request id, access log, panic recovery.
// @uses      internal/domain, internal/schema, log/slog, net/http, time.
// @reason    SPEC-API-001 §4 requires every request to be tagged with a
//
//	request_id and logged structurally, and §8 requires an
//	INTERNAL_ERROR to be logged with that id. AGENTS.md §1.6 makes
//	panic recovery at every boundary non-negotiable, so the 500 path
//	reports the same id an operator can grep for.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability stable
// @since     2026-09-16
package router

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/requestctx"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// RequestIDHeader is the header a caller may supply or receive.
const RequestIDHeader = "X-Request-Id"

// RequestIDFrom returns the request id attached by requestID, or "" when the
// middleware did not run. The key itself lives in internal/requestctx, below the
// router and the handlers, so a handler can read the id for its own error log
// without importing this package upward (AGENTS.md §1.5).
func RequestIDFrom(ctx context.Context) string { return requestctx.From(ctx) }

// chain wraps the mux in the middleware every route needs. The outermost
// layers are the ones that must observe everything inside them: the request id
// (so recovery and logging can report it) and the access log (so a recovered
// 500 is still recorded).
func chain(h http.Handler) http.Handler {
	return requestID(logging(recoverer(envelope(h))))
}

// requestID gives every request an id, echoes it in the response, and puts it in
// the context so error paths can attach it to their log lines (§4, §8). An id
// supplied by the caller is kept, which lets a client correlate its own trace.
//
// A supplied id is kept only within the shape that is safe to reflect and log.
// The value goes back in a response header and into every access-log line of the
// request, so an unbounded one lets a single header put a megabyte into both, and
// a forged id can poison an operator's grep for one trace.
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if !usableRequestID(id) {
			id = domain.NewULID(time.Now())
		}
		w.Header().Set(RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(requestctx.With(r.Context(), id)))
	})
}

// maxRequestIDLength bounds a caller-supplied id. Real trace ids are short, and a
// ULID is 26 characters, so the cap only ever rejects noise.
const maxRequestIDLength = 64

func usableRequestID(id string) bool {
	if id == "" || len(id) > maxRequestIDLength {
		return false
	}
	for index := 0; index < len(id); index++ {
		if id[index] < 0x20 || id[index] == 0x7f {
			return false
		}
	}
	return true
}

// logging records one structured line per request with the status and duration,
// plus the error code when the request failed (register G9), so one line answers
// "what happened" and "why" together.
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		rec := &responseRecorder{
			ResponseWriter: w, status: http.StatusOK,
			requestID: RequestIDFrom(r.Context()),
		}

		next.ServeHTTP(rec, r)

		attrs := []slog.Attr{
			slog.String("request_id", RequestIDFrom(r.Context())),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.status),
			slog.Int64("duration_ms", time.Since(started).Milliseconds()),
		}
		// A failed request names its code; a served one carries no code field at
		// all, so the field's presence means exactly "this request failed".
		// The message is never logged: it can quote an upstream's text back,
		// and that text can carry a credential (register G18).
		if rec.errorCode != "" {
			attrs = append(attrs, slog.String("code", rec.errorCode))
		}
		slog.LogAttrs(r.Context(), slog.LevelInfo, "request handled", attrs...)
	})
}

// recoverer stops a panic in one request from killing the process (AGENTS.md
// §1.6: a panic in an unrecovered goroutine takes down the whole binary) and
// answers with the §8 envelope, logged against the request id.
func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read the id up front: the deferred closure then needs no context
		// access, and the value it logs is the one the request started with.
		requestID := RequestIDFrom(r.Context())
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("request panic recovered",
					"request_id", requestID,
					"method", r.Method,
					"path", r.URL.Path,
					"panic", rec,
				)
				schema.WriteError(w, domain.NewInternalError("internal server error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// responseRecorder captures the status for the access log while forwarding every
// write to the real writer.
type responseRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	// errorCode is the machine code of the error envelope the handler wrote, or
	// "" for a request that produced no error envelope. The error writers set it
	// through SetErrorCode and the access log reads it (register G9).
	errorCode string
	// requestID is the trace id of the request this writer answers, captured from
	// the context the request-id middleware already filled. A handler's error path
	// reads it through schema.RequestIDCarrier so its own log line can be joined to
	// the access-log line below.
	requestID string
}

func (rec *responseRecorder) RequestID() string { return rec.requestID }

// SetErrorCode records the code of an error response, so the access log line can
// name it. The error writers reach this method through the envelope middleware's
// recorder, which forwards it (see statusRecorder.SetErrorCode).
func (rec *responseRecorder) SetErrorCode(code string) { rec.errorCode = code }

// Flush forwards the flush to the writer inside. Go promotes only the methods of
// the embedded interface and not http.Flusher, so without this method the writer
// a handler receives stops implementing http.Flusher under the chain, `newSSESink`
// stores a nil flusher, and every streamed answer is written at once (draft 010
// F5). It is the same forwarding rule as SetErrorCode for the same reason.
func (rec *responseRecorder) Flush() {
	if flusher, ok := rec.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Unwrap exposes the writer inside, so http.ResponseController (Go 1.20+) can
// reach every optional capability through the wrapper, not just the two this
// file forwards by name.
func (rec *responseRecorder) Unwrap() http.ResponseWriter { return rec.ResponseWriter }

func (rec *responseRecorder) WriteHeader(code int) {
	if !rec.wroteHeader {
		rec.status = code
		rec.wroteHeader = true
	}
	rec.ResponseWriter.WriteHeader(code)
}

func (rec *responseRecorder) Write(b []byte) (int, error) {
	if !rec.wroteHeader {
		rec.wroteHeader = true
	}
	return rec.ResponseWriter.Write(b)
}
