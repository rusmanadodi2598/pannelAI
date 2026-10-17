// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/proxy_bulk.go
// @for       The two bulk routes over proxy candidates (SPEC-API-001 §7.11).
// @uses      net/http, internal/domain, internal/schema, internal/service.
// @reason    A candidate row has no tags and belongs to no provider, so the axes an endpoint selector can narrow by are not silently droppable here: an operator who asked for "the free-tier proxies" and received the whole pool would have no way to notice. The handler refuses that narrowing and passes on the number the panel displayed, which the service checks against storage before it erases anything.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-10-16
package handler

import (
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// BulkStatus serves POST /api/v1/proxies/bulk-status (§7.11).
func (h *ProxyHandler) BulkStatus(w http.ResponseWriter, r *http.Request) {
	var req schema.BulkProxyStatusRequest
	if err := schema.DecodeAndValidate(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	sel, ok := proxySelection(w, req.Selector)
	if !ok {
		return
	}
	outcome, err := h.proxies.BulkStatus(r.Context(), sel, *req.Enabled, req.ConfirmCount)
	if err != nil {
		writeBulkRefusal(w, err, 0)
		return
	}
	writeBulkProxyOutcome(w, outcome)
}

// BulkPreview serves POST /api/v1/proxies/bulk-preview (§7.11): the same selector, and the
// answer is what the erase would cost. Nothing is written.
func (h *ProxyHandler) BulkPreview(w http.ResponseWriter, r *http.Request) {
	var req schema.BulkProxyDeleteRequest
	if err := schema.DecodeAndValidate(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	sel, ok := proxySelection(w, req.Selector)
	if !ok {
		return
	}
	outcome, err := h.proxies.BulkPreview(r.Context(), sel, req.ConfirmCount)
	if err != nil {
		writeBulkRefusal(w, err, 0)
		return
	}
	writeBulkProxyOutcome(w, outcome)
}

// BulkDelete serves POST /api/v1/proxies/bulk-delete (§7.11).
func (h *ProxyHandler) BulkDelete(w http.ResponseWriter, r *http.Request) {
	var req schema.BulkProxyDeleteRequest
	if err := schema.DecodeAndValidate(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	sel, ok := proxySelection(w, req.Selector)
	if !ok {
		return
	}
	outcome, err := h.proxies.BulkDelete(r.Context(), sel, req.ConfirmCount, req.ConfirmEndpoints)
	if err != nil {
		writeBulkRefusal(w, err, 0)
		return
	}
	writeBulkProxyOutcome(w, outcome)
}

// proxySelection narrows a bulk selector to the two axes a pool row actually has, and
// refuses the rest rather than ignoring them.
func proxySelection(w http.ResponseWriter, in schema.BulkSelector) (service.BulkProxySelection, bool) {
	parsed, err := schema.ParseBulkSelector(in)
	if err != nil {
		schema.WriteError(w, err)
		return service.BulkProxySelection{}, false
	}
	if parsed.Tag != "" || parsed.ProviderID != "" || parsed.AuthType != "" {
		schema.WriteError(w, domain.NewValidationError(
			"proxy candidates carry no tag, provider, or auth type to narrow by; select by ids or all"))
		return service.BulkProxySelection{}, false
	}
	return service.BulkProxySelection{IDs: parsed.IDs, All: parsed.All}, true
}

// writeBulkProxyOutcome answers a proxy bulk action with the pin report beside its rows,
// because on this surface the report is the part an operator needs after the fact: the
// endpoints that just lost their egress path are named whether or not they were the point.
func writeBulkProxyOutcome(w http.ResponseWriter, outcome service.BulkProxyOutcome) {
	rows := make([]schema.BulkResultRow, 0, len(outcome.Results))
	for _, result := range outcome.Results {
		row := schema.BulkResultRow{Index: result.Index, ID: result.ID}
		if result.Error != nil {
			appErr := domain.AsAppError(result.Error)
			row.Error = appErr.Message
			row.Code = appErr.Code
		}
		rows = append(rows, row)
	}
	schema.WriteJSON(w, http.StatusOK, schema.BulkProxyResponse{
		Matched: outcome.Matched,
		Changed: outcome.Changed,
		Results: rows,
		Pins: schema.ProxyPinsResponse{
			Endpoints: outcome.Pins.Endpoints,
			Providers: outcome.Pins.Providers,
		},
	})
}
