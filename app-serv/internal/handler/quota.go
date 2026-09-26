// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/quota.go
// @for       The quota window reads and the budget-cap write (SPEC-API-001
//
//	§7.12).
//
// @uses      internal/domain, internal/schema, internal/service, net/http.
// @reason    §7.12 exposes every endpoint's windows, one endpoint's windows, and
//
//	the cap that makes the router skip an exhausted endpoint. The cap
//	body is validated before the service sees it (AGENTS.md §2.4), and
//	the cost is a decimal string on the wire, never a float (§4).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-18
package handler

import (
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// QuotaHandler serves the /api/v1/quotas routes (§7.12).
type QuotaHandler struct {
	quotas *service.QuotaService
}

// NewQuotaHandler validates deps and returns the handler.
func NewQuotaHandler(quotas *service.QuotaService) *QuotaHandler {
	return &QuotaHandler{quotas: quotas}
}

// List serves GET /api/v1/quotas: the collection read, paged over provider
// groups (docs/PORT/006-PORT-QUOTA-PAGING.md D1). The page unit is the provider
// group, so one page carries whole cards; per_page counts groups on this route,
// and the meta block reports the total group count. The params go through the
// house decoder, so an out-of-range value is a refusal rather than a clamp
// (SPEC-API-001 §4, draft 010 F6).
func (h *QuotaHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage, err := schema.DecodePage(r)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	windows, total, err := h.quotas.ListWindowsPaged(r.Context(), page, perPage)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.QuotaWindowList{
		Data: windowResponses(windows),
		Meta: schema.Page{Page: page, PerPage: perPage, Total: total},
	})
}

// Get serves GET /api/v1/quotas/{endpoint_id}: one endpoint's windows plus the
// stored budget cap, so a cap that was written can be read back even when no
// usage window exists yet (SPEC-API-001 §7.12, owner decision D5 = b). The
// path value cannot be empty through the route (a Go 1.22 wildcard matches a
// full segment), so there is no empty-id guard here: the service owns that
// check for the paths that can reach it.
func (h *QuotaHandler) Get(w http.ResponseWriter, r *http.Request) {
	endpointID := r.PathValue("endpoint_id")
	windows, err := h.quotas.ListWindows(r.Context(), endpointID)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	cap, stored, err := h.quotas.GetCap(r.Context(), endpointID)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	resp := schema.QuotaEndpointDetail{
		EndpointID: endpointID,
		Data:       windowResponses(windows),
	}
	if stored {
		mapped := schema.QuotaCapResponseFrom(cap)
		resp.Cap = &mapped
	}
	schema.WriteJSON(w, http.StatusOK, resp)
}

// PutCap serves PUT /api/v1/quotas/{endpoint_id}: it replaces the cap set for
// one endpoint and returns the stored value. An endpoint the gateway does not
// know is refused by the service with NOT_FOUND (draft 005 F2).
func (h *QuotaHandler) PutCap(w http.ResponseWriter, r *http.Request) {
	endpointID := r.PathValue("endpoint_id")
	var req schema.QuotaCapRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidateQuotaCap(req); err != nil {
		schema.WriteError(w, err)
		return
	}

	cost, err := parseOptionalCost(req.MonthlyCostUSD)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	cap, err := h.quotas.SetCap(r.Context(), endpointID, cost, req.MonthlyTokens)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.QuotaCapResponseFrom(cap))
}

// windowResponses maps stored windows onto their wire shape, always as a
// non-nil slice so an endpoint with no windows answers an empty array rather
// than a null.
func windowResponses(windows []domain.QuotaWindow) []schema.QuotaWindowResponse {
	resp := make([]schema.QuotaWindowResponse, 0, len(windows))
	for _, window := range windows {
		resp = append(resp, schema.QuotaWindowResponseFrom(window))
	}
	return resp
}

// parseOptionalCost lowers an optional decimal string to the value object,
// keeping the parse at the boundary so the service receives a typed amount
// (AGENTS.md §1.4).
func parseOptionalCost(raw *string) (*domain.Decimal, error) {
	if raw == nil {
		return nil, nil
	}
	parsed, err := domain.ParseDecimal(*raw)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
