// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/usage.go
// @for       The usage summary, timeseries, records, and request-detail routes
//
//	(SPEC-API-001 §7.12).
//
// @uses      internal/schema, internal/service, net/http, time.
// @reason    §7.12 offers four reads over one table; AGENTS.md §1.5 keeps
//
//	validation in the schema layer and aggregation in the service, so
//	this handler only decodes a bounded filter, calls, and encodes.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-18
package handler

import (
	"net/http"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// UsageHandler serves the /api/v1/usage routes (§7.12).
type UsageHandler struct {
	usage *service.UsageService
	clock func() time.Time
}

// NewUsageHandler validates deps and returns the handler.
func NewUsageHandler(usage *service.UsageService) *UsageHandler {
	return &UsageHandler{usage: usage, clock: time.Now}
}

// Summary serves GET /api/v1/usage/summary. The group breakdown is optional, so
// a caller asking for totals alone gets the same totals block with no groups.
func (h *UsageHandler) Summary(w http.ResponseWriter, r *http.Request) {
	query, err := schema.DecodeUsageFilter(r)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	filter := query.Filter(h.clock())

	var groupBy domain.UsageGroupBy
	if query.GroupBy != "" {
		parsed, err := domain.ParseUsageGroupBy(query.GroupBy)
		if err != nil {
			schema.WriteError(w, err)
			return
		}
		groupBy = parsed
	}

	totals, groups, err := h.usage.Summary(r.Context(), filter, groupBy)
	if err != nil {
		schema.WriteError(w, err)
		return
	}

	resp := schema.UsageSummaryResponse{
		From:    schema.Timestamp(filter.From),
		To:      schema.Timestamp(filter.To),
		GroupBy: query.GroupBy,
		Totals:  schema.UsageTotalsResponseFrom(totals),
		Groups:  make([]schema.UsageGroupResponse, 0, len(groups)),
	}
	for _, group := range groups {
		resp.Groups = append(resp.Groups, schema.UsageGroupResponse{
			Key:    group.Key,
			Totals: schema.UsageTotalsResponseFrom(group.Totals),
		})
	}
	schema.WriteJSON(w, http.StatusOK, resp)
}

// Timeseries serves GET /api/v1/usage/timeseries. The granularity defaults to
// hour, which is the finer of the two the contract offers and therefore loses
// no information when the caller does not choose.
func (h *UsageHandler) Timeseries(w http.ResponseWriter, r *http.Request) {
	query, err := schema.DecodeUsageFilter(r)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	granularity := domain.UsageHourly
	if query.Granularity != "" {
		parsed, err := domain.ParseUsageGranularity(query.Granularity)
		if err != nil {
			schema.WriteError(w, err)
			return
		}
		granularity = parsed
	}

	filter := query.Filter(h.clock())
	buckets, err := h.usage.Timeseries(r.Context(), filter, granularity)
	if err != nil {
		schema.WriteError(w, err)
		return
	}

	resp := schema.UsageTimeseriesResponse{
		Granularity: string(granularity),
		From:        schema.Timestamp(filter.From),
		To:          schema.Timestamp(filter.To),
		Buckets:     make([]schema.UsageBucketResponse, 0, len(buckets)),
	}
	for _, bucket := range buckets {
		resp.Buckets = append(resp.Buckets, schema.UsageBucketResponse{
			Bucket: schema.Timestamp(bucket.Bucket),
			Totals: schema.UsageTotalsResponseFrom(bucket.Totals),
		})
	}
	schema.WriteJSON(w, http.StatusOK, resp)
}

// Records serves GET /api/v1/usage/records, paginated and always bounded.
func (h *UsageHandler) Records(w http.ResponseWriter, r *http.Request) {
	query, err := schema.DecodeUsageFilter(r)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	page, perPage, err := schema.DecodePage(r)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	records, total, err := h.usage.Records(r.Context(), query.Filter(h.clock()), page, perPage)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	resp := schema.UsageRecordList{
		Data: make([]schema.UsageRecordResponse, 0, len(records)),
		Meta: schema.Page{Page: page, PerPage: perPage, Total: total},
	}
	for _, record := range records {
		resp.Data = append(resp.Data, schema.UsageRecordResponseFrom(record))
	}
	schema.WriteJSON(w, http.StatusOK, resp)
}

// Detail serves GET /api/v1/usage/records/{request_id}, joining the captured
// log when one exists.
func (h *UsageHandler) Detail(w http.ResponseWriter, r *http.Request) {
	requestID := r.PathValue("request_id")
	if requestID == "" {
		schema.WriteError(w, domain.NewValidationError("request_id is required"))
		return
	}
	detail, err := h.usage.Detail(r.Context(), requestID)
	if err != nil {
		schema.WriteError(w, err)
		return
	}

	var captured *schema.LogDetailResponse
	if detail.Log != nil {
		mapped := schema.LogDetailResponseFrom(*detail.Log, detail.CaptureEnabled, detail.CaptureBody)
		captured = &mapped
	}
	schema.WriteJSON(w, http.StatusOK, schema.UsageRecordDetailFrom(
		schema.UsageRecordResponseFrom(detail.Usage), detail.CaptureEnabled, detail.CaptureBody, captured))
}
