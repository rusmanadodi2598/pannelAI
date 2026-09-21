// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/usage.go
// @for       The usage filter query and the summary/timeseries contracts.
// @uses      internal/domain (UsageFilter, UsageTotals).
// @reason    SPEC-API-001 §7.12 fixes the filter set (from, to, group_by,
//
//	granularity) and §2.4 requires those as typed structs validated
//	before the service sees them, so an unknown group_by and
//	granularity are 400s here rather than silently ignored queries.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-18
package schema

import (
	"net/http"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// maxQueryTextBytes bounds the free-text filters so a pasted blob cannot reach
// a pattern match (AGENTS.md §1.7: no unbounded input in a query).
const maxQueryTextBytes = 200

// usageFilterFields names every free-text filter so one loop can bound them all
// without repeating the check per field.
var usageFilterFields = []string{"status", "endpoint_id", "provider_id", "model", "gateway_key_id", "q"}

// UsageFilterQuery is the decoded filter set shared by summary, timeseries, and
// records (SPEC-API-001 §7.12). GroupBy and Granularity are empty when the
// caller did not ask for them. Status is the domain value object, so the closed
// set is held by the type from the boundary inward (draft 010 F2/F9).
type UsageFilterQuery struct {
	From        *time.Time
	To          *time.Time
	GroupBy     string
	Granularity string
	Status      domain.UsageStatus
	EndpointID  string
	ProviderID  string
	Model       string
	GatewayKey  string
	Q           string
}

// DecodeUsageFilter reads and validates the shared usage filters. A malformed
// timestamp, an inverted range, an unknown group_by or granularity, and an
// over-long free-text filter are all VALIDATION_ERROR, so the query the service
// runs is one the contract describes.
func DecodeUsageFilter(r *http.Request) (UsageFilterQuery, error) {
	query := r.URL.Query()
	from, err := decodeTimeParam(query.Get("from"), "from")
	if err != nil {
		return UsageFilterQuery{}, err
	}
	to, err := decodeTimeParam(query.Get("to"), "to")
	if err != nil {
		return UsageFilterQuery{}, err
	}
	if from != nil && to != nil && to.Before(*from) {
		return UsageFilterQuery{}, domain.NewValidationError("to must not be earlier than from")
	}

	groupBy := strings.TrimSpace(query.Get("group_by"))
	if groupBy != "" && !domain.UsageGroupBy(groupBy).IsValid() {
		return UsageFilterQuery{}, domain.NewValidationError("group_by must be one of provider, model, endpoint, gateway_key")
	}
	granularity := strings.TrimSpace(query.Get("granularity"))
	if granularity != "" && !domain.UsageGranularity(granularity).IsValid() {
		return UsageFilterQuery{}, domain.NewValidationError("granularity must be one of hour, day")
	}

	// The status closed set is checked here, at the boundary, rather than left
	// to the repository: the read predicate treats an empty value as
	// "unfiltered" and every other value as an equality, so an unknown value
	// must be a 400, not a silently empty 200 (draft 010 F2). An omitted or
	// empty parameter stays "unfiltered", which is why the parse runs only
	// when a value was sent (the same guard group_by and granularity use).
	var status domain.UsageStatus
	if raw := strings.TrimSpace(query.Get("status")); raw != "" {
		parsed, err := domain.ParseUsageStatus(raw)
		if err != nil {
			return UsageFilterQuery{}, err
		}
		status = parsed
	}

	texts := UsageFilterQuery{
		From: from, To: to, GroupBy: groupBy, Granularity: granularity,
		Status:     status,
		EndpointID: strings.TrimSpace(query.Get("endpoint_id")),
		ProviderID: strings.TrimSpace(query.Get("provider_id")),
		Model:      strings.TrimSpace(query.Get("model")),
		GatewayKey: strings.TrimSpace(query.Get("gateway_key_id")),
		Q:          strings.TrimSpace(query.Get("q")),
	}
	for _, field := range usageFilterFields {
		if len(texts.value(field)) > maxQueryTextBytes {
			return UsageFilterQuery{}, domain.NewValidationError(field + " must be at most 200 characters")
		}
	}
	return texts, nil
}

// value reads one named filter so the bound check stays a single loop.
func (q UsageFilterQuery) value(field string) string {
	switch field {
	case "status":
		return string(q.Status)
	case "endpoint_id":
		return q.EndpointID
	case "provider_id":
		return q.ProviderID
	case "model":
		return q.Model
	case "gateway_key_id":
		return q.GatewayKey
	default:
		return q.Q
	}
}

// Filter lowers the validated query into the domain filter, applying the read
// window default when the caller sent no range, so an unbounded scan can never
// be requested (AGENTS.md §1.7).
func (q UsageFilterQuery) Filter(now time.Time) domain.UsageFilter {
	return domain.NewUsageFilter(domain.UsageFilterInput{
		From: q.From, To: q.To, ProviderID: q.ProviderID, EndpointID: q.EndpointID,
		Model: q.Model, GatewayKeyID: q.GatewayKey, Status: q.Status, Query: q.Q,
	}, now)
}

// decodeTimeParam parses an RFC3339 timestamp (SPEC-API-001 §4: timestamps are
// RFC3339 UTC). An absent parameter stays nil, which means "unbounded" and is
// distinct from a zero instant.
func decodeTimeParam(value, name string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, domain.NewValidationError(name + " must be an RFC3339 timestamp")
	}
	utc := parsed.UTC()
	return &utc, nil
}

// UsageTotalsResponse is the aggregate block every usage response carries. Cost
// is a decimal string and every token field an integer (SPEC-API-001 §4), and
// the error rate is a decimal string for the same reason.
type UsageTotalsResponse struct {
	Requests         int64  `json:"requests"`
	TokensIn         int64  `json:"tokens_in"`
	TokensOut        int64  `json:"tokens_out"`
	TokensCacheRead  int64  `json:"tokens_cache_read"`
	TokensCacheWrite int64  `json:"tokens_cache_write"`
	CostUSD          string `json:"cost_usd"`
	LatencyMS        int64  `json:"latency_ms"`
	LatencyP50MS     int64  `json:"latency_p50_ms"`
	LatencyP95MS     int64  `json:"latency_p95_ms"`
	ErrorCount       int64  `json:"error_count"`
	ErrorRate        string `json:"error_rate"`
}

// UsageGroupResponse is one group-by row.
type UsageGroupResponse struct {
	Key    string              `json:"key"`
	Totals UsageTotalsResponse `json:"totals"`
}

// UsageSummaryResponse is the body of GET /api/v1/usage/summary.
type UsageSummaryResponse struct {
	From    string               `json:"from"`
	To      string               `json:"to"`
	GroupBy string               `json:"group_by"`
	Totals  UsageTotalsResponse  `json:"totals"`
	Groups  []UsageGroupResponse `json:"groups"`
}

// UsageBucketResponse is one timeseries point.
type UsageBucketResponse struct {
	Bucket string              `json:"bucket"`
	Totals UsageTotalsResponse `json:"totals"`
}

// UsageTimeseriesResponse is the body of GET /api/v1/usage/timeseries.
type UsageTimeseriesResponse struct {
	Granularity string                `json:"granularity"`
	From        string                `json:"from"`
	To          string                `json:"to"`
	Buckets     []UsageBucketResponse `json:"buckets"`
}

// UsageTotalsResponseFrom maps a domain aggregate onto the wire shape.
func UsageTotalsResponseFrom(t domain.UsageTotals) UsageTotalsResponse {
	return UsageTotalsResponse{
		Requests:         t.Requests,
		TokensIn:         t.TokensIn,
		TokensOut:        t.TokensOut,
		TokensCacheRead:  t.TokensCacheRead,
		TokensCacheWrite: t.TokensCacheWrite,
		CostUSD:          t.CostUSD,
		LatencyMS:        t.LatencyMS,
		LatencyP50MS:     t.LatencyP50MS,
		LatencyP95MS:     t.LatencyP95MS,
		ErrorCount:       t.ErrorCount,
		ErrorRate:        t.ErrorRate(),
	}
}
