// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_search_result.go
// @for       Reading a search provider's answer into the §7.10 result shape.
// @uses      internal/dataplane, internal/schema, encoding/json, strings.
// @reason    Search APIs disagree on where the list lives and what a result's
//
//	fields are called, so the reading is one place with the spellings
//	named rather than a branch per provider inside the use case.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"encoding/json"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// searchEnvelope is the union of the list locations the reference's search
// providers answer with: `results` (Tavily, Exa, SearXNG, You.com), `organic`
// (Serper), and `web.results` (Brave).
type searchEnvelope struct {
	Results []searchItem `json:"results"`
	Organic []searchItem `json:"organic"`
	Web     *struct {
		Results []searchItem `json:"results"`
	} `json:"web"`
}

// searchItem is one upstream result, with the spellings the providers use for
// the same values: Serper answers `link` where the others answer `url`, and
// Tavily answers `content` where the others answer `snippet` or `description`.
type searchItem struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Link        string `json:"link"`
	Snippet     string `json:"snippet"`
	Description string `json:"description"`
	Content     string `json:"content"`
	Text        string `json:"text"`
}

// normalizeSearchResults reads whichever list the provider answered with and
// maps it onto the reference's result item.
func normalizeSearchResults(body []byte) ([]schema.SearchResult, error) {
	var upstream searchEnvelope
	if err := json.Unmarshal(body, &upstream); err != nil {
		return nil, dataplane.InternalError("the search answer could not be read", err)
	}
	items := upstream.Results
	if len(items) == 0 && len(upstream.Organic) > 0 {
		items = upstream.Organic
	}
	if len(items) == 0 && upstream.Web != nil {
		items = upstream.Web.Results
	}

	results := make([]schema.SearchResult, 0, len(items))
	for index, item := range items {
		results = append(results, schema.SearchResult{
			Title:    item.Title,
			URL:      firstNonEmpty(item.URL, item.Link),
			Snippet:  firstNonEmpty(item.Snippet, item.Description, item.Content, item.Text),
			Position: index + 1,
		})
	}
	return results, nil
}

// firstNonEmpty returns the first non-empty value, so one result shape can be
// read from several spellings without a branch per field.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
