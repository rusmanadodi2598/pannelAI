// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/endpoint_bulk.go
// @for       The endpoint-level batch routes: several accounts in one call, several
//
//	keys on one endpoint, and the OAuth credential import
//	(SPEC-API-001 §7.5, §8.1).
//
// @uses      internal/domain, internal/schema, internal/service, errors, net/http,
//
//	time.
//
// @reason    §8.1 makes a batch all-or-nothing but still requires every row's outcome
//
//	to be reported by index, so a refusal has to answer with both a machine
//	code and the offending row — a bare §8 envelope could not carry the row
//	and a bare body could not carry the code. The three batch routes share
//	that rule, which is why they share one handler and one refusal writer.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-17
package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// EndpointBulkHandler serves the batch onboarding routes (§7.5).
type EndpointBulkHandler struct {
	endpoints *service.EndpointService
}

// NewEndpointBulkHandler validates deps and returns the handler.
func NewEndpointBulkHandler(endpoints *service.EndpointService) *EndpointBulkHandler {
	return &EndpointBulkHandler{endpoints: endpoints}
}

// rowIndexer is the structural surface a store or service error exposes when it can
// name the batch row it refused, so this layer attributes a row without importing
// either layer's error type (AGENTS.md §1.5).
type rowIndexer interface {
	// BulkRowIndex reports the offending row's zero-based position, or false when
	// the error cannot be attributed to one row.
	BulkRowIndex() (int, bool)
}

// BulkCreate serves POST /api/v1/endpoints/bulk (§7.5). Nothing is written unless the
// whole batch passes (§8.1).
func (h *EndpointBulkHandler) BulkCreate(w http.ResponseWriter, r *http.Request) {
	var req schema.BulkCreateEndpointsRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return
	}
	authType, err := schema.ParseAuthType(req.AuthType)
	if err != nil {
		schema.WriteError(w, err)
		return
	}

	accounts := make([]service.BulkAccountInput, 0, len(req.Endpoints))
	for _, item := range req.Endpoints {
		accounts = append(accounts, service.BulkAccountInput{
			Label:    item.Label,
			Priority: item.Priority,
			Keys:     toKeyInputs(item.Keys),
		})
	}
	created, err := h.endpoints.BulkCreateAccounts(r.Context(), req.ProviderID, authType, accounts)
	if err != nil {
		writeBulkRefusal(w, err, len(accounts))
		return
	}
	h.writeCreatedEndpoints(w, created)
}

// AddKeysBulk serves POST /api/v1/endpoints/{id}/keys/bulk (§7.5). A duplicate label
// inside the batch is a VALIDATION_ERROR, and nothing is written unless the whole
// batch passes.
func (h *EndpointBulkHandler) AddKeysBulk(w http.ResponseWriter, r *http.Request) {
	id, ok := pathValue(w, r, "id")
	if !ok {
		return
	}
	var req schema.BulkAddKeysRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return
	}
	added, err := h.endpoints.AddKeyBatch(r.Context(), id, toKeyInputs(req.Keys))
	if err != nil {
		writeBulkRefusal(w, err, len(req.Keys))
		return
	}

	now := time.Now()
	resp := schema.BulkKeyResponse{
		Created: make([]schema.EndpointKeyResponse, 0, len(added)),
		Results: make([]schema.BulkResultRow, 0, len(added)),
	}
	for i, key := range added {
		resp.Created = append(resp.Created, toKeyResponse(key, now))
		resp.Results = append(resp.Results, schema.BulkResultRow{Index: i, ID: key.ID()})
	}
	schema.WriteJSON(w, http.StatusCreated, resp)
}

// writeCreatedEndpoints answers a successful account batch with every row's index and
// its created endpoint.
func (h *EndpointBulkHandler) writeCreatedEndpoints(w http.ResponseWriter, created []domain.UpstreamEndpoint) {
	now := time.Now()
	resp := schema.BulkEndpointResponse{
		Created: make([]schema.EndpointResponse, 0, len(created)),
		Results: make([]schema.BulkResultRow, 0, len(created)),
	}
	for i, endpoint := range created {
		resp.Created = append(resp.Created, toEndpointResponse(endpoint, now, true))
		resp.Results = append(resp.Results, schema.BulkResultRow{Index: i, ID: endpoint.ID()})
	}
	schema.WriteJSON(w, http.StatusCreated, resp)
}

// writeBulkRefusal answers a refused batch with the §8 code, plus the offending row's
// index when the refusal names one.
//
// Every row is reported, not only the offending one: §8.1 says nothing is written
// unless all rows pass, so a client shown only the bad row would imply the others had
// been stored. When the refusal cannot be pinned to a row, the row list is empty and
// the code and message carry it — repeating the message on every row would read as N
// failures where only the batch failed.
func writeBulkRefusal(w http.ResponseWriter, err error, rowCount int) {
	appErr := domain.AsAppError(err)
	index, attributed := bulkRowIndex(err)

	var results []schema.BulkResultRow
	if attributed {
		results = make([]schema.BulkResultRow, 0, rowCount)
		for i := range rowCount {
			row := schema.BulkResultRow{Index: i}
			if i == index {
				row.Error = appErr.Message
			}
			results = append(results, row)
		}
	}
	schema.WriteJSON(w, appErr.HTTPStatus(), schema.BulkRefusal{
		Error:   schema.BulkError{Code: appErr.Code, Message: appErr.Message},
		Results: results,
	})
}

// bulkRowIndex reports which batch row a refusal names, when it names one.
func bulkRowIndex(err error) (int, bool) {
	var indexed rowIndexer
	if errors.As(err, &indexed) {
		return indexed.BulkRowIndex()
	}
	return -1, false
}
