// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/usage_types_status_test.go
// @for       The usage and log filters' status as a domain value object
//
//	(draft 010 F2 + F9).
//
// @uses      testing, time.
// @reason    Draft 010 F9: UsageFilter.Status was a raw string, so an
//
//	unknown status could be constructed straight into the domain by
//	any non-HTTP caller and reach the repository's "empty means
//	unfiltered" predicate as a silently-empty query. These cases pin
//	the closed set at the constructor and at Validate for both filter
//	types, so the class is closed by the type, not by one boundary
//	checking one value.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-22
package domain

import (
	"testing"
	"time"
)

// statusFilterCases is the one closed-set table both filter types must answer
// identically, so the logs route cannot re-declare the set differently from
// the usage route (draft 010 F2/F9 shared rule).
var statusFilterCases = []struct {
	name    string
	status  string
	wantErr bool
}{
	{name: "empty means unfiltered", status: ""},
	{name: "success is a member", status: "success"},
	{name: "error is a member", status: "error"},
	{name: "unknown word is rejected", status: "banana", wantErr: true},
	{name: "uppercase member is rejected: the set is case-sensitive", status: "SUCCESS", wantErr: true},
	{name: "capitalized member is rejected", status: "Success", wantErr: true},
	{name: "both members together are not a list", status: "success,error", wantErr: true},
	{name: "sql fragment is rejected as a value", status: "' OR 1=1", wantErr: true},
	{name: "a null literal is not the empty value", status: "null", wantErr: true},
}

// TestNewUsageFilter_StatusClosedSet proves the usage filter's constructor
// keeps the closed set for every caller, HTTP or not: both members and the
// empty "unfiltered" value construct, and anything else is rejected at
// construction rather than reaching a query.
func TestNewUsageFilter_StatusClosedSet(t *testing.T) {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

	for _, tc := range statusFilterCases {
		t.Run(tc.name, func(t *testing.T) {
			filter := NewUsageFilter(UsageFilterInput{Status: UsageStatus(tc.status)}, now)
			err := filter.Validate()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Validate accepted Status=%q, want VALIDATION_ERROR", tc.status)
				}
				if appErr := AsAppError(err); appErr.Code != "VALIDATION_ERROR" {
					t.Fatalf("error code = %s, want VALIDATION_ERROR", appErr.Code)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate(Status=%q) error = %v", tc.status, err)
			}
			if string(filter.Status) != tc.status {
				t.Fatalf("Status = %q, want %q", filter.Status, tc.status)
			}
		})
	}
}

// TestNewLogFilter_StatusClosedSet proves the log filter holds the same closed
// set under its own status type, with the same table and the same semantics:
// the empty value stays unfiltered and both members keep filtering.
func TestNewLogFilter_StatusClosedSet(t *testing.T) {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

	for _, tc := range statusFilterCases {
		t.Run(tc.name, func(t *testing.T) {
			filter := NewLogFilter(LogFilterInput{Status: RequestLogStatus(tc.status)}, now)
			err := filter.Validate()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Validate accepted Status=%q, want VALIDATION_ERROR", tc.status)
				}
				if appErr := AsAppError(err); appErr.Code != "VALIDATION_ERROR" {
					t.Fatalf("error code = %s, want VALIDATION_ERROR", appErr.Code)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate(Status=%q) error = %v", tc.status, err)
			}
			if string(filter.Status) != tc.status {
				t.Fatalf("Status = %q, want %q", filter.Status, tc.status)
			}
		})
	}
}

// TestUsageFilter_ValidateStillRejectsInvertedRange pins that closing the
// status set did not disturb the range rule that was already there.
func TestUsageFilter_ValidateStillRejectsInvertedRange(t *testing.T) {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	early := now.Add(-2 * time.Hour)
	late := now.Add(-1 * time.Hour)
	filter := NewUsageFilter(UsageFilterInput{From: &late, To: &early}, now)
	if err := filter.Validate(); err == nil {
		t.Fatalf("Validate accepted an inverted range, want VALIDATION_ERROR")
	}
}
