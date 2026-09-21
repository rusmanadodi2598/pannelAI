// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/usage_filter_status_test.go
// @for       The status filter's closed set at the wire boundary (draft 010 F2).
// @uses      internal/domain, net/http, net/http/httptest, testing, time.
// @reason    Draft 010 F2: the status query parameter accepted values outside
//
//	the domain's closed set (success|error), and the repository's
//	"empty means unfiltered" predicate turned any unknown value into a
//	silently empty 200 instead of a 400. The panel already validates
//	this field as a Zod enum, so these cases pin the gateway to the
//	same set, including the benign values that must keep passing and
//	the trim rule that runs before the check.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-22
package schema

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestDecodeUsageFilter_StatusClosedSet covers the status filter at the
// boundary: every member of the closed set and the empty "unfiltered" value
// decode, a trailing space is trimmed before the check, and every other shape
// a caller can send is a 400 VALIDATION_ERROR rather than a query that
// silently matches nothing.
func TestDecodeUsageFilter_StatusClosedSet(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    domain.UsageStatus
		wantErr bool
	}{
		{name: "empty means unfiltered", raw: "", want: ""},
		{name: "success passes", raw: "success", want: domain.UsageStatusSuccess},
		{name: "error passes", raw: "error", want: domain.UsageStatusError},
		{name: "a trailing space is trimmed, then the member passes", raw: "success%20", want: domain.UsageStatusSuccess},
		{name: "unknown word is rejected", raw: "banana", wantErr: true},
		{name: "uppercase member is rejected: the set is case-sensitive", raw: "SUCCESS", wantErr: true},
		{name: "capitalized member is rejected", raw: "Success", wantErr: true},
		{name: "trimmed uppercase is still rejected", raw: "SUCCESS%20", wantErr: true},
		{name: "both members together are not a list", raw: "success,error", wantErr: true},
		{name: "a sql fragment is rejected as a value, not filtered out", raw: "'%20OR%201=1", wantErr: true},
		{name: "null literal is rejected", raw: "null", wantErr: true},
		{name: "a lone comma is rejected", raw: ",", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/v1/usage/records?status="+tc.raw, nil)
			query, err := DecodeUsageFilter(r)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("DecodeUsageFilter accepted status=%q, want VALIDATION_ERROR", tc.raw)
				}
				appErr := domain.AsAppError(err)
				if appErr.Code != "VALIDATION_ERROR" {
					t.Fatalf("error code = %s, want VALIDATION_ERROR", appErr.Code)
				}
				if appErr.HTTPStatus() != http.StatusBadRequest {
					t.Fatalf("HTTP status = %d, want 400", appErr.HTTPStatus())
				}
				return
			}
			if err != nil {
				t.Fatalf("DecodeUsageFilter(status=%q) error = %v", tc.raw, err)
			}
			if query.Status != tc.want {
				t.Fatalf("Status = %q, want %q", query.Status, tc.want)
			}
		})
	}
}

// TestDecodeUsageFilter_StatusFlowsToDomainFilter proves the accepted value
// survives the lowering into the domain filter, so rejecting the rest cannot
// have silently broken the members that must keep filtering.
func TestDecodeUsageFilter_StatusFlowsToDomainFilter(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		name string
		raw  string
		want domain.UsageStatus
	}{
		{name: "success reaches the domain filter", raw: "success", want: domain.UsageStatusSuccess},
		{name: "error reaches the domain filter", raw: "error", want: domain.UsageStatusError},
		{name: "empty stays unfiltered", raw: "", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/v1/usage/records?status="+tc.raw, nil)
			query, err := DecodeUsageFilter(r)
			if err != nil {
				t.Fatalf("DecodeUsageFilter(status=%q) error = %v", tc.raw, err)
			}
			filter := query.Filter(now)
			if filter.Status != tc.want {
				t.Fatalf("filter.Status = %q, want %q", filter.Status, tc.want)
			}
		})
	}
}

// TestDecodeLogFilter_InheritsStatusClosedSet proves the logs route, which
// reuses the usage decoder for its shared filters, inherits the same closed
// set instead of re-declaring one that can drift: one boundary, two routes.
func TestDecodeLogFilter_InheritsStatusClosedSet(t *testing.T) {
	rejected := httptest.NewRequest(http.MethodGet, "/api/v1/logs/requests?status=banana", nil)
	if _, err := DecodeLogFilter(rejected); err == nil {
		t.Fatalf("DecodeLogFilter accepted status=banana, want VALIDATION_ERROR")
	}
	accepted := httptest.NewRequest(http.MethodGet, "/api/v1/logs/requests?status=error", nil)
	query, err := DecodeLogFilter(accepted)
	if err != nil {
		t.Fatalf("DecodeLogFilter(status=error) error = %v", err)
	}
	if query.Status != domain.UsageStatusError {
		t.Fatalf("Status = %q, want error", query.Status)
	}
}
