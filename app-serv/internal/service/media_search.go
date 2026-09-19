// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_search.go
// @for       The web search use case of SPEC-API-001 §7.10.
// @uses      internal/dataplane, internal/domain, internal/registry, internal/schema,
//
//	context, encoding/json, strconv, strings, time.
//
// @reason    Search is the one media kind whose request shape is not fixed: the
//
//	reference carries a per-provider builder because search APIs
//	disagree on the spelling of `q`/`query` and `num`/`count`. The
//	registry declares those names, so the payload is built from the
//	block rather than from a provider switch here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// Search performs one web search call and normalizes the answer.
//
// The provider may be named directly, or through a model string whose first
// segment is one: the reference accepts either, and refusing the direct form
// would make a caller name a model the search route never uses.
func (s *MediaCallService) Search(ctx context.Context, req schema.SearchRequest, keyID string) (schema.SearchResponse, dataplane.Outcome, error) {
	providerID, err := searchProvider(req)
	if err != nil {
		// The request named no provider, so the row is named by nothing.
		s.recorder.refuse(ctx, dataplane.Outcome{}, keyID, err)
		return schema.SearchResponse{}, dataplane.Outcome{}, err
	}
	block, err := s.Block(providerID, domain.MediaKindSearch)
	if err != nil {
		s.recorder.refuse(ctx, mediaRefusalOutcome(providerID, ""), keyID, err)
		return schema.SearchResponse{}, dataplane.Outcome{}, err
	}

	queryParam := declaredParam(block.QueryParam, "query")
	maxParam := declaredParam(block.MaxResultsParam, "max_results")
	maxResults := maxResultsFor(block, req.MaxResults)

	started := time.Now()
	call, err := s.prepareForCall(ctx, providerID+"/", domain.MediaKindSearch, searchQuery(block, queryParam, maxParam, req.Query, maxResults), keyID)
	if err != nil {
		return schema.SearchResponse{}, dataplane.Outcome{}, err
	}
	request := dataplane.MediaRequest{Method: httpMethod(block)}
	if request.Method != "GET" {
		body, err := json.Marshal(searchBody{
			queryParam: queryParam, query: req.Query, maxParam: maxParam, maxResults: maxResults,
		})
		if err != nil {
			return schema.SearchResponse{}, call.Outcome(),
				dataplane.InternalError("the search request could not be built", err)
		}
		request.Body = body
	}

	answer, err := s.Perform(ctx, call, request, keyID, nil)
	if err != nil {
		return schema.SearchResponse{}, call.Outcome(), err
	}
	results, err := normalizeSearchResults(answer.Body)
	if err != nil {
		return schema.SearchResponse{}, call.Outcome(), err
	}
	return schema.SearchResponse{
		Provider: call.ProviderID,
		Query:    req.Query,
		Results:  results,
		Usage:    schema.SearchUsage{QueriesUsed: 1, SearchCostUSD: block.CostPerQuery},
		Metrics: schema.SearchMetrics{
			ResponseTimeMS: time.Since(started).Milliseconds(),
			// The reference reads a provider-reported total where one exists;
			// until that is ported the count of what this answer carried is
			// what the field can honestly report.
			TotalResultsAvailable: len(results),
		},
	}, call.Outcome(), nil
}

// searchProvider resolves the provider a search request names, from `provider`
// or from the model string's first segment.
func searchProvider(req schema.SearchRequest) (string, error) {
	if provider := strings.TrimSpace(req.Provider); provider != "" {
		return provider, nil
	}
	if strings.TrimSpace(req.Model) == "" {
		return "", dataplane.ValidationError("provider (or model) is required")
	}
	providerID, _, err := splitMediaModel(req.Model)
	if err != nil {
		return "", err
	}
	return providerID, nil
}

// searchQuery builds the URL parameters a GET provider reads. A POST provider
// reads them from its body, so its target carries none.
func searchQuery(media registry.MediaConfig, queryParam, maxParam, query string, maxResults int) map[string]string {
	if httpMethod(media) != "GET" {
		return nil
	}
	return map[string]string{queryParam: query, maxParam: strconv.Itoa(maxResults)}
}

// searchBody is the JSON body a POST search provider expects. Its field names
// come from the registry, which a struct tag cannot express, so the type
// marshals itself — the one place a map is used, and it is closed by the
// constructor above rather than assembled at a call site (AGENTS.md §1.4).
type searchBody struct {
	queryParam string
	query      string
	maxParam   string
	maxResults int
}

// MarshalJSON writes the two declared fields under their declared names. The
// values marshal to their own JSON types first, then ride as RawMessage so the
// dynamic keys need no untyped value (AGENTS.md §1.4): a search API reads a
// quoted string for the query and a bare number for the limit.
func (b searchBody) MarshalJSON() ([]byte, error) {
	query, err := json.Marshal(b.query)
	if err != nil {
		return nil, err
	}
	max, err := json.Marshal(b.maxResults)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]json.RawMessage{b.queryParam: query, b.maxParam: max})
}

// searchEnvelope and the result reading live in media_search_result.go: the
// spellings belong together, and this file is the request side of the call.

// declaredParam reports a declared parameter name, or the default when the
// registry declares none.
func declaredParam(declared, fallback string) string {
	if trimmed := strings.TrimSpace(declared); trimmed != "" {
		return trimmed
	}
	return fallback
}

// maxResultsFor applies the block's default and its ceiling to the caller's
// request, the way the reference clamps `max_results` against `maxMaxResults`.
func maxResultsFor(media registry.MediaConfig, requested *int) int {
	limit := media.DefaultMaxResults
	if limit < 1 {
		limit = 5
	}
	if requested != nil && *requested > 0 {
		limit = *requested
	}
	if media.MaxMaxResults > 0 && limit > media.MaxMaxResults {
		limit = media.MaxMaxResults
	}
	return limit
}

// httpMethod reports the block's declared method, defaulting to POST.
func httpMethod(media registry.MediaConfig) string {
	if method := strings.ToUpper(strings.TrimSpace(media.Method)); method != "" {
		return method
	}
	return "POST"
}
