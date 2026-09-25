// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/pricing.go
// @for       The embedded rate tables and the three-step resolver behind every
//
//	cost estimate the usage accounting writes.
//
// @uses      embed, encoding/json, strings, sync (standard library only).
// @reason    SPEC-API-001 §7.12 states the recorded cost figures are estimates
//
//	for display, and the reference computes them locally from
//	$-per-million-token tables rather than reading a price from the
//	upstream. The tables are generated from the reference
//	(tools/pricing-gen.mjs) and embedded the way registry.yaml is, so a
//	deployment carries its rates inside the binary and a rate change is
//	a regeneration, not an edit.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package registry

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

// pricingFile is the generated rate document's embedded name.
const pricingFile = "pricing.json"

//go:embed pricing.json
var embeddedPricing []byte

// Rate is one model's per-million-token prices, as the reference declares them.
// A field is empty when the reference declares no rate for it, which is what
// the cost formula's fallbacks read.
type Rate struct {
	Input         string `json:"input"`
	Output        string `json:"output"`
	Cached        string `json:"cached"`
	Reasoning     string `json:"reasoning"`
	CacheCreation string `json:"cache_creation"`
}

// providerRate is one provider-specific override row.
type providerRate struct {
	Provider string `json:"provider"`
	ID       string `json:"id"`
	Rate
}

// patternRate is one glob rule, in the reference's order (first match wins).
type patternRate struct {
	Pattern string `json:"pattern"`
	Rate
}

// pricingDocument is the generated file's shape.
type pricingDocument struct {
	Revision string         `json:"revision"`
	Source   string         `json:"source"`
	Note     string         `json:"note"`
	Model    []modelRate    `json:"model"`
	Provider []providerRate `json:"provider"`
	Pattern  []patternRate  `json:"pattern"`
}

// modelRate is one canonical model row.
type modelRate struct {
	ID string `json:"id"`
	Rate
}

// pricingIndex is the decoded tables. It is built once, on first use, because
// the resolver is called from the request path and the document is static.
type pricingIndex struct {
	byModel         map[string]Rate
	byProviderModel map[string]Rate
	patterns        []patternRule
}

// patternRule is one glob rule with its pattern pre-lower-cased, because
// matchesGlob expects lower-cased arguments and lower-casing per request would
// re-do work the once-built index can carry.
type patternRule struct {
	pattern string
	Rate
}

var (
	pricingOnce sync.Once
	pricing     pricingIndex
)

// loadPricing decodes the embedded tables exactly once. A malformed document is
// a build-time fault rather than a request-time one: the file is generated and
// committed, so the only way it fails to parse is a broken regeneration, and
// panicking at first use names that instead of silently pricing everything at
// zero.
func loadPricing() {
	pricingOnce.Do(func() {
		var doc pricingDocument
		if err := json.Unmarshal(embeddedPricing, &doc); err != nil {
			panic("registry: decoding embedded " + pricingFile + ": " + err.Error())
		}
		index := pricingIndex{
			byModel:         make(map[string]Rate, len(doc.Model)),
			byProviderModel: make(map[string]Rate, len(doc.Provider)),
			patterns:        make([]patternRule, len(doc.Pattern)),
		}
		for _, row := range doc.Model {
			// reason: exact-case key, same as the reference's direct index — the
			// table carries case-twins with different rates, so a folded key would
			// silently swap a model onto its twin's price.
			index.byModel[row.ID] = row.Rate
		}
		for _, row := range doc.Provider {
			index.byProviderModel[pricingKey(row.Provider, row.ID)] = row.Rate
		}
		for i, rule := range doc.Pattern {
			index.patterns[i] = patternRule{pattern: strings.ToLower(rule.Pattern), Rate: rule.Rate}
		}
		pricing = index
	})
}

// pricingKey names one provider-specific override. The lookup is exact-case,
// because the reference indexes its override table with the provider and model
// strings as written and carries case-twin entries whose rates differ
// ("MiniMax-M2.5" against "minimax-m2.5"), so folding case here would price a
// registry pair at its twin's rate.
func pricingKey(provider, model string) string {
	return provider + "\x00" + model
}

// ResolvePricing answers the reference's three-step chain for one
// (provider, model) pair: the provider-specific override, then the canonical
// model entry — with any vendor prefix stripped, and also under its original
// spelling — then the ordered glob patterns. It reports false when nothing
// matches, which the caller records as a zero cost rather than guessing a rate.
func ResolvePricing(provider, model string) (Rate, bool) {
	loadPricing()
	model = strings.TrimSpace(model)
	if model == "" {
		return Rate{}, false
	}

	if rate, ok := pricing.byProviderModel[pricingKey(provider, model)]; ok {
		return rate, true
	}

	base := model
	if slash := strings.LastIndex(model, "/"); slash >= 0 {
		base = model[slash+1:]
	}
	if rate, ok := pricing.byModel[base]; ok {
		return rate, true
	}
	if rate, ok := pricing.byModel[model]; ok {
		return rate, true
	}

	// The patterns are the reference's one case-insensitive step: its regex
	// matches case-blind because registry ids mix casing ("MiniMax-M2.5" and
	// "minimax-m2.5" both exist), so both sides are lowered for the match.
	lowerBase, lowerModel := strings.ToLower(base), strings.ToLower(model)
	for _, rule := range pricing.patterns {
		if matchesGlob(rule.pattern, lowerBase) || matchesGlob(rule.pattern, lowerModel) {
			return rule.Rate, true
		}
	}
	return Rate{}, false
}
