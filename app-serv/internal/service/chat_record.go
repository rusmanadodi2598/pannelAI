// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/chat_record.go
// @for       The accounting pair one chat call writes: a usage row and a
//
//	request log under the request's own identifier.
//
// @uses      internal/dataplane, internal/domain, context, time.
// @reason    SPEC-API-001 §7.12/§7.13 make one recorded row per request part of
//
//	the §7.15 pipeline, and register G18 found the chat plane writing only
//	the usage half. Keeping the pair beside chat.go means the failure path
//	and the success path cannot disagree about which rows a call leaves,
//	and keeps chat.go inside the AGENTS.md §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// record writes one usage row and one request log for a chat call, under the
// router's request id so the pair is reachable from either surface (§4).
//
// The bodies are handed over as they arrived: the capture setting and the
// truncation rule are applied by LogService.Record, so a deployment with capture
// off stores no body at all, and this caller does not re-decide what §7.13
// governs in one place.
//
// The log's error text is the code alone, never the failure's message: a chat
// upstream's message is quoted back verbatim, and an authentication failure's
// message can carry the presented credential — a stored row must not hold either.
//
// A recording failure is deliberately not returned: the client already has its
// answer, and failing the request over an accounting write would turn a served
// call into an error the client cannot act on.
func (s *ChatService) record(ctx context.Context, in dataplane.Request, outcome dataplane.Outcome, keyID, errorCode string) {
	if s.usage == nil && s.logs == nil && s.quotas == nil {
		return
	}
	requestID := s.requestIDFrom(ctx)
	status := domain.UsageStatusSuccess
	if errorCode != "" {
		status = domain.UsageStatusError
	}
	// The usage row is written only for a call that reached an attempt: the
	// aggregate requires a provider and a model, and a request refused before
	// the pipeline (an unknown model, an invalid body) has neither. Skipping it
	// here rather than letting the aggregate reject it is deliberate — a
	// swallowed validation error is what hid this plane's missing error row
	// (register G17).
	if s.usage != nil && outcome.ProviderID != "" && outcome.Model != "" {
		input := domain.UsageRecordInput{
			RequestID:    requestID,
			TS:           s.clock().UTC(),
			EndpointID:   outcome.EndpointID,
			ProviderID:   outcome.ProviderID,
			Model:        outcome.Model,
			Combo:        outcome.Combo,
			GatewayKeyID: keyID,
			LatencyMS:    outcome.LatencyMS,
			Status:       status,
			ErrorCode:    errorCode,
		}
		if outcome.Usage != nil {
			input.TokensIn = int64(outcome.Usage.PromptTokens)
			input.TokensOut = int64(outcome.Usage.CompletionTokens)
			if details := outcome.Usage.PromptTokensDetails; details != nil {
				input.TokensCacheRead = int64(details.CachedTokens)
				input.TokensCacheWrite = int64(details.CacheCreationTokens)
			}
		}
		// The estimate is the rate tables' answer for the tokens this call
		// reported, and zero for every path without one — a failed call
		// delivered nothing, an unpriced model has no rate to apply, and a
		// report of all zeros prices to zero. §7.12: the figure is an estimate
		// for display, never a billed amount.
		input.CostUSD = chatCostEstimate(outcome, errorCode)
		// reason: a failed accounting write must not fail a request the client
		// has already received an answer to; a lost row is reported by the
		// panel's totals, which is where an operator can act on it.
		_, _ = s.usage.Record(ctx, input)
	}
	// The quota counters advance by the tokens the upstream billed, in every
	// window this call falls inside (§7.12). A call that reported no usage
	// advanced nothing, which is the same deliberate skip the usage row makes:
	// counting a request the upstream never measured would put a fabricated
	// number in a window an operator reads as spend.
	if s.quotas != nil && outcome.Usage != nil {
		s.quotas.Record(ctx, outcome.EndpointID, chatQuotaUnits(outcome.Usage))
	}
	if s.logs != nil {
		// reason: same as above — the log is the second half of the accounting
		// pair, not a condition of the answer.
		_, _ = s.logs.Record(ctx, domain.RequestLogInput{
			RequestID:    requestID,
			GatewayKeyID: keyID,
			EndpointID:   outcome.EndpointID,
			ProviderID:   outcome.ProviderID,
			Model:        outcome.Model,
			Status:       requestLogStatus(status),
			LatencyMS:    outcome.LatencyMS,
			RequestBody:  string(in.Raw),
			ResponseBody: string(outcome.Body),
			Error:        errorCode,
		})
	}
}

// chatQuotaUnits is what one chat call spends against a quota window: every
// token the upstream billed, prompt and completion alike, because a window is a
// budget of work rather than of one direction. Cached reads count too, since the
// provider counts them against its own rate limit; the cached-write side does
// not, because it is not a second billed unit. A report whose total is zero or
// negative is treated as nothing to count rather than a negative spend.
func chatQuotaUnits(usage *schema.Usage) int64 {
	if usage == nil {
		return 0
	}
	units := int64(usage.PromptTokens) + int64(usage.CompletionTokens)
	if units < 0 {
		return 0
	}
	return units
}

// requestIDFrom reads the router's request id, falling back to a fresh ULID so
// both rows of one call carry one (the column is not nullable in practice, and a
// blank id would make the pair unfindable). The rule itself lives in
// dataplane_record.go, where the media plane reads it the same way.
func (s *ChatService) requestIDFrom(ctx context.Context) string {
	return requestIDOrNew(ctx, s.requestID, s.clock)
}
