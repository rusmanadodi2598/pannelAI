// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/usage_types.go
// @for       The usage vocabulary: filters, group-by dimensions, and totals.
// @uses      internal/domain (Decimal, AppError constructors), math/big, time.
// @reason    SPEC-API-001 §7.12 fixes the group-by dimension set and the cost
//
//	rule, so the closed sets and the aggregation arithmetic live here
//	where a unit test can exercise them with no database, and the
//	summary, timeseries, and records reads all speak one filter type.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-18
package domain

import "time"

// DefaultUsageWindow is how far back a usage read looks when the caller sends
// no range. It exists so a request without from/to can never become an
// unbounded scan (AGENTS.md §1.7), and it matches the panel's period selector
// default (SPEC-UI §6.5).
const DefaultUsageWindow = 24 * time.Hour

// UsageGroupBy is one dimension of the usage summary breakdown. The set is
// closed and mirrors the columns the migration indexes (SPEC-API-001 §6).
type UsageGroupBy string

const (
	// UsageGroupProvider breaks totals down by upstream provider.
	UsageGroupProvider UsageGroupBy = "provider"
	// UsageGroupModel breaks totals down by upstream model id.
	UsageGroupModel UsageGroupBy = "model"
	// UsageGroupEndpoint breaks totals down by configured endpoint.
	UsageGroupEndpoint UsageGroupBy = "endpoint"
	// UsageGroupGatewayKey breaks totals down by client key.
	UsageGroupGatewayKey UsageGroupBy = "gateway_key"
)

// ParseUsageGroupBy validates a wire value before it reaches a query. An
// unknown dimension is rejected rather than ignored, because silently returning
// ungrouped totals would look like a working filter with one row.
func ParseUsageGroupBy(value string) (UsageGroupBy, error) {
	group := UsageGroupBy(value)
	if !group.IsValid() {
		return "", NewValidationError("invalid group_by: " + value)
	}
	return group, nil
}

// IsValid reports whether the dimension is one the summary can group by. The
// empty dimension is not valid here: "no breakdown" is expressed by the caller
// omitting group_by, not by a zero value that would group on an empty column.
func (g UsageGroupBy) IsValid() bool {
	switch g {
	case UsageGroupProvider, UsageGroupModel, UsageGroupEndpoint, UsageGroupGatewayKey:
		return true
	default:
		return false
	}
}

// Column names the stored column this dimension groups on, so the repository
// never builds a column name from caller input (AGENTS.md §1.7).
func (g UsageGroupBy) Column() string {
	switch g {
	case UsageGroupProvider:
		return "provider_id"
	case UsageGroupModel:
		return "model"
	case UsageGroupEndpoint:
		return "endpoint_id"
	case UsageGroupGatewayKey:
		return "gateway_key_id"
	default:
		return ""
	}
}

// UsageGranularity is the timeseries bucket width.
type UsageGranularity string

const (
	// UsageHourly buckets by clock hour.
	UsageHourly UsageGranularity = "hour"
	// UsageDaily buckets by calendar day in UTC.
	UsageDaily UsageGranularity = "day"
)

// ParseUsageGranularity validates a wire value before it reaches a query.
func ParseUsageGranularity(value string) (UsageGranularity, error) {
	granularity := UsageGranularity(value)
	if !granularity.IsValid() {
		return "", NewValidationError("invalid granularity: " + value)
	}
	return granularity, nil
}

// IsValid reports whether the granularity is one the timeseries can bucket by.
// The empty value is invalid for the same reason an empty group_by is: the
// caller expresses "unset" by omitting the parameter.
func (g UsageGranularity) IsValid() bool {
	return g == UsageHourly || g == UsageDaily
}

// Interval renders the granularity as the PostgreSQL interval literal the
// bucket truncation uses. The value comes from this closed set, never from
// caller input, so it is safe to interpolate into the statement.
func (g UsageGranularity) Interval() string {
	if g == UsageDaily {
		return "1 day"
	}
	return "1 hour"
}

// ParseUsageStatus validates a wire value against the closed set before it
// reaches a query (draft 010 F2/F9). An unknown status is rejected rather than
// passed through, because the read predicate treats an empty value as
// "unfiltered" and every other value as an equality, so a value outside the
// set would silently match zero rows and answer 200.
func ParseUsageStatus(value string) (UsageStatus, error) {
	status := UsageStatus(value)
	if !status.IsValid() {
		return "", NewValidationError("status must be one of success, error")
	}
	return status, nil
}

// UsageFilter narrows a usage read. Every text field is empty when unfiltered,
// and From/To are always populated, because an unbounded range is not
// requestable (AGENTS.md §1.7). Status is the domain value object, so a value
// outside the closed set cannot be constructed into a filter (draft 010 F9).
type UsageFilter struct {
	From       time.Time
	To         time.Time
	ProviderID string
	EndpointID string
	Model      string
	GatewayKey string
	Status     UsageStatus
	Query      string
}

// UsageFilterInput is the constructor input, where a nil range means "use the
// default window" and is distinct from a zero instant.
type UsageFilterInput struct {
	From         *time.Time
	To           *time.Time
	ProviderID   string
	EndpointID   string
	Model        string
	GatewayKeyID string
	Status       UsageStatus
	Query        string
}

// NewUsageFilter resolves a partial filter into a bounded one. A missing To is
// "now" and a missing From is the default window before it, so every read has a
// finite range.
func NewUsageFilter(in UsageFilterInput, now time.Time) UsageFilter {
	to := now.UTC()
	if in.To != nil {
		to = in.To.UTC()
	}
	from := to.Add(-DefaultUsageWindow)
	if in.From != nil {
		from = in.From.UTC()
	}
	return UsageFilter{
		From:       from,
		To:         to,
		ProviderID: in.ProviderID,
		EndpointID: in.EndpointID,
		Model:      in.Model,
		GatewayKey: in.GatewayKeyID,
		Status:     in.Status,
		Query:      in.Query,
	}
}

// Validate rejects a filter the repository must not run: an inverted range, or
// a status outside the closed set. The status rule lives here as well as at
// the wire boundary so a non-HTTP caller cannot construct a filter whose
// predicate silently matches nothing (draft 010 F9).
func (f UsageFilter) Validate() error {
	if f.To.Before(f.From) {
		return NewValidationError("to must not be earlier than from")
	}
	if f.Status != "" && !f.Status.IsValid() {
		return NewValidationError("status must be one of success, error")
	}
	return nil
}
