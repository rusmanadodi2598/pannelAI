// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/media_search.go
// @for       The web search contract of SPEC-API-001 §7.10.
// @uses      encoding/json, internal/domain.
// @reason    §7.10's search route takes a query and answers with the reference's
//
//	normalized envelope. The request accepts `provider` or `model`
//	because the reference does: a caller who knows the search provider
//	names it, and one who names a model has its provider resolved from
//	the same string every other media route uses.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-19
package schema

import (
	"encoding/json"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// SearchRequest is the body of POST /api/v1/search.
type SearchRequest struct {
	Provider   string `json:"provider,omitempty" validate:"omitempty,max=64"`
	Model      string `json:"model,omitempty" validate:"omitempty,max=200"`
	Query      string `json:"query" validate:"required,min=1,max=400"`
	MaxResults *int   `json:"max_results,omitempty" validate:"omitempty,gte=1"`
	SearchType string `json:"search_type,omitempty" validate:"omitempty,max=32"`
	Country    string `json:"country,omitempty" validate:"omitempty,max=8"`
	Language   string `json:"language,omitempty" validate:"omitempty,max=8"`
}

// SearchResponse is the normalized search answer (SPEC-API-001 §7.10).
type SearchResponse struct {
	Provider string         `json:"provider"`
	Query    string         `json:"query"`
	Results  []SearchResult `json:"results"`
	Usage    SearchUsage    `json:"usage"`
	Metrics  SearchMetrics  `json:"metrics"`
}

// SearchResult is one result. The reference's item carries more metadata
// (score, published_at, content, citation); this is the subset every one of its
// providers fills in, and the rest is added when a consumer needs it.
type SearchResult struct {
	Title    string `json:"title"`
	URL      string `json:"url"`
	Snippet  string `json:"snippet,omitempty"`
	Position int    `json:"position"`
}

// SearchUsage reports what the search cost, in the reference's shape.
type SearchUsage struct {
	QueriesUsed   int     `json:"queries_used"`
	SearchCostUSD float64 `json:"search_cost_usd"`
}

// SearchMetrics reports how the search performed.
type SearchMetrics struct {
	ResponseTimeMS        int64 `json:"response_time_ms"`
	TotalResultsAvailable int   `json:"total_results_available"`
}

// DecodeSearchRequest decodes a search body into its typed contract.
func DecodeSearchRequest(raw []byte) (SearchRequest, error) {
	var req SearchRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return SearchRequest{}, domain.NewValidationError("invalid request body: " + jsonErrorTail(err))
	}
	return req, nil
}
