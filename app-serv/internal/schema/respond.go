// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/respond.go
// @for       The single serialization exit point for management handlers.
// @uses      encoding/json, net/http, internal/domain.
// @reason    AGENTS.md §1.3 requires every client response to be structured,
//
//	and SPEC-API-001 §8 requires the error envelope; funnelling both
//	through here keeps no handler inventing a shape or leaking a
//	driver message.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-16
package schema

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// HealthResponse is returned by GET /api/v1/health (SPEC-API-001 §7.1).
type HealthResponse struct {
	Status    string            `json:"status"`
	Checks    map[string]string `json:"checks"`
	Version   string            `json:"version"`
	Commit    string            `json:"commit"`
	Timestamp string            `json:"timestamp"`
}

// SystemInfo is returned by GET /api/v1/version (§7.1): build version, commit,
// and the revision of the embedded provider registry.
type SystemInfo struct {
	Version          string `json:"version"`
	Commit           string `json:"commit"`
	BuildDate        string `json:"build_date"`
	GoVersion        string `json:"go_version"`
	RegistryRevision string `json:"registry_revision"`
}

// ErrorBody is the management error envelope (SPEC-API-001 §8).
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail carries the machine code and the English message.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Timestamp formats a time as RFC3339 UTC so every handler renders
// identically (SPEC-API-001 §4: timestamps are RFC3339 UTC).
func Timestamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// WriteJSON marshals v and writes it with the given status.
//
// An encode failure cannot be turned into an error response: the status line is
// already committed by then, so the client would receive a truncated body
// either way. It is logged instead of discarded, because a silent truncation is
// far harder to diagnose than a log line naming the request.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encoding response body failed", "status", status, "error", err)
	}
}

// ErrorCodeRecorder receives the machine code of an error response, so the
// access log line can name why a request failed instead of only its status
// (register G9). The router's access-log recorder implements it and the
// envelope middleware forwards it through the writer chain; a writer that does
// not implement it is simply not asked, which is what keeps the seam optional
// for a test driving a handler with a plain httptest.ResponseRecorder.
type ErrorCodeRecorder interface {
	SetErrorCode(code string)
}

// WriteError converts any error to the management envelope and writes it.
// Unknown errors become INTERNAL_ERROR so driver internals never leak (§8).
//
// The code is recorded before the body is written, so the access log names it
// even though the log line is produced after the handler returns.
func WriteError(w http.ResponseWriter, err error) {
	appErr := domain.AsAppError(err)
	if recorder, ok := w.(ErrorCodeRecorder); ok {
		recorder.SetErrorCode(appErr.Code)
	}
	if appErr.RetryAfter > 0 {
		seconds := int64((appErr.RetryAfter + time.Second - 1) / time.Second)
		if seconds < 1 {
			seconds = 1
		}
		w.Header().Set("Retry-After", strconv.FormatInt(seconds, 10))
	}
	WriteJSON(w, appErr.HTTPStatus(), ErrorBody{Error: ErrorDetail{
		Code:    appErr.Code,
		Message: appErr.Message,
	}})
}

// WriteHealth writes a health response with the given HTTP status, so a
// degraded dependency is reported as such instead of always answering 200.
func WriteHealth(w http.ResponseWriter, code int, state domain.HealthState, results []domain.ProbeResult, info SystemInfo) {
	checks := make(map[string]string, len(results))
	for _, r := range results {
		checks[r.Name] = string(r.State)
	}

	WriteJSON(w, code, HealthResponse{
		Status:    string(state),
		Checks:    checks,
		Version:   info.Version,
		Commit:    info.Commit,
		Timestamp: Timestamp(time.Now()),
	})
}

// WriteSystemInfo writes the version payload.
func WriteSystemInfo(w http.ResponseWriter, info SystemInfo) {
	WriteJSON(w, http.StatusOK, info)
}
