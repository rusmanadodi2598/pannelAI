// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/chat_record_cost.go
// @for       The chat plane's cost estimate: the rate tables' answer for the
//
//	tokens one call reported.
//
// @uses      internal/dataplane, internal/registry, internal/schema.
// @reason    Live evidence 2026-09-23: every served chat call recorded
//
//	cost_usd = 0 because the usage row was built without the field, so
//	the panel's cost series read zero for every model. The estimate
//	itself lives in the registry's rate tables (the reference computes
//	it locally rather than reading a price from the upstream), and this
//	file is the one place that maps a call's outcome onto the formula's
//	input, so the two accounting sites cannot map the cache and
//	reasoning splits differently.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-23
package service

import (
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// zeroCostUSD is what a call without an estimate records, at the decimal scale
// the usage column stores so a reader sees one shape for "no cost". It is
// spelled out rather than left empty because the domain rejects an unparseable
// amount and an absent one would be read as zero by accident rather than by
// this rule.
const zeroCostUSD = "0.00000000"

// chatCostEstimate renders the estimate a chat usage row carries. It answers
// zero for a call that failed (nothing was delivered), for a call that reported
// no usage (nothing was measured), and for a model the rate tables do not price
// (no rate to apply) — the reference's own resolver answers null for the last
// case, and a zero estimate is what the panel renders as "no cost data".
func chatCostEstimate(outcome dataplane.Outcome, errorCode string) string {
	if errorCode != "" || outcome.Usage == nil {
		return zeroCostUSD
	}
	return registry.ChatCost(outcome.ProviderID, outcome.Model, chatTokenCounts(outcome.Usage))
}

// chatTokenCounts maps the OpenAI accounting block onto the formula's input.
// The prompt count is already cache-inclusive at every fold site in the data
// plane (the Claude reader adds the cache fields in, the OpenAI reader passes
// them through), which is the convention the formula subtracts from, and
// reasoning tokens ride their own premium rate.
func chatTokenCounts(usage *schema.Usage) registry.TokenCounts {
	counts := registry.TokenCounts{
		Prompt:     int64(usage.PromptTokens),
		Completion: int64(usage.CompletionTokens),
	}
	if details := usage.PromptTokensDetails; details != nil {
		counts.Cached = int64(details.CachedTokens)
		counts.CacheCreation = int64(details.CacheCreationTokens)
	}
	if detail := usage.CompletionTokensDetail; detail != nil {
		counts.Reasoning = int64(detail.ReasoningTokens)
	}
	return counts
}
