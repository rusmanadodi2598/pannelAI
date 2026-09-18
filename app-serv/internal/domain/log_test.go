// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/log_test.go
// @for       Table-driven tests for capture, truncation, and retention
//
//	boundaries.
//
// @uses      testing, time.
// @reason    AGENTS.md §2.1 and §2.4 require the capture and retention rules to
//
//	be pinned at their boundaries, because each one decides whether a
//	byte is stored: an off-by-one at the capture cap silently stores a
//	payload nobody intended to keep.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-18
package domain

import (
	"strings"
	"testing"
	"time"
)

// TestTruncateBody covers the truncation boundary: exactly at the cap, one
// byte over, empty, and the degenerate caps.
func TestTruncateBody(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		maxBytes  int
		wantBody  string
		wantTrunc bool
	}{
		{"empty body", "", 16, "", false},
		{"under the cap", "hello", 16, "hello", false},
		{"exactly at the cap", "0123456789", 10, "0123456789", false},
		{"one byte over is cut", "0123456789A", 10, "0123456789" + TruncationMarker, true},
		{"far over the cap", strings.Repeat("x", 100), 8, "xxxxxxxx" + TruncationMarker, true},
		{"zero cap stores nothing", "hello", 0, "", false},
		{"negative cap stores nothing", "hello", -1, "", false},
		{"one byte cap", "hello", 1, "h" + TruncationMarker, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := TruncateBody(tc.body, tc.maxBytes)
			if got != tc.wantBody {
				t.Fatalf("TruncateBody(%q, %d) = %q, want %q", tc.body, tc.maxBytes, got, tc.wantBody)
			}
			if Truncated(got) != tc.wantTrunc {
				t.Fatalf("Truncated(%q) = %v, want %v", got, Truncated(got), tc.wantTrunc)
			}
		})
	}
}

// TestCaptureBodies covers the capture switch: off stores nothing whatever the
// bodies are, on applies the cap, and an unusable cap stores nothing rather
// than everything.
func TestCaptureBodies(t *testing.T) {
	cases := []struct {
		name         string
		enabled      bool
		maxBytes     int
		requestBody  string
		responseBody string
		wantRequest  string
		wantResponse string
	}{
		{"capture off drops both", false, 1024, "req", "resp", "", ""},
		{"capture off with a zero cap", false, 0, "req", "resp", "", ""},
		{"capture on keeps short bodies", true, 1024, "req", "resp", "req", "resp"},
		{"capture on truncates both", true, 4, "request", "response", "requ" + TruncationMarker, "resp" + TruncationMarker},
		{"capture on with a zero cap stores nothing", true, 0, "req", "resp", "", ""},
		{"capture on with empty bodies", true, 1024, "", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request, response := CaptureBodies(tc.requestBody, tc.responseBody, tc.enabled, tc.maxBytes)
			if request != tc.wantRequest {
				t.Fatalf("request body = %q, want %q", request, tc.wantRequest)
			}
			if response != tc.wantResponse {
				t.Fatalf("response body = %q, want %q", response, tc.wantResponse)
			}
		})
	}
}

// TestRetentionCutoff covers the retention boundary, including the case the
// settings validator would never let through.
func TestRetentionCutoff(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		days int
		want time.Time
	}{
		{"one day", 1, now.AddDate(0, 0, -1)},
		{"seven days is the documented default", 7, now.AddDate(0, 0, -7)},
		{"a year", 365, now.AddDate(0, 0, -365)},
		{"zero keeps only the boundary instant", 0, now},
		{"negative clamps to zero rather than deleting everything", -5, now},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RetentionCutoff(now, tc.days); !got.Equal(tc.want) {
				t.Fatalf("RetentionCutoff(%d) = %s, want %s", tc.days, got, tc.want)
			}
		})
	}
}

// TestNewRequestLog covers the validation the recorder's payload passes
// through, including the zero-timestamp default.
func TestNewRequestLog(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name    string
		in      RequestLogInput
		wantErr bool
	}{
		{"successful request", RequestLogInput{RequestID: "req_1", Status: RequestLogSuccess}, false},
		{"failed request", RequestLogInput{RequestID: "req_2", Status: RequestLogError, Error: "UPSTREAM_ERROR"}, false},
		{"zero timestamp defaults to now", RequestLogInput{RequestID: "req_3", Status: RequestLogSuccess}, false},
		{"missing request id", RequestLogInput{Status: RequestLogSuccess}, true},
		{"unknown status", RequestLogInput{RequestID: "req_4", Status: "pending"}, true},
		{"empty status", RequestLogInput{RequestID: "req_5"}, true},
		{"negative latency", RequestLogInput{RequestID: "req_6", Status: RequestLogSuccess, LatencyMS: -1}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry, err := NewRequestLog(tc.in, now)
			if tc.wantErr {
				if err == nil {
					t.Fatal("NewRequestLog = nil error, want a rejection")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewRequestLog error = %v", err)
			}
			if !entry.TS().Equal(now) && !tc.in.TS.IsZero() {
				t.Fatalf("TS = %s, want %s", entry.TS(), tc.in.TS.UTC())
			}
			if entry.TS().Location() != time.UTC {
				t.Fatalf("TS location = %s, want UTC", entry.TS().Location())
			}
		})
	}
}

// TestNewLogFilter covers the bounded default window so a log read can never be
// an unbounded scan.
func TestNewLogFilter(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	early := now.Add(-96 * time.Hour)
	late := now.Add(-2 * time.Hour)

	cases := []struct {
		name     string
		in       LogFilterInput
		wantFrom time.Time
		wantTo   time.Time
	}{
		{"no range uses the default window", LogFilterInput{}, now.Add(-DefaultLogWindow), now},
		{"explicit range is kept", LogFilterInput{From: &early, To: &late}, early, late},
		{"only from bounds the end at now", LogFilterInput{From: &early}, early, now},
		{"inverted range collapses to the end instant rather than erroring", LogFilterInput{From: &late, To: &early}, early, early},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			filter := NewLogFilter(tc.in, now)
			if !filter.From.Equal(tc.wantFrom) || !filter.To.Equal(tc.wantTo) {
				t.Fatalf("filter = [%s, %s], want [%s, %s]", filter.From, filter.To, tc.wantFrom, tc.wantTo)
			}
			if filter.To.Before(filter.From) {
				t.Fatalf("filter range is inverted: [%s, %s]", filter.From, filter.To)
			}
		})
	}
}
