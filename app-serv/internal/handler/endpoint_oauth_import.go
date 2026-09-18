// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/endpoint_oauth_import.go
// @for       Importing already-obtained OAuth credentials as endpoints
//
//	(SPEC-API-001 §7.5, §8.1).
//
// @uses      internal/domain, internal/schema, internal/service, net/http, time.
// @reason    The import path exists for accounts obtained on a machine with no
//
//	browser callback, so there is no authorization flow to run — only
//	tokens to seal and an account identity to match. Its response is the
//	same per-row report the other batches use, so it shares this handler,
//	and its own file keeps the AGENTS.md §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-17
package handler

import (
	"net/http"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// ImportOAuth serves POST /api/v1/providers/{provider_id}/oauth/bulk (§7.5). It
// imports credentials obtained elsewhere, so there is no authorization flow here —
// only sealing and account matching (§8.1).
func (h *EndpointHandler) ImportOAuth(w http.ResponseWriter, r *http.Request) {
	providerID, ok := pathValue(w, r, "provider_id")
	if !ok {
		return
	}
	var req schema.BulkOAuthImportRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return
	}

	accounts, err := toOAuthAccounts(req.Accounts)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	results, err := h.endpoints.BulkImportOAuth(r.Context(), providerID, accounts)
	if err != nil {
		writeBulkRefusal(w, err, len(accounts))
		return
	}

	resp := schema.BulkOAuthResponse{
		Created: make([]schema.BulkOAuthRow, 0, len(results)),
		Results: make([]schema.BulkResultRow, 0, len(results)),
	}
	for _, result := range results {
		endpoint := result.Endpoint
		resp.Created = append(resp.Created, schema.BulkOAuthRow{
			Index:     result.Index,
			ID:        endpoint.ID(),
			Label:     endpoint.Label(),
			Account:   toAccountResponse(endpoint.Account()),
			TokenHint: result.TokenHint,
			Updated:   result.Updated,
		})
		resp.Results = append(resp.Results, schema.BulkResultRow{Index: result.Index, ID: endpoint.ID()})
	}
	schema.WriteJSON(w, http.StatusCreated, resp)
}

// toOAuthAccounts maps decoded import rows onto the service's input type, parsing the
// optional expiry the wire carries as an RFC3339 string.
func toOAuthAccounts(rows []schema.BulkOAuthAccountInput) ([]service.OAuthAccountInput, error) {
	out := make([]service.OAuthAccountInput, 0, len(rows))
	for _, row := range rows {
		account := service.OAuthAccountInput{
			Label:        row.Label,
			AccessToken:  row.AccessToken,
			RefreshToken: row.RefreshToken,
			Scopes:       row.Scopes,
		}
		if row.Account != nil {
			account.Account = domain.EndpointAccount{
				Name:        row.Account.Name,
				Email:       row.Account.Email,
				MachineID:   row.Account.MachineID,
				WorkspaceID: row.Account.WorkspaceID,
			}
		}
		if row.ExpiresAt != nil && *row.ExpiresAt != "" {
			parsed, err := time.Parse(time.RFC3339, *row.ExpiresAt)
			if err != nil {
				return nil, domain.NewValidationError("expires_at must be an RFC3339 timestamp")
			}
			account.ExpiresAt = &parsed
		}
		out = append(out, account)
	}
	return out, nil
}
