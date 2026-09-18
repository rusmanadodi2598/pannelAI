// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/log.go
// @for       The request-log list, detail, purge, and console routes
//
//	(SPEC-API-001 §7.13).
//
// @uses      internal/domain, internal/schema, internal/service, net/http.
// @reason    §7.13 lists logs newest first, serves the captured bodies only when
//
//	capture is on, purges by retention, and exposes a bounded console
//	buffer. AGENTS.md §1.5 keeps SQL and Redis out of this layer, so
//	every route here decodes, calls the service, and encodes.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-18
package handler

import (
	"net/http"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// LogHandler serves the /api/v1/logs routes (§7.13).
type LogHandler struct {
	logs  *service.LogService
	clock func() time.Time
}

// NewLogHandler validates deps and returns the handler.
func NewLogHandler(logs *service.LogService) *LogHandler {
	return &LogHandler{logs: logs, clock: time.Now}
}

// Requests serves GET /api/v1/logs/requests, paginated and always bounded.
func (h *LogHandler) Requests(w http.ResponseWriter, r *http.Request) {
	query, err := schema.DecodeLogFilter(r)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	page, perPage, err := schema.DecodePage(r)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	entries, total, err := h.logs.Requests(r.Context(), query.Filter(h.clock()), page, perPage)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	resp := schema.LogList{
		Data: make([]schema.LogRecordResponse, 0, len(entries)),
		Meta: schema.Page{Page: page, PerPage: perPage, Total: total},
	}
	for _, entry := range entries {
		resp.Data = append(resp.Data, schema.LogRecordResponseFrom(entry))
	}
	schema.WriteJSON(w, http.StatusOK, resp)
}

// Detail serves GET /api/v1/logs/requests/{request_id} with the captured bodies
// when capture is on.
func (h *LogHandler) Detail(w http.ResponseWriter, r *http.Request) {
	requestID := r.PathValue("request_id")
	if requestID == "" {
		schema.WriteError(w, domain.NewValidationError("request_id is required"))
		return
	}
	entry, captureEnabled, maxBytes, err := h.logs.Request(r.Context(), requestID)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.LogDetailResponseFrom(entry, captureEnabled, maxBytes))
}

// Purge serves DELETE /api/v1/logs/requests, deleting rows older than the
// retention setting and reporting how many were removed.
func (h *LogHandler) Purge(w http.ResponseWriter, r *http.Request) {
	deleted, err := h.logs.Purge(r.Context())
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.LogPurgeResponse{Deleted: deleted})
}

// Console serves GET /api/v1/logs/console: the bounded ring, oldest line first.
func (h *LogHandler) Console(w http.ResponseWriter, r *http.Request) {
	lines, maxRecords, err := h.logs.Console(r.Context())
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.ConsoleResponseFrom(lines, maxRecords))
}

// ClearConsole serves DELETE /api/v1/logs/console, emptying the buffer
// server-side so every panel sees the same empty console.
func (h *LogHandler) ClearConsole(w http.ResponseWriter, r *http.Request) {
	if err := h.logs.ClearConsole(r.Context()); err != nil {
		schema.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
