// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/log.go
// @for       The request-log list, detail, purge, and console-buffer contracts.
// @uses      internal/domain (LogFilter, RequestLog) and the shared usage filter.
// @reason    SPEC-API-001 §7.13 fixes the log filters, the captured-body detail,
//
//	and the console buffer shape; §2.4 requires them typed before the
//	handler, and the detail response has to distinguish "capture is
//	off" from "no bodies were stored" for the panel to be honest.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-18
package schema

import (
	"net/http"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// LogFilterQuery is the decoded log filter set (SPEC-API-001 §7.13). It reuses
// the usage filter decoder for the shared fields so one request cannot be
// filtered two different ways depending on which route read it.
type LogFilterQuery struct {
	UsageFilterQuery
}

// DecodeLogFilter reads and validates the log filters.
func DecodeLogFilter(r *http.Request) (LogFilterQuery, error) {
	decoded, err := DecodeUsageFilter(r)
	if err != nil {
		return LogFilterQuery{}, err
	}
	return LogFilterQuery{UsageFilterQuery: decoded}, nil
}

// Filter lowers the validated query into the domain filter with the same
// bounded default window the usage reads use.
func (q LogFilterQuery) Filter(now time.Time) domain.LogFilter {
	return domain.NewLogFilter(domain.LogFilterInput{
		From:       q.From,
		To:         q.To,
		Status:     q.Status,
		EndpointID: q.EndpointID,
		Model:      q.Model,
		Query:      q.Q,
		GatewayKey: q.GatewayKey,
	}, now)
}

// LogRecordResponse is one request-log row without its bodies, which is what a
// list needs and what keeps a page from carrying megabytes of payload.
type LogRecordResponse struct {
	RequestID    string `json:"request_id"`
	TS           string `json:"ts"`
	GatewayKeyID string `json:"gateway_key_id,omitempty"`
	EndpointID   string `json:"endpoint_id,omitempty"`
	ProviderID   string `json:"provider_id,omitempty"`
	Model        string `json:"model,omitempty"`
	Status       string `json:"status"`
	LatencyMS    int64  `json:"latency_ms"`
	Error        string `json:"error,omitempty"`
	HasBodies    bool   `json:"has_bodies"`
}

// LogList wraps a page of log rows with the pagination meta block.
type LogList struct {
	Data []LogRecordResponse `json:"data"`
	Meta Page                `json:"meta"`
}

// LogDetailResponse is the full captured request and response. Bodies are
// omitted when capture is off, and each body carries the truncation marker the
// capture path wrote, so a truncated body says so at the point it was cut
// rather than ending silently (SPEC-UI §6.11).
type LogDetailResponse struct {
	RequestID           string `json:"request_id"`
	TS                  string `json:"ts"`
	GatewayKeyID        string `json:"gateway_key_id,omitempty"`
	EndpointID          string `json:"endpoint_id,omitempty"`
	ProviderID          string `json:"provider_id,omitempty"`
	Model               string `json:"model,omitempty"`
	Status              string `json:"status"`
	LatencyMS           int64  `json:"latency_ms"`
	Error               string `json:"error,omitempty"`
	CaptureEnabled      bool   `json:"capture_enabled"`
	CaptureBodyMaxBytes int    `json:"capture_body_max_bytes"`
	RequestBody         string `json:"request_body,omitempty"`
	ResponseBody        string `json:"response_body,omitempty"`
}

// LogPurgeResponse reports how many rows a purge removed.
type LogPurgeResponse struct {
	Deleted int64 `json:"deleted"`
}

// ConsoleResponse is the body of GET /api/v1/logs/console. Lines are oldest
// first so the panel appends them without reversing, and MaxRecords is the
// bound in force so the panel can say why older lines are gone.
type ConsoleResponse struct {
	Lines      []string `json:"lines"`
	MaxRecords int      `json:"max_records"`
}

// LogRecordResponseFrom maps one stored log row onto the wire shape.
func LogRecordResponseFrom(entry domain.RequestLog) LogRecordResponse {
	return LogRecordResponse{
		RequestID:    entry.RequestID(),
		TS:           Timestamp(entry.TS()),
		GatewayKeyID: entry.GatewayKeyID(),
		EndpointID:   entry.EndpointID(),
		ProviderID:   entry.ProviderID(),
		Model:        entry.Model(),
		Status:       entry.Status(),
		LatencyMS:    entry.LatencyMS(),
		Error:        entry.Error(),
		HasBodies:    entry.RequestBody() != "" || entry.ResponseBody() != "",
	}
}

// LogDetailResponseFrom maps one stored log row and the capture settings that
// governed it onto the wire shape. The bodies are dropped entirely when capture
// is off, so a stale body written before capture was disabled cannot be read as
// if it were current.
func LogDetailResponseFrom(entry domain.RequestLog, captureEnabled bool, maxBytes int) LogDetailResponse {
	resp := LogDetailResponse{
		RequestID:           entry.RequestID(),
		TS:                  Timestamp(entry.TS()),
		GatewayKeyID:        entry.GatewayKeyID(),
		EndpointID:          entry.EndpointID(),
		ProviderID:          entry.ProviderID(),
		Model:               entry.Model(),
		Status:              entry.Status(),
		LatencyMS:           entry.LatencyMS(),
		Error:               entry.Error(),
		CaptureEnabled:      captureEnabled,
		CaptureBodyMaxBytes: maxBytes,
	}
	if captureEnabled {
		resp.RequestBody = entry.RequestBody()
		resp.ResponseBody = entry.ResponseBody()
	}
	return resp
}

// ConsoleResponseFrom maps the stored console lines onto the wire shape.
func ConsoleResponseFrom(lines []string, maxRecords int) ConsoleResponse {
	if lines == nil {
		lines = []string{}
	}
	return ConsoleResponse{Lines: lines, MaxRecords: maxRecords}
}
