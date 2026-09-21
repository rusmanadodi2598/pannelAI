// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/log.go
// @for       The RequestLog aggregate and the capture, truncation, and
//
//	retention rules that govern it.
//
// @uses      internal/domain (AppError constructors), strings, time.
// @reason    SPEC-API-001 §7.13 makes body capture conditional on a setting and
//
//	truncated to a configured size, and makes retention delete rows
//	older than a configured age. Both rules decide whether a byte is
//	stored, so they live here where a unit test can exercise the
//	boundary with no database, rather than inside the repository that
//	happens to write the row.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-18
package domain

import "time"

// TruncationMarker is appended to a captured body that was cut short, so the
// reader learns the payload was truncated at the point it ends instead of
// assuming the upstream sent a short body (SPEC-UI §6.11).
const TruncationMarker = "\n...[truncated]"

// DefaultLogWindow is how far back a log read looks when the caller sends no
// range, matching the usage default so both surfaces answer the same question
// the same way (AGENTS.md §1.7: no unbounded query).
const DefaultLogWindow = 24 * time.Hour

// RequestLogStatus is how a logged request ended. The set mirrors the values
// the data plane writes and the migration's rows carry.
type RequestLogStatus string

const (
	// RequestLogSuccess is a request that produced a response.
	RequestLogSuccess RequestLogStatus = "success"
	// RequestLogError is a request that failed.
	RequestLogError RequestLogStatus = "error"
)

// IsValid reports whether the status is one this aggregate stores.

func (s RequestLogStatus) IsValid() bool {
	return s == RequestLogSuccess || s == RequestLogError
}

// Validate rejects a status outside the closed set, so a non-HTTP caller
// cannot hand the repository a predicate that silently matches nothing (draft
// 010 F2/F9). The range is not checked here: NewLogFilter already collapses an
// inverted range to the end instant, which the constructor's test pins as the
// intended behavior for logs.
func (f LogFilter) Validate() error {
	if f.Status != "" && !f.Status.IsValid() {
		return NewValidationError("status must be one of success, error")
	}
	return nil
}

// RequestLog is one request's durable log row. It carries the bodies only when
// capture was on, so an empty body here means "not captured", which is why the
// read side reports the capture setting alongside it.
type RequestLog struct {
	requestID    string
	ts           time.Time
	gatewayKeyID string
	endpointID   string
	providerID   string
	model        string
	status       RequestLogStatus
	latencyMS    int64
	requestBody  string
	responseBody string
	errText      string
}

// RequestLogInput is the recorder's payload. Bodies arrive already truncated by
// CaptureBodies, so this type carries no truncation rule of its own.
type RequestLogInput struct {
	RequestID    string
	TS           time.Time
	GatewayKeyID string
	EndpointID   string
	ProviderID   string
	Model        string
	Status       RequestLogStatus
	LatencyMS    int64
	RequestBody  string
	ResponseBody string
	Error        string
}

// NewRequestLog validates and builds a log row. A missing request id or an
// unknown status is rejected: the id is the primary key and the status is what
// every filter reads, so a row without either is unusable.
func NewRequestLog(in RequestLogInput, now time.Time) (RequestLog, error) {
	if in.RequestID == "" {
		return RequestLog{}, NewValidationError("request_id is required")
	}
	if !in.Status.IsValid() {
		return RequestLog{}, NewValidationError("status must be success or error")
	}
	if in.LatencyMS < 0 {
		return RequestLog{}, NewValidationError("latency_ms must not be negative")
	}
	ts := in.TS.UTC()
	if ts.IsZero() {
		ts = now.UTC()
	}
	return RequestLog{
		requestID:    in.RequestID,
		ts:           ts,
		gatewayKeyID: in.GatewayKeyID,
		endpointID:   in.EndpointID,
		providerID:   in.ProviderID,
		model:        in.Model,
		status:       in.Status,
		latencyMS:    in.LatencyMS,
		requestBody:  in.RequestBody,
		responseBody: in.ResponseBody,
		errText:      in.Error,
	}, nil
}

// RehydrateRequestLog rebuilds a stored row for the repository load path.
func RehydrateRequestLog(requestID string, ts time.Time, gatewayKeyID, endpointID, providerID, model string, status RequestLogStatus, latencyMS int64, requestBody, responseBody, errText string) RequestLog {
	return RequestLog{
		requestID:    requestID,
		ts:           ts.UTC(),
		gatewayKeyID: gatewayKeyID,
		endpointID:   endpointID,
		providerID:   providerID,
		model:        model,
		status:       status,
		latencyMS:    latencyMS,
		requestBody:  requestBody,
		responseBody: responseBody,
		errText:      errText,
	}
}

// Accessors expose the row without allowing mutation.
func (l RequestLog) RequestID() string           { return l.requestID }
func (l RequestLog) TS() time.Time               { return l.ts }
func (l RequestLog) GatewayKeyID() string        { return l.gatewayKeyID }
func (l RequestLog) EndpointID() string          { return l.endpointID }
func (l RequestLog) ProviderID() string          { return l.providerID }
func (l RequestLog) Model() string               { return l.model }
func (l RequestLog) Status() string              { return string(l.status) }
func (l RequestLog) LogStatus() RequestLogStatus { return l.status }
func (l RequestLog) LatencyMS() int64            { return l.latencyMS }
func (l RequestLog) RequestBody() string         { return l.requestBody }
func (l RequestLog) ResponseBody() string        { return l.responseBody }
func (l RequestLog) Error() string               { return l.errText }

// HasBodies reports whether either body was captured.
func (l RequestLog) HasBodies() bool {
	return l.requestBody != "" || l.responseBody != ""
}

// LogFilter narrows a request-log read. From and To are always populated, so a
// log query can never be an unbounded scan (AGENTS.md §1.7). Status is the
// domain value object, so a value outside the closed set cannot be constructed
// into a filter (draft 010 F2/F9: the logs route shares the usage decoder, so
// it holds the same rule).
type LogFilter struct {
	From       time.Time
	To         time.Time
	Status     RequestLogStatus
	EndpointID string
	Model      string
	GatewayKey string
	Query      string
}

// LogFilterInput is the constructor input, where a nil range means "use the
// default window".
type LogFilterInput struct {
	From       *time.Time
	To         *time.Time
	Status     RequestLogStatus
	EndpointID string
	Model      string
	GatewayKey string
	Query      string
}

// NewLogFilter resolves a partial filter into a bounded one.
func NewLogFilter(in LogFilterInput, now time.Time) LogFilter {
	to := now.UTC()
	if in.To != nil {
		to = in.To.UTC()
	}
	from := to.Add(-DefaultLogWindow)
	if in.From != nil {
		from = in.From.UTC()
	}
	if to.Before(from) {
		from = to
	}
	return LogFilter{
		From:       from,
		To:         to,
		Status:     in.Status,
		EndpointID: in.EndpointID,
		Model:      in.Model,
		GatewayKey: in.GatewayKey,
		Query:      in.Query,
	}
}
