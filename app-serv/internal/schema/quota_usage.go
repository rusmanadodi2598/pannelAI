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
// @stability stable
// @since     2026-09-28
package schema

import (
	"strconv"
)

// PublishedQuotaWindowResponse is one bucket the provider published. Total is
// absent when the provider states no ceiling, which is not a ceiling of zero.
//
// The three flags let the panel tell those states apart rather than invent one:
// Unlimited means no ceiling to divide by, IsCreditBalance means the figure is
// money in a named currency rather than a share of a window, and Unit carries
// the dimension the provider named. Each is omitted when not the case, so a
// plain requests window answers the fields it always did.
type PublishedQuotaWindowResponse struct {
	Label           string  `json:"label"`
	Used            string  `json:"used"`
	Total           *string `json:"total,omitempty"`
	ResetsAt        *string `json:"resets_at,omitempty"`
	Unit            string  `json:"unit,omitempty"`
	Unlimited       bool    `json:"unlimited,omitempty"`
	IsCreditBalance bool    `json:"is_credit_balance,omitempty"`
	Recurring       bool    `json:"recurring,omitempty"`
}

// PublishedQuotaUsageResponse is the body of
// GET /api/v1/quotas/{endpoint_id}/usage: the provider's own words about one
// connection. Message carries the soft outcome, the family publishes nothing,
// the credential was refused, the provider errored, and is then the whole answer
// with Data empty: the card renders that sentence rather than failing the page.
// Cached separates the number the poll worker stored from one the gateway asked
// for now, because a staleness claim travels with its figure.
type PublishedQuotaUsageResponse struct {
	EndpointID string `json:"endpoint_id"`
	ProviderID string `json:"provider_id"`
	Plan       string `json:"plan,omitempty"`
	// FetchedAt dates the figures in Data, not the attempt that produced them: a
	// soft answer leaves it at the row's creation instant, which is a
	// placeholder and not a stamp.
	FetchedAt string                         `json:"fetched_at"`
	Message   string                         `json:"message,omitempty"`
	Data      []PublishedQuotaWindowResponse `json:"data"`
	Cached    bool                           `json:"cached,omitempty"`

	// Failures counts consecutive polls that produced no answer and
	// LastAttemptAt says when the worker last asked. They travel together: a
	// card can hold the last good figures while the current poll fails.
	Failures      int    `json:"failures,omitempty"`
	LastAttemptAt string `json:"last_attempt_at,omitempty"`

	// NeverPolled marks an account the poll worker has not answered for: the
	// absence of an answer, not an answer of absence. "Nobody asked yet" and
	// "the provider says nothing" need different things from the operator.
	NeverPolled bool `json:"never_polled,omitempty"`
}

// PublishedQuotaAmount renders one provider-reported amount as a decimal string at
// the precision the value actually carries: no padding to a fixed scale, which
// would claim a credit balance of 3000 is known to eight places.
func PublishedQuotaAmount(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
