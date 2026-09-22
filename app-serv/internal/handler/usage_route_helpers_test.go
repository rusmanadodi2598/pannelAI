// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/usage_route_helpers_test.go
// @for       The shared table runner, assertion path, and pinned window for the
//
//	§7.12 usage route tests.
//
// @uses      encoding/json, net/http, net/http/httptest, strings, testing, time.
// @reason    The summary/timeseries tests and the records/detail tests ask the
//
//	same questions of a response, and draft 010 F1's tables grew both
//	files past the AGENTS.md §1.1 warning line. Keeping the walkers
//	here means a route file reads as cases rather than as plumbing,
//	and the two files stay inside the budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-22
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// usageWindow pins the range a forwarding assertion compares, so the tests
// speak instants rather than formatted strings.
var usageWindow = [2]time.Time{
	time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC),
	time.Date(2026, 9, 22, 8, 0, 0, 0, time.UTC),
}

// rfc3339 renders one instant the way the wire carries it.
func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// usageCase is one table row: the query, the expected status, the envelope code
// when a refusal is expected, the body substrings the row pins, and an optional
// seed that breaks the read before the request runs.
type usageCase struct {
	name     string
	query    string
	want     int
	code     string
	contains []string
	seed     func(*stubUsageRepo)
}

// runUsageCases drives one handler method through every row of a table, so the
// request setup and assertion path are written once.
func runUsageCases(t *testing.T, cases []usageCase, call func(*UsageHandler, http.ResponseWriter, *http.Request), target string) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, repo := newUsageHandlerFixture(t, nil)
			if tc.seed != nil {
				tc.seed(repo)
			}
			rec := httptest.NewRecorder()
			call(h, rec, usageRequest(http.MethodGet, target+tc.query, ""))
			assertUsageResponse(t, rec, tc.want, tc.code, tc.contains)
		})
	}
}

// assertUsageResponse checks the status, the envelope code when one is
// expected, and the body substrings a row pins, so every table row shares one
// assertion path.
func assertUsageResponse(t *testing.T, rec *httptest.ResponseRecorder, want int, code string, contains []string) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, want, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not one JSON document: %v (body: %s)", err, rec.Body.String())
	}
	if code != "" {
		errObj, _ := body["error"].(map[string]any)
		if errObj == nil {
			t.Fatalf("body = %s, want the error envelope", rec.Body.String())
		}
		if got, _ := errObj["code"].(string); got != code {
			t.Fatalf("code = %q, want %q", got, code)
		}
		if msg, _ := errObj["message"].(string); msg == "" {
			t.Fatal("the error envelope carries no message")
		}
	}
	for _, want := range contains {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("body = %s, want it to contain %q", rec.Body.String(), want)
		}
	}
}
