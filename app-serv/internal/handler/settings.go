// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/settings.go
// @for       The settings read and partial-patch routes (SPEC-API-001 §7.14).
// @uses      internal/schema, internal/service, net/http.
// @reason    §7.14 serves one typed document and a patch validated per key, and
//
//	secrets are never returned. AGENTS.md §1.5 keeps the merge and the
//	persistence in the service, so this handler only decodes, calls,
//	and encodes; the deprecated caveman key has no route here by
//	design (§7.9).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-18
package handler

import (
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// SettingsHandler serves the /api/v1/settings routes (§7.14).
type SettingsHandler struct {
	settings *service.SettingsService
}

// NewSettingsHandler validates deps and returns the handler.
func NewSettingsHandler(settings *service.SettingsService) *SettingsHandler {
	return &SettingsHandler{settings: settings}
}

// Get serves GET /api/v1/settings: the typed document with the stored values
// merged over the §7.14 defaults.
func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settings.Settings(r.Context())
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.SettingsResponseFrom(settings))
}

// Patch serves PATCH /api/v1/settings: a partial update, validated per key,
// applied to the stored values, and persisted for the groups it touched.
func (h *SettingsHandler) Patch(w http.ResponseWriter, r *http.Request) {
	var req schema.PatchSettingsRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidatePatch(req); err != nil {
		schema.WriteError(w, err)
		return
	}
	settings, err := h.settings.Update(r.Context(), schema.ToSettingsPatch(req))
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.SettingsResponseFrom(settings))
}
