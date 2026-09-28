// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/quota_usage.go
// @for       The published-quota read contract: what one connection's provider says
//
//	it has left.
//
// @uses      strconv.
// @reason    SPEC-API-001 §7.12's windows are counted by this gateway, so they are
//
//	integers. A provider's own allocation is not: it publishes credits and
//	requests with fractions, and states no ceiling at all for an unlimited
//	bucket. Those amounts cross the wire as decimal strings, the same §4
//	rule cost follows, because a figure the panel has to divide to draw a
//	bar must not arrive rounded by whoever encoded it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-28
package schema

import (
	"strconv"
)

// PublishedQuotaWindowResponse is one bucket the provider published. Total is
// absent when the provider states no ceiling, which is not a ceiling of zero: an
// unlimited bucket has a spent amount and nothing to spend it against.
type PublishedQuotaWindowResponse struct {
	Label    string  `json:"label"`
	Used     string  `json:"used"`
	Total    *string `json:"total,omitempty"`
	ResetsAt *string `json:"resets_at,omitempty"`
}

// PublishedQuotaUsageResponse is the body of
// GET /api/v1/quotas/{endpoint_id}/usage: the provider's own words about one
// connection. Message carries the soft outcome — the family publishes nothing,
// the credential was refused, the provider errored — and is then the whole answer,
// with Data empty, because the reference's card renders that sentence rather than
// failing the page.
type PublishedQuotaUsageResponse struct {
	EndpointID string                         `json:"endpoint_id"`
	ProviderID string                         `json:"provider_id"`
	Plan       string                         `json:"plan,omitempty"`
	FetchedAt  string                         `json:"fetched_at"`
	Message    string                         `json:"message,omitempty"`
	Data       []PublishedQuotaWindowResponse `json:"data"`
}

// PublishedQuotaAmount renders one provider-reported amount as a decimal string at
// the precision the value actually carries: no padding to a fixed scale, which
// would claim a credit balance of 3000 is known to eight places.
func PublishedQuotaAmount(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
