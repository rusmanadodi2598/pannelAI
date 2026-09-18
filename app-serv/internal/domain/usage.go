// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/usage.go
// @for       The UsageRecord aggregate: one served request's tokens, cost, and
//
//	latency, as recorded after the upstream call completes.
//
// @uses      internal/domain (ULID, Decimal, AppError constructors), time.
// @reason    SPEC-API-001 §7.12 makes this the accounting row the summary,
//
//	timeseries, records, and rollup are all read from, so its shape
//	and its invariants belong in one aggregate: a negative token count
//	or an unparseable cost must be rejected at construction, because a
//	bad row silently skews every aggregate built on it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-18
package domain

import "time"

// UsageStatus is how a recorded request ended. The set is closed and mirrors
// the values the data plane writes.
type UsageStatus string

const (
	// UsageStatusSuccess is a request that produced a response.
	UsageStatusSuccess UsageStatus = "success"
	// UsageStatusError is a request that failed at or after the upstream call.
	UsageStatusError UsageStatus = "error"
)

// IsValid reports whether the status is one this aggregate records.
func (s UsageStatus) IsValid() bool {
	return s == UsageStatusSuccess || s == UsageStatusError
}

// Success reports whether the recorded request completed.
func (s UsageStatus) Success() bool { return s == UsageStatusSuccess }

// UsageRecordIDPrefix is the type prefix for a usage row (SPEC-API-001 §4).
const UsageRecordIDPrefix = "usg_"

// UsageRecordInput is the reporter's payload. It is a plain struct rather than
// a constructor with fourteen positional parameters, because the data plane
// fills it field by field as it reads an upstream response and a positional
// call of that width is where an argument order mistake hides.
type UsageRecordInput struct {
	RequestID        string
	TS               time.Time
	EndpointID       string
	ProviderID       string
	Model            string
	Combo            string
	GatewayKeyID     string
	TokensIn         int64
	TokensOut        int64
	TokensCacheRead  int64
	TokensCacheWrite int64
	CostUSD          string
	LatencyMS        int64
	Status           UsageStatus
	ErrorCode        string
}

// UsageRecord is one accounting row. Fields are unexported on purpose
// (AGENTS.md §2.2): the aggregate is constructed once and read, not mutated.
type UsageRecord struct {
	id               string
	requestID        string
	ts               time.Time
	endpointID       string
	providerID       string
	model            string
	combo            string
	gatewayKeyID     string
	tokensIn         int64
	tokensOut        int64
	tokensCacheRead  int64
	tokensCacheWrite int64
	costUSD          string
	latencyMS        int64
	status           UsageStatus
	errorCode        string
}

// NewUsageRecord validates and builds a record. A missing request id, provider,
// or model is rejected: each is a column the read side filters and groups on,
// so a row without one is invisible to every query that matters. Negative
// counters and an unparseable cost are rejected for the same reason.
func NewUsageRecord(in UsageRecordInput, id string, now time.Time) (UsageRecord, error) {
	if in.RequestID == "" {
		return UsageRecord{}, NewValidationError("request_id is required")
	}
	if in.ProviderID == "" {
		return UsageRecord{}, NewValidationError("provider_id is required")
	}
	if in.Model == "" {
		return UsageRecord{}, NewValidationError("model is required")
	}
	if !in.Status.IsValid() {
		return UsageRecord{}, NewValidationError("status must be success or error")
	}
	for name, value := range map[string]int64{
		"tokens_in":          in.TokensIn,
		"tokens_out":         in.TokensOut,
		"tokens_cache_read":  in.TokensCacheRead,
		"tokens_cache_write": in.TokensCacheWrite,
		"latency_ms":         in.LatencyMS,
	} {
		if value < 0 {
			return UsageRecord{}, NewValidationError(name + " must not be negative")
		}
	}
	cost, err := ParseDecimal(in.CostUSD)
	if err != nil {
		return UsageRecord{}, err
	}
	if cost.IsNegative() {
		return UsageRecord{}, NewValidationError("cost_usd must not be negative")
	}

	ts := in.TS.UTC()
	if ts.IsZero() {
		ts = now.UTC()
	}
	if id == "" {
		id = UsageRecordIDPrefix + NewULID(ts)
	}
	return UsageRecord{
		id:               id,
		requestID:        in.RequestID,
		ts:               ts,
		endpointID:       in.EndpointID,
		providerID:       in.ProviderID,
		model:            in.Model,
		combo:            in.Combo,
		gatewayKeyID:     in.GatewayKeyID,
		tokensIn:         in.TokensIn,
		tokensOut:        in.TokensOut,
		tokensCacheRead:  in.TokensCacheRead,
		tokensCacheWrite: in.TokensCacheWrite,
		costUSD:          cost.String(),
		latencyMS:        in.LatencyMS,
		status:           in.Status,
		errorCode:        in.ErrorCode,
	}, nil
}

// RehydrateUsageRecord rebuilds a stored row. It is for the repository load
// path only; never use it to record a request.
func RehydrateUsageRecord(
	id, requestID string,
	ts time.Time,
	endpointID, providerID, gatewayKeyID, model, combo string,
	tokensIn, tokensOut, tokensCacheRead, tokensCacheWrite int64,
	costUSD string,
	latencyMS int64,
	status, errorCode string,
) UsageRecord {
	return UsageRecord{
		id:               id,
		requestID:        requestID,
		ts:               ts.UTC(),
		endpointID:       endpointID,
		providerID:       providerID,
		gatewayKeyID:     gatewayKeyID,
		model:            model,
		combo:            combo,
		tokensIn:         tokensIn,
		tokensOut:        tokensOut,
		tokensCacheRead:  tokensCacheRead,
		tokensCacheWrite: tokensCacheWrite,
		costUSD:          costUSD,
		latencyMS:        latencyMS,
		status:           UsageStatus(status),
		errorCode:        errorCode,
	}
}

// Accessors expose the state the read side needs without allowing mutation.
func (r UsageRecord) ID() string               { return r.id }
func (r UsageRecord) RequestID() string        { return r.requestID }
func (r UsageRecord) TS() time.Time            { return r.ts }
func (r UsageRecord) EndpointID() string       { return r.endpointID }
func (r UsageRecord) ProviderID() string       { return r.providerID }
func (r UsageRecord) GatewayKeyID() string     { return r.gatewayKeyID }
func (r UsageRecord) Model() string            { return r.model }
func (r UsageRecord) Combo() string            { return r.combo }
func (r UsageRecord) TokensIn() int64          { return r.tokensIn }
func (r UsageRecord) TokensOut() int64         { return r.tokensOut }
func (r UsageRecord) TokensCacheRead() int64   { return r.tokensCacheRead }
func (r UsageRecord) TokensCacheWrite() int64  { return r.tokensCacheWrite }
func (r UsageRecord) CostUSD() string          { return r.costUSD }
func (r UsageRecord) LatencyMS() int64         { return r.latencyMS }
func (r UsageRecord) Status() string           { return string(r.status) }
func (r UsageRecord) UsageStatus() UsageStatus { return r.status }
func (r UsageRecord) ErrorCode() string        { return r.errorCode }

// TotalTokens is every token the request consumed, including cache traffic.
// The panel shows the four counters separately, but the monthly token cap is
// compared against one number, and this is that number.
func (r UsageRecord) TotalTokens() int64 {
	return r.tokensIn + r.tokensOut + r.tokensCacheRead + r.tokensCacheWrite
}

// Totals projects the record into the aggregate shape the read side sums, so
// an in-memory aggregation and a SQL rollup cannot disagree about a field.
func (r UsageRecord) Totals() UsageTotals {
	totals := UsageTotals{
		Requests:         1,
		TokensIn:         r.tokensIn,
		TokensOut:        r.tokensOut,
		TokensCacheRead:  r.tokensCacheRead,
		TokensCacheWrite: r.tokensCacheWrite,
		CostUSD:          r.costUSD,
		LatencyMS:        r.latencyMS,
		LatencyP50MS:     r.latencyMS,
		LatencyP95MS:     r.latencyMS,
	}
	if !r.status.Success() {
		totals.ErrorCount = 1
	}
	return totals
}
