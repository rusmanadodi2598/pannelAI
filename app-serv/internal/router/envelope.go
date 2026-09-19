// Package router maps HTTP routes to handlers.
//
// @file      internal/router/envelope.go
// @for       Restating the mux's own 404 and 405 answers in the §8 envelope.
// @uses      encoding/json, log/slog, net/http, internal/schema.
// @reason    SPEC-API-001 §8 gives every management error one shape, and the
//
//	mux produces two of them itself: an unknown path and a wrong verb
//	are answered before any handler runs. Those answers are plain text,
//	so they are withheld and re-written here. This lives apart from
//	router.go because it is middleware, not the route table, and because
//	the route table is already at the §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-19
package router

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// envelope restates the mux's own plain-text 404/405 answers in the management
// error envelope (SPEC-API-001 §8). It intercepts only responses no handler
// produced: a handler always sets a JSON content type before writing, so
// anything already JSON passes through untouched and is never double-bodied.
func envelope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		if !rec.suppressed {
			return
		}
		switch rec.status {
		case http.StatusNotFound:
			writeEnvelope(w, http.StatusNotFound, "NOT_FOUND", "route not found")
		case http.StatusMethodNotAllowed:
			writeEnvelope(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method is not allowed for this route")
		}
	})
}

// writeEnvelope emits the §8 error shape for a response the mux produced and
// this middleware suppressed, so no other body exists for that request.
func writeEnvelope(w http.ResponseWriter, code int, errCode, msg string) {
	if recorder, ok := w.(schema.ErrorCodeRecorder); ok {
		recorder.SetErrorCode(errCode)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(schema.ErrorBody{Error: schema.ErrorDetail{Code: errCode, Message: msg}}); err != nil {
		slog.Error("encoding routing error envelope failed", "status", code, "error", err)
	}
}

// statusRecorder intercepts only the mux's built-in 404/405 answers. Those are
// the responses that carry a text/plain content type; a handler response always
// sets application/json first, so it is forwarded verbatim.
type statusRecorder struct {
	http.ResponseWriter
	status     int
	suppressed bool
}

func (rec *statusRecorder) WriteHeader(code int) {
	if rec.suppressed {
		return
	}
	builtin := code == http.StatusNotFound || code == http.StatusMethodNotAllowed
	if builtin && rec.Header().Get("Content-Type") != "application/json" {
		// Withhold the mux's plain-text body so envelope() can replace it.
		rec.suppressed = true
		rec.status = code
		return
	}
	rec.ResponseWriter.WriteHeader(code)
}

func (rec *statusRecorder) Write(b []byte) (int, error) {
	if rec.suppressed {
		return len(b), nil
	}
	return rec.ResponseWriter.Write(b)
}

// SetErrorCode forwards an error code to the recorder inside, so the access log
// can name why the request failed (register G9). The forwarding is explicit
// because Go promotes only the methods of the embedded interface
// (http.ResponseWriter) and not the extra ones the concrete value behind it
// carries — without this method a handler's SetErrorCode assertion would fail on
// the writer it actually receives, and the code would silently stop being logged.
func (rec *statusRecorder) SetErrorCode(code string) {
	if recorder, ok := rec.ResponseWriter.(schema.ErrorCodeRecorder); ok {
		recorder.SetErrorCode(code)
	}
}
