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
// @stability experimental
// @since     2026-09-16
package router

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// RequestIDHeader is the header a caller may supply or receive.
const RequestIDHeader = "X-Request-Id"

// contextKeyRequestID is the private context key for the request id.
type contextKeyRequestID struct{}

// RequestIDFrom returns the request id attached by requestID, or "" when the
// middleware did not run.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(contextKeyRequestID{}).(string)
	return id
}

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
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if id == "" {
			id = domain.NewULID(time.Now())
		}
		w.Header().Set(RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKeyRequestID{}, id)))
	})
}

// logging records one structured line per request with the status and duration,
// plus the error code when the request failed (register G9), so one line answers
// "what happened" and "why" together.
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}

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
}

// SetErrorCode records the code of an error response, so the access log line can
// name it. The error writers reach this method through the envelope middleware's
// recorder, which forwards it (see statusRecorder.SetErrorCode).
func (rec *responseRecorder) SetErrorCode(code string) { rec.errorCode = code }

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
