// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/pricing_cost.go
// @for       The token-to-cost estimate: the reference's additive formula over
//
//	the embedded rates, in exact rational arithmetic.
//
// @uses      math/big, strings, internal/registry (the resolved rates).
// @reason    SPEC-API-001 §4 requires cost to cross the wire as a decimal
//
//	string and §7.12 states the figure is an estimate for display. The
//	reference computes it in float64, which is where its own 8th
//	decimal drifts; this port keeps the sum exact and renders it at the
//	column's scale, so the number a client reads back is the number
//	the gateway computed. The formula is the reference's, including its
//	conventions: prompt_tokens is cache-inclusive, so the cached and
//	cache-creation subsets are subtracted before the full input rate
//	applies, and reasoning tokens are a premium added on their own rate.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package registry

import (
	"math/big"
	"strings"
)

// TokenCounts is the accounting a cost estimate reads. The fields mirror the
// reference's canonical convention: Prompt includes the cache subsets, so
// Cached and CacheCreation are subtracted rather than added.
type TokenCounts struct {
	Prompt        int64
	Completion    int64
	Cached        int64
	CacheCreation int64
	Reasoning     int64
}

// million is the denominator every rate is quoted per.
var million = new(big.Rat).SetInt64(1000000)

// ChatCost renders the estimated cost of one chat call as the decimal string a
// usage row stores, at the column's scale. An unpriced model, an empty report,
// or a report whose tokens are all zero answers "0": the reference's resolver
// answers null for a model it cannot price, and a zero estimate is what the
// panel renders as "no cost data" rather than a guessed rate.
func ChatCost(provider, model string, tokens TokenCounts) string {
	rates, ok := ResolvePricing(provider, model)
	if !ok {
		return zeroCost
	}
	return CalculateCost(tokens, rates)
}

// zeroCost is the rendering of an absent estimate, shared so every path that
// answers "no cost" answers the same string. It is the column's scale rather
// than a bare "0", because the reference's own cost function renders its
// zero answers there and the corpus pins this port to that shape.
const zeroCost = "0.00000000"

// CalculateCost applies the reference's formula to already-resolved rates. It
// is exported so the formula can be tested against the reference's own numbers
// without going through the resolver, and so a caller holding a rate for
// another reason (a stored override, a future provider-reported price) prices a
// call the same way the resolver path does.
//
// The arithmetic is exact: every term is a rational, and the sum is rendered at
// the column's scale. Nothing here is a float, so a rate with a six-decimal
// tail (several real entries have one) cannot drift on the way to the row.
//
// A sum of zero renders as the media plane's own "no cost" spelling rather
// than at the column's scale, so "this call cost nothing" reads the same
// whichever accounting site wrote it.
func CalculateCost(tokens TokenCounts, rates Rate) string {
	total := new(big.Rat)

	// The cache subsets are part of the prompt, so they are removed before the
	// full input rate applies; a pathological report whose subsets exceed the
	// prompt clamps at zero rather than pricing a negative amount.
	prompt := tokens.Prompt
	if prompt < 0 {
		prompt = 0
	}
	cached := clampTokens(tokens.Cached)
	cacheCreation := clampTokens(tokens.CacheCreation)
	nonCached := prompt - cached - cacheCreation
	if nonCached < 0 {
		nonCached = 0
	}

	total.Add(total, term(nonCached, rates.Input))
	total.Add(total, term(cached, fallbackRate(rates.Cached, rates.Input)))
	total.Add(total, term(cacheCreation, fallbackRate(rates.CacheCreation, rates.Input)))

	completion := clampTokens(tokens.Completion)
	total.Add(total, term(completion, rates.Output))

	// Reasoning is a premium rate applied on top of the output rate: the
	// reference adds it as a separate term, and its own extractors fold
	// provider-reported thinking tokens into completion before this point.
	total.Add(total, term(clampTokens(tokens.Reasoning), fallbackRate(rates.Reasoning, rates.Output)))

	return ratToDecimal(total)
}

// term multiplies a token count by a per-million rate.
func term(count int64, rate string) *big.Rat {
	value, ok := new(big.Rat).SetString(strings.TrimSpace(rate))
	if !ok || value.Sign() == 0 || count == 0 {
		return new(big.Rat)
	}
	product := new(big.Rat).Mul(value, new(big.Rat).SetInt64(count))
	return product.Quo(product, million)
}

// fallbackRate returns the primary rate when it is declared, otherwise the
// fallback the reference uses for that field.
func fallbackRate(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}

// clampTokens keeps a negative count from inverting a cost.
func clampTokens(count int64) int64 {
	if count < 0 {
		return 0
	}
	return count
}

// ratToDecimal renders an exact rational at the scale the numeric(20, 8)
// column stores, so the value a client reads back is the value computed.
func ratToDecimal(value *big.Rat) string {
	return value.FloatString(decimalScale)
}

// decimalScale matches the cost column's scale (SPEC-API-001 §6).
const decimalScale = 8
