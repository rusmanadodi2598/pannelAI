// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/endpoint_bulk_mutation.go
// @for       The six bulk mutation routes over accounts and credentials: park, retag, and delete the set one selector resolves (SPEC-API-001 §7.5, §8.1).
// @uses      net/http, internal/domain, internal/schema, internal/service.
// @reason    Every one of these routes can act on rows no browser ever showed the operator, so the handler's job is the part a client cannot be trusted with: read the selector at the boundary, hand the service the number the caller confirmed, and answer with the count the server resolved. The response keeps the {error, results} shape the batch routes already use, so a client maps one failure style across all of them.
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

// lowerBulkSelector turns a validated wire selector into the service's resolved form.
// The tag and auth type crossed the vocabulary boundary in schema, so nothing below this
// line compares raw strings (SPEC-API-001 §8.1).
func lowerBulkSelector(in schema.BulkSelection) service.BulkSelector {
	return service.BulkSelector{
		IDs:        in.IDs,
		Tag:        in.Tag,
		AuthType:   in.AuthType,
		ProviderID: in.ProviderID,
		All:        in.All,
	}
}

// readSelector resolves the selector on an already-decoded request body.
func readSelector(w http.ResponseWriter, sel schema.BulkSelector) (service.BulkSelector, bool) {
	parsed, err := schema.ParseBulkSelector(sel)
	if err != nil {
		schema.WriteError(w, err)
		return service.BulkSelector{}, false
	}
	return lowerBulkSelector(parsed), true
}

// writeBulkOutcome answers a completed bulk mutation with the server's own count of the
// set, what it changed, and each row's outcome.
//
// A row that refused is reported beside the rows that landed, and the status is still 200:
// the action the operator asked for happened to the set the server resolved, and the rows
// that could not take it are the detail, not a failed request.
func writeBulkOutcome(w http.ResponseWriter, outcome service.BulkOutcome) {
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
	schema.WriteJSON(w, http.StatusOK, schema.BulkMutationResponse{
		Matched: outcome.Matched,
		Changed: outcome.Changed,
		Results: rows,
	})
}

// BulkStatus serves POST /api/v1/endpoints/bulk-status (§7.5).
func (h *EndpointBulkHandler) BulkStatus(w http.ResponseWriter, r *http.Request) {
	var req schema.BulkStatusRequest
	if err := schema.DecodeAndValidate(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	sel, ok := readSelector(w, req.Selector)
	if !ok {
		return
	}
	outcome, err := h.endpoints.BulkStatus(r.Context(), sel, req.Status, req.WindowHours, req.ConfirmCount)
	if err != nil {
		writeBulkRefusal(w, err, 0)
		return
	}
	writeBulkOutcome(w, outcome)
}

// BulkDelete serves POST /api/v1/endpoints/bulk-delete (§7.5).
func (h *EndpointBulkHandler) BulkDelete(w http.ResponseWriter, r *http.Request) {
	var req schema.BulkDeleteRequest
	if err := schema.DecodeAndValidate(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	sel, ok := readSelector(w, req.Selector)
	if !ok {
		return
	}
	outcome, err := h.endpoints.BulkDelete(r.Context(), sel, req.ConfirmCount)
	if err != nil {
		writeBulkRefusal(w, err, 0)
		return
	}
	writeBulkOutcome(w, outcome)
}

// BulkTags serves POST /api/v1/endpoints/bulk-tags (§7.5).
func (h *EndpointBulkHandler) BulkTags(w http.ResponseWriter, r *http.Request) {
	var req schema.BulkTagsRequest
	if err := schema.DecodeAndValidate(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	sel, ok := readSelector(w, req.Selector)
	if !ok {
		return
	}
	outcome, err := h.endpoints.BulkTags(r.Context(), sel, req.Add, req.Remove, req.Clear, req.ConfirmCount)
	if err != nil {
		writeBulkRefusal(w, err, 0)
		return
	}
	writeBulkOutcome(w, outcome)
}

// BulkKeyStatus serves POST /api/v1/endpoint-keys/bulk-status (§7.5).
func (h *EndpointBulkHandler) BulkKeyStatus(w http.ResponseWriter, r *http.Request) {
	var req schema.BulkStatusRequest
	if err := schema.DecodeAndValidate(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	sel, ok := readSelector(w, req.Selector)
	if !ok {
		return
	}
	outcome, err := h.endpoints.BulkKeyStatus(r.Context(), sel, req.Status, req.WindowHours, req.ConfirmCount)
	if err != nil {
		writeBulkRefusal(w, err, 0)
		return
	}
	writeBulkOutcome(w, outcome)
}

// BulkKeyDelete serves POST /api/v1/endpoint-keys/bulk-delete (§7.5).
func (h *EndpointBulkHandler) BulkKeyDelete(w http.ResponseWriter, r *http.Request) {
	var req schema.BulkDeleteRequest
	if err := schema.DecodeAndValidate(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	sel, ok := readSelector(w, req.Selector)
	if !ok {
		return
	}
	outcome, err := h.endpoints.BulkKeyDelete(r.Context(), sel, req.ConfirmCount)
	if err != nil {
		writeBulkRefusal(w, err, 0)
		return
	}
	writeBulkOutcome(w, outcome)
}

// BulkKeyTags serves POST /api/v1/endpoint-keys/bulk-tags (§7.5).
func (h *EndpointBulkHandler) BulkKeyTags(w http.ResponseWriter, r *http.Request) {
	var req schema.BulkTagsRequest
	if err := schema.DecodeAndValidate(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	sel, ok := readSelector(w, req.Selector)
	if !ok {
		return
	}
	outcome, err := h.endpoints.BulkKeyTags(r.Context(), sel, req.Add, req.Remove, req.Clear, req.ConfirmCount)
	if err != nil {
		writeBulkRefusal(w, err, 0)
		return
	}
	writeBulkOutcome(w, outcome)
}
