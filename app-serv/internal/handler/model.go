// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/model.go
// @for       The model catalog endpoints: the merged catalog, custom models, the
//
//	alias set, and the disabled set (SPEC-API-001 §7.6).
//
// @uses      internal/schema, internal/service, internal/domain, net/http.
// @reason    §7.6 is four routes over one screen, and every one of them is
//
//	decode → validate → call → encode. Keeping them in one file makes
//	the set auditable against the spec table; AGENTS.md §1.5 keeps the
//	SQL and the merge out of here, so no branch below inspects a row.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-17
package handler

import (
	"net/http"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// ModelHandler serves the /api/v1/models routes (§7.6).
type ModelHandler struct {
	catalog *service.ModelCatalogService
}

// NewModelHandler validates deps and returns the handler.
func NewModelHandler(catalog *service.ModelCatalogService) *ModelHandler {
	return &ModelHandler{catalog: catalog}
}

// Catalog serves GET /api/v1/models/catalog with the §7.6 filters, including
// the `active` boolean (draft 025): `true` narrows the answer to providers
// holding an active endpoint, `false` narrows nothing, and any other spelling
// is a VALIDATION_ERROR rather than a silently ignored filter.
func (h *ModelHandler) Catalog(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	active, err := decodeBoolFilter(query.Get("active"))
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	filter := service.CatalogFilter{
		ProviderID: strings.TrimSpace(query.Get("provider_id")),
		Capability: strings.TrimSpace(query.Get("capability")),
		Query:      strings.TrimSpace(query.Get("q")),
		Active:     active,
	}
	models, err := h.catalog.Catalog(r.Context(), filter)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.ModelCatalogResponse{Data: schema.ToModelResponses(models)})
}

// decodeBoolFilter reads one optional boolean query parameter. The vocabulary
// is the two lowercase spellings; an empty value reads as absent, which is the
// house rule every other query parameter follows, and a misspelling like `yes`
// or `1` is refused rather than silently narrowing nothing while reading as
// narrowed (draft 025 F5 — the same failure the usage status filter had before
// it became a closed set).
func decodeBoolFilter(raw string) (*bool, error) {
	trimmed := strings.TrimSpace(raw)
	switch trimmed {
	case "":
		return nil, nil
	case "true":
		value := true
		return &value, nil
	case "false":
		value := false
		return &value, nil
	default:
		return nil, domain.NewValidationError("active must be true or false")
	}
}

// CustomList serves GET /api/v1/models/custom, narrowed by `?provider_id=`
// when the parameter is present. The narrowing happens in the service, so the
// alias and prefix spellings are accepted here exactly as the catalog accepts
// them.
func (h *ModelHandler) CustomList(w http.ResponseWriter, r *http.Request) {
	models, err := h.catalog.Custom(r.Context(), strings.TrimSpace(r.URL.Query().Get("provider_id")))
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	responses := make([]schema.CustomModelResponse, 0, len(models))
	for _, model := range models {
		responses = append(responses, schema.ToCustomModelResponse(model))
	}
	schema.WriteJSON(w, http.StatusOK, schema.CustomModelList{Data: responses})
}

// CustomCreate serves POST /api/v1/models/custom.
func (h *ModelHandler) CustomCreate(w http.ResponseWriter, r *http.Request) {
	var req schema.CreateCustomModelRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return
	}
	model, err := h.catalog.AddCustom(r.Context(), req.ProviderID, req.ModelID, req.DisplayName,
		domain.NewModelCapabilities(req.Capabilities...))
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusCreated, schema.ToCustomModelResponse(model))
}

// CustomDelete serves DELETE /api/v1/models/custom/{id}.
func (h *ModelHandler) CustomDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		schema.WriteError(w, domain.NewValidationError("id is required"))
		return
	}
	if err := h.catalog.RemoveCustom(r.Context(), id); err != nil {
		schema.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AliasesGet serves GET /api/v1/models/aliases.
func (h *ModelHandler) AliasesGet(w http.ResponseWriter, r *http.Request) {
	aliases, err := h.catalog.Aliases(r.Context())
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.AliasList{Data: schema.ToAliasResponses(aliases)})
}

// AliasesPut serves PUT /api/v1/models/aliases, replacing the whole set.
func (h *ModelHandler) AliasesPut(w http.ResponseWriter, r *http.Request) {
	var req schema.ReplaceAliasesRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return
	}
	aliases, err := schema.ToModelAliases(req.Aliases)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := h.catalog.ReplaceAliases(r.Context(), aliases); err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.AliasList{Data: schema.ToAliasResponses(aliases)})
}

// DisabledGet serves GET /api/v1/models/disabled.
func (h *ModelHandler) DisabledGet(w http.ResponseWriter, r *http.Request) {
	refs, err := h.catalog.Disabled(r.Context())
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.DisabledList{Data: schema.ToDisabledResponses(refs)})
}

// DisabledPut serves PUT /api/v1/models/disabled, replacing the whole set.
func (h *ModelHandler) DisabledPut(w http.ResponseWriter, r *http.Request) {
	var req schema.ReplaceDisabledRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return
	}
	refs, err := schema.ToModelRefs(req.Models)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := h.catalog.ReplaceDisabled(r.Context(), refs); err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.DisabledList{Data: schema.ToDisabledResponses(refs)})
}
