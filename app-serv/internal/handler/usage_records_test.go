// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/usage_records_test.go
// @for       Table-driven HTTP tests for the §7.12 records list and the
//
//	single-request detail, including the captured-log join.
//
// @uses      encoding/json, internal/domain, net/http, net/http/httptest,
//
//	strings, testing.
//
// @reason    Draft 010 F1: the records routes are where the pagination meta
//
//	block and the capture-policy join live, and neither had a
//	request-to-response test. The per_page refusals are the F6 owner
//	decision (D3 = refuse) reaching the wire, and the detail cases pin
//	the honest distinction the panel renders: capture off, capture on
//	with a stored log, and a missing usage row answering 404.
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

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// TestUsageHandler_Records covers the paged read: the default page size, the
// meta block echoing what the caller asked for, the pagination refusals the
// shared decoder now makes (draft 010 F6), and a storage failure as 500.
func TestUsageHandler_Records(t *testing.T) {
	cases := []usageCase{
		{
			name:     "defaults answer one row with the default page size",
			want:     http.StatusOK,
			contains: []string{`"data":[{`, `"request_id":"req_010"`, `"per_page":25`, `"total":1`},
		},
		{
			name:     "the requested page and size are echoed in the meta block",
			query:    "?page=3&per_page=100",
			want:     http.StatusOK,
			contains: []string{`"page":3`, `"per_page":100`},
		},
		{
			name:     "an unknown status is refused, never a silent empty page",
			query:    "?status=banana",
			want:     http.StatusBadRequest,
			code:     "VALIDATION_ERROR",
			contains: []string{"status must be one of success, error"},
		},
		{
			name:     "page zero is refused",
			query:    "?page=0",
			want:     http.StatusBadRequest,
			code:     "VALIDATION_ERROR",
			contains: []string{"page must be at least 1"},
		},
		{
			name:     "per_page one over the cap is refused, not clamped",
			query:    "?per_page=101",
			want:     http.StatusBadRequest,
			code:     "VALIDATION_ERROR",
			contains: []string{"per_page must be at most 100"},
		},
		{
			name:     "a non-numeric per_page is refused",
			query:    "?per_page=abc",
			want:     http.StatusBadRequest,
			code:     "VALIDATION_ERROR",
			contains: []string{"per_page must be an integer"},
		},
		{
			name: "a storage failure is an internal error",
			want: http.StatusInternalServerError,
			code: "INTERNAL_ERROR",
			seed: func(r *stubUsageRepo) { r.listErr = errUsageRepo },
		},
	}
	runUsageCases(t, cases, func(h *UsageHandler, w http.ResponseWriter, r *http.Request) { h.Records(w, r) }, "/api/v1/usage/records")
}

// TestUsageHandler_RecordsForwardsPage proves the decoded page reaches the
// repository, which the meta block alone cannot prove (the response could echo
// the request while querying something else).
func TestUsageHandler_RecordsForwardsPage(t *testing.T) {
	h, repo := newUsageHandlerFixture(t, nil)
	rec := httptest.NewRecorder()
	h.Records(rec, usageRequest(http.MethodGet, "/api/v1/usage/records?page=2&per_page=50", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.lastQ.Page != 2 || repo.lastQ.PerPage != 50 {
		t.Fatalf("forwarded page query = %+v, want {2 50}", repo.lastQ)
	}
}

// TestUsageHandler_Detail covers the joined read as one behaviour with
// variations: capture off (no log block, capture_enabled false), capture on
// with a stored log (bodies present), capture on with the log gone (no log
// block but capture_enabled true, so the panel can say "capture is on, this
// request was never logged"), a missing usage row answering 404, an empty
// request id, and a storage failure as 500.
func TestUsageHandler_Detail(t *testing.T) {
	cases := []struct {
		name     string
		capture  bool
		withLog  bool
		id       string
		seed     func(*stubUsageRepo)
		want     int
		code     string
		contains []string
	}{
		{
			name:     "capture off answers the usage row without a log block",
			capture:  false,
			withLog:  true,
			id:       "req_010",
			want:     http.StatusOK,
			contains: []string{`"capture_enabled":false`, `"usage":{"id":"usg_010"`},
		},
		{
			name:     "capture on with a stored log answers the captured bodies",
			capture:  true,
			withLog:  true,
			id:       "req_010",
			want:     http.StatusOK,
			contains: []string{`"capture_enabled":true`, `"request_body":"captured in"`, `"response_body":"captured out"`},
		},
		{
			name:     "capture on with the log gone still reports capture on",
			capture:  true,
			withLog:  false,
			id:       "req_010",
			want:     http.StatusOK,
			contains: []string{`"capture_enabled":true`, `"request_id":"req_010"`},
		},
		{
			name:     "a missing usage row answers not found",
			capture:  true,
			withLog:  true,
			id:       "req_missing",
			want:     http.StatusNotFound,
			code:     "NOT_FOUND",
			contains: []string{"usage record not found"},
		},
		{
			name:     "an empty request id is a validation error",
			id:       "",
			want:     http.StatusBadRequest,
			code:     "VALIDATION_ERROR",
			contains: []string{"request_id is required"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var logs repository.RequestLogRepository = missingUsageLogs{}
			if tc.withLog {
				logs = &stubUsageLogs{entry: usageSeedLog(t)}
			}
			var h *UsageHandler
			if tc.capture {
				h, _ = newUsageHandlerFixtureCapturing(t, logs)
			} else {
				h, _ = newUsageHandlerFixture(t, logs)
			}
			rec := httptest.NewRecorder()
			path := "/api/v1/usage/records/" + tc.id
			h.Detail(rec, usageRequest(http.MethodGet, path, tc.id))
			assertUsageResponse(t, rec, tc.want, tc.code, tc.contains)
			if tc.code == "" {
				assertUsageLogBlock(t, rec.Body.String(), tc.capture && tc.withLog)
			}
		})
	}
}

// assertUsageLogBlock pins the capture rule the panel renders: a log block
// exists only when capture is on and a row was stored. "Capture is off" and
// "this request was never logged" are therefore different answers.
func assertUsageLogBlock(t *testing.T, body string, wantLog bool) {
	t.Helper()
	if got := strings.Contains(body, `"log":{`); got != wantLog {
		t.Fatalf("log block present = %v, want %v (body: %s)", got, wantLog, body)
	}
}

// TestUsageHandler_DetailStorageFailure pins that a repository failure on the
// detail read is an internal error, separate from the not-found mapping.
func TestUsageHandler_DetailStorageFailure(t *testing.T) {
	h, repo := newUsageHandlerFixture(t, nil)
	repo.mu.Lock()
	repo.getErr = errUsageRepo
	repo.mu.Unlock()
	rec := httptest.NewRecorder()
	h.Detail(rec, usageRequest(http.MethodGet, "/api/v1/usage/records/req_010", "req_010"))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not one JSON document: %v (body: %s)", err, rec.Body.String())
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body: %s)", rec.Code, rec.Body.String())
	}
	errObj, _ := body["error"].(map[string]any)
	if errObj == nil || errObj["code"] != "INTERNAL_ERROR" {
		t.Fatalf("body = %s, want the INTERNAL_ERROR envelope", rec.Body.String())
	}
}
