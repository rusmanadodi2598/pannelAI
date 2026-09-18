// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/usage_totals.go
// @for       The UsageTotals aggregate: summing the counters and deriving the
//
//	error rate a summary displays.
//
// @uses      internal/domain (Decimal), math/big.
// @reason    SPEC-API-001 §4 makes cost a decimal string on the wire, so the
//
//	sum that produces it must be exact: a float would drift from the sum
//	a client computes from the strings it was shown. The rule lives
//	beside the totals type rather than in the query, so an in-memory
//	sum and a SQL rollup cannot disagree.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-18
package domain

import (
	"math/big"
	"time"
)

// errorRatePlaces is the display scale of a derived error rate.
const errorRatePlaces = 4

// UsageTotals is the aggregate a summary, a group row, or a bucket reports.
// CostUSD is carried as a decimal string so the service sums exactly by parsing
// it once and rendering once, and the wire never carries a float (§4).
type UsageTotals struct {
	Requests         int64
	TokensIn         int64
	TokensOut        int64
	TokensCacheRead  int64
	TokensCacheWrite int64
	CostUSD          string
	LatencyMS        int64
	LatencyP50MS     int64
	LatencyP95MS     int64
	ErrorCount       int64
}

// ErrorRate renders the failed-request share as a decimal string with four
// places, or "0.0000" for an empty range. A rate is a derived display figure,
// so it is computed from the counters rather than stored.
func (t UsageTotals) ErrorRate() string {
	if t.Requests <= 0 {
		return "0.0000"
	}
	rate := new(big.Rat).SetFrac(big.NewInt(t.ErrorCount), big.NewInt(t.Requests))
	return rate.FloatString(errorRatePlaces)
}

// Add returns the element-wise sum, with the costs summed exactly. It is how a
// group breakdown is totalled into the summary's own totals, so the parts and
// the whole are computed the same way instead of by two different rules.
//
// The percentiles are carried from the receiver rather than combined: a median
// of medians is a different and wrong number, so a caller that needs real
// percentiles reads them from the aggregate that computed them over every row.
func (t UsageTotals) Add(other UsageTotals) UsageTotals {
	return UsageTotals{
		Requests:         t.Requests + other.Requests,
		TokensIn:         t.TokensIn + other.TokensIn,
		TokensOut:        t.TokensOut + other.TokensOut,
		TokensCacheRead:  t.TokensCacheRead + other.TokensCacheRead,
		TokensCacheWrite: t.TokensCacheWrite + other.TokensCacheWrite,
		CostUSD:          SumCosts(t.CostUSD, other.CostUSD),
		LatencyMS:        t.LatencyMS + other.LatencyMS,
		LatencyP50MS:     t.LatencyP50MS,
		LatencyP95MS:     t.LatencyP95MS,
		ErrorCount:       t.ErrorCount + other.ErrorCount,
	}
}

// SumCosts adds two cost strings exactly, rendering the result at the column's
// scale. An unparseable operand is treated as zero: the stored value came from
// a numeric column, so a malformed string cannot arise from this schema, and
// failing a whole summary over it would be the worse outcome.
func SumCosts(a, b string) string {
	left, err := ParseDecimal(a)
	if err != nil {
		left = ZeroDecimal()
	}
	right, err := ParseDecimal(b)
	if err != nil {
		right = ZeroDecimal()
	}
	return left.Add(right).String()
}

// RateBucket is one timeseries point: an instant and the totals it covers.
type RateBucket struct {
	Bucket time.Time
	Totals UsageTotals
}

// UsageGroupRow is one summary breakdown row.
type UsageGroupRow struct {
	Key    string
	Totals UsageTotals
}
