// The published-quota half of the quota handler's output: the provider's own answer,
// mapped onto the wire without being merged into this gateway's counters.
//
// @file      internal/handler/quota_published_response.go
// @for       Mapping provider-published quota answers onto the collection and detail bodies.
// @uses      internal/schema, internal/service.
// @reason    Two ledgers share one screen and neither corrects the other, so the mapping that
//
//	keeps them apart (decimal strings as the provider spelled them, a NULL ceiling that is not a
//	zero, an attempt stamp that is not the figures' stamp) lives in one place instead of being
//	repeated by each route that carries an answer.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-10-02
package handler

import (
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// publishedUsageResponse maps one provider answer onto the wire shape.
func publishedUsageResponse(usage service.PublishedUsage) schema.PublishedQuotaUsageResponse {
	resp := schema.PublishedQuotaUsageResponse{
		EndpointID:  usage.EndpointID,
		ProviderID:  usage.ProviderID,
		Plan:        usage.Plan,
		FetchedAt:   schema.Timestamp(usage.FetchedAt),
		Message:     usage.Message,
		Data:        publishedWindowResponses(usage.Windows),
		Cached:      usage.Cached,
		Failures:    usage.FailuresRun,
		NeverPolled: usage.NeverPolled,
	}
	if usage.LastAttemptAt != nil {
		resp.LastAttemptAt = schema.Timestamp(*usage.LastAttemptAt)
	}
	return resp
}

// publishedUsageResponses maps a page's cached provider answers onto the collection
// body. Always a non-nil slice: an account the worker has not answered is absent from the
// array, and a page whose providers have all fallen silent still answers `[]` — the same
// rule `data` follows, because `null` would be a second spelling of "nothing here" that a
// reader has to special-case.
func publishedUsageResponses(answers []service.PublishedUsage) []schema.PublishedQuotaUsageResponse {
	out := make([]schema.PublishedQuotaUsageResponse, 0, len(answers))
	for _, answer := range answers {
		out = append(out, publishedUsageResponse(answer))
	}
	return out
}

// publishedWindowResponses maps the provider's buckets onto the wire shape, always
// as a non-nil slice so an account with nothing published answers an empty array
// beside the message that says why.
func publishedWindowResponses(windows []service.PublishedWindow) []schema.PublishedQuotaWindowResponse {
	resp := make([]schema.PublishedQuotaWindowResponse, 0, len(windows))
	for _, window := range windows {
		row := schema.PublishedQuotaWindowResponse{
			Label:           window.Label,
			Used:            schema.PublishedQuotaAmount(window.Used),
			Unit:            window.Unit,
			Unlimited:       window.Unlimited,
			IsCreditBalance: window.IsCreditBalance,
			Recurring:       window.Recurring,
		}
		if window.HasTotal {
			row.Total = ptr(schema.PublishedQuotaAmount(window.Total))
		}
		if window.ResetsAt != nil {
			row.ResetsAt = ptr(schema.Timestamp(*window.ResetsAt))
		}
		resp = append(resp, row)
	}
	return resp
}
