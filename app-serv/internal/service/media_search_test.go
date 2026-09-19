// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_search_test.go
// @for       The web search payload and result normalization of §7.10.
// @uses      internal/dataplane, internal/domain, internal/schema, context,
//
//	encoding/json, net/url, strings, testing.
//
// @reason    Search is the kind whose request shape comes from the registry, so
//
//	the cases pin that the declared parameter names are the ones sent
//	and that every list location the reference's providers answer with
//	normalizes into the same result items.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestMediaCallService_SearchRequest pins the provider resolution and the
// declared parameter names.
func TestMediaCallService_SearchRequest(t *testing.T) {
	cases := []struct {
		name          string
		req           schema.SearchRequest
		wantQuery     string
		wantMax       string
		wantProvider  string
		wantErrorText string
	}{
		{
			name: "the declared query and count names", wantProvider: "brave-search",
			req:       schema.SearchRequest{Provider: "brave-search", Query: "golang generics", MaxResults: intPtr(3)},
			wantQuery: "golang generics", wantMax: "3",
		},
		{
			name: "a model names the provider", wantProvider: "brave-search",
			req:       schema.SearchRequest{Model: "brave-search/web", Query: "golang generics"},
			wantQuery: "golang generics", wantMax: "5",
		},
		{
			name: "the provider's ceiling clamps the request", wantProvider: "brave-search",
			req:       schema.SearchRequest{Provider: "brave-search", Query: "golang generics", MaxResults: intPtr(500)},
			wantQuery: "golang generics", wantMax: "20",
		},
		{
			name: "neither provider nor model", req: schema.SearchRequest{Query: "golang generics"},
			wantErrorText: "provider (or model) is required",
		},
		{
			name:          "a provider with no search service",
			req:           schema.SearchRequest{Provider: "openai", Query: "golang generics"},
			wantErrorText: "does not offer search",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, caller, _ := mediaCallFixture(t)
			caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(
				`{"web":{"results":[{"title":"Go","url":"https://go.dev","description":"the language"}]}}`)}

			response, _, err := svc.Search(context.Background(), tc.req)
			if tc.wantErrorText != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrorText) {
					t.Fatalf("error = %v, want it to mention %q", err, tc.wantErrorText)
				}
				return
			}
			if err != nil {
				t.Fatalf("Search() error = %v", err)
			}
			parsed, err := url.Parse(caller.requests[0].URL)
			if err != nil {
				t.Fatalf("parsing the target: %v", err)
			}
			if got := parsed.Query().Get("q"); got != tc.wantQuery {
				t.Fatalf("q = %q, want %q (target %s)", got, tc.wantQuery, caller.requests[0].URL)
			}
			if got := parsed.Query().Get("count"); got != tc.wantMax {
				t.Fatalf("count = %q, want %q (target %s)", got, tc.wantMax, caller.requests[0].URL)
			}
			if response.Provider != tc.wantProvider {
				t.Fatalf("provider = %q, want %q", response.Provider, tc.wantProvider)
			}
			if caller.requests[0].Method != "GET" {
				t.Fatalf("method = %q, want the declared GET", caller.requests[0].Method)
			}
		})
	}
}

// TestMediaCallService_SearchNormalizesResults pins every list location the
// reference's providers answer with, and the field spellings they use.
func TestMediaCallService_SearchNormalizesResults(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantURL string
		wantSn  string
	}{
		{
			name:    "a results list",
			body:    `{"results":[{"title":"Tavily","url":"https://t.example.com","content":"snip"}]}`,
			wantURL: "https://t.example.com", wantSn: "snip",
		},
		{
			name:    "serper's organic list with a link field",
			body:    `{"organic":[{"title":"Serper","link":"https://s.example.com","snippet":"snip"}]}`,
			wantURL: "https://s.example.com", wantSn: "snip",
		},
		{
			name:    "brave's nested web list",
			body:    `{"web":{"results":[{"title":"Brave","url":"https://b.example.com","description":"snip"}]}}`,
			wantURL: "https://b.example.com", wantSn: "snip",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, caller, _ := mediaCallFixture(t)
			caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(tc.body)}

			response, _, err := svc.Search(context.Background(), schema.SearchRequest{
				Provider: "brave-search", Query: "golang generics",
			})
			if err != nil {
				t.Fatalf("Search() error = %v", err)
			}
			if len(response.Results) != 1 {
				t.Fatalf("results = %+v, want one", response.Results)
			}
			result := response.Results[0]
			if result.URL != tc.wantURL || result.Snippet != tc.wantSn || result.Position != 1 {
				t.Fatalf("result = %+v, want %s with snippet %q at position 1", result, tc.wantURL, tc.wantSn)
			}
			if response.Usage.QueriesUsed != 1 || response.Usage.SearchCostUSD != 0.005 {
				t.Fatalf("usage = %+v, want one query at the declared cost", response.Usage)
			}
		})
	}
}

// TestMediaCallService_SearchRefusesANonJSONAnswer pins that an unreadable
// answer is a gateway failure rather than an empty result list a client would
// read as "no matches".
func TestMediaCallService_SearchRefusesANonJSONAnswer(t *testing.T) {
	svc, caller, _ := mediaCallFixture(t)
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte("upstream says no")}

	_, _, err := svc.Search(context.Background(), schema.SearchRequest{Provider: "brave-search", Query: "golang"})
	if err == nil || !strings.Contains(err.Error(), "could not be read") {
		t.Fatalf("error = %v, want the unreadable-answer refusal", err)
	}
}

// TestSearchBodyMarshalsDeclaredNames pins the POST payload: the field names
// come from the declaration, so a provider reading `query` is not sent `q`.
func TestSearchBodyMarshalsDeclaredNames(t *testing.T) {
	encoded, err := json.Marshal(searchBody{queryParam: "query", query: "golang", maxParam: "max_results", maxResults: 7})
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decoding the payload: %v", err)
	}
	if decoded["query"] != "golang" || decoded["max_results"] != float64(7) {
		t.Fatalf("payload = %s, want the declared names", encoded)
	}
}
