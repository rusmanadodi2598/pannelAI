//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/log_query_scope_integration_test.go
// @for       The log free-text q scope against a real server (draft 010 F8).
// @uses      internal/domain, internal/repository, context, testing, time.
// @reason    Draft 010 F8: the logs route shares the usage decoder, so the same
//
//	panel placeholder promised a request id the predicate did not
//	search. The log row carries error text where a usage row carries an
//	error code, so its scope is `request_id OR error OR model`, and only
//	a real server can prove the OR survives the nullable error column.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-22
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// TestLogRepository_QueryScopeMatchesIdentityAndError seeds log rows whose
// request id, error text, and model differ, then proves each of the three
// fields the contract now names is searchable, that the match is a
// case-insensitive substring, that an empty filter is still "everything", and
// that a row with no error text still matches on its identity.
func TestLogRepository_QueryScopeMatchesIdentityAndError(t *testing.T) {
	repo := newLogRepo(t)
	ctx := context.Background()
	base := time.Now().UTC().Add(-time.Hour)

	seedScopeLog(t, repo, "req_alpha", base, "gpt-4o", "upstream refused the credential")
	seedScopeLog(t, repo, "req_beta", base.Add(time.Minute), "claude-3", "")
	seedScopeLog(t, repo, "req_gamma", base.Add(2*time.Minute), "o3-mini", "MODEL_NOT_FOUND")

	filter := domain.NewLogFilter(domain.LogFilterInput{
		From: ptrTime(base.Add(-time.Minute)),
		To:   ptrTime(base.Add(time.Hour)),
	}, time.Now())

	cases := []struct {
		name      string
		query     string
		wantRows  int
		wantFirst string
	}{
		{name: "an empty filter reads every row", query: "", wantRows: 3},
		{name: "the request id is searchable", query: "req_beta", wantRows: 1, wantFirst: "req_beta"},
		{name: "the request id matches as a substring", query: "req_", wantRows: 3},
		{name: "the error text is searchable", query: "credential", wantRows: 1, wantFirst: "req_alpha"},
		{name: "the error text matches case-insensitively", query: "UpStReAm", wantRows: 1, wantFirst: "req_alpha"},
		{name: "the error code in the text is searchable", query: "MODEL_NOT_FOUND", wantRows: 1, wantFirst: "req_gamma"},
		{name: "the model is searchable", query: "claude", wantRows: 1, wantFirst: "req_beta"},
		{name: "the model matches case-insensitively", query: "GPT", wantRows: 1, wantFirst: "req_alpha"},
		{name: "a value matching nothing answers zero rows", query: "no-such-request", wantRows: 0},
		{name: "a row with no error text still matches on its id", query: "req_beta", wantRows: 1, wantFirst: "req_beta"},
		{name: "an SQL fragment stays a literal and matches nothing", query: "' OR 1=1 --", wantRows: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scoped := filter
			scoped.Query = tc.query
			entries, total, err := repo.List(ctx, scoped, repository.PageQuery{Page: 1, PerPage: 100})
			if err != nil {
				t.Fatalf("List error = %v", err)
			}
			if len(entries) != tc.wantRows || total != int64(tc.wantRows) {
				t.Fatalf("rows = %d, total = %d, want %d and %d", len(entries), total, tc.wantRows, tc.wantRows)
			}
			if tc.wantFirst != "" && entries[0].RequestID() != tc.wantFirst {
				t.Fatalf("first row request id = %q, want %q", entries[0].RequestID(), tc.wantFirst)
			}
		})
	}
}

// seedScopeLog inserts one log row with the model and error text the scope
// cases distinguish.
func seedScopeLog(t *testing.T, repo *LogRepository, requestID string, ts time.Time, model, errText string) {
	t.Helper()
	entry, err := domain.NewRequestLog(domain.RequestLogInput{
		RequestID: requestID,
		TS:        ts,
		Model:     model,
		Status:    domain.RequestLogSuccess,
		LatencyMS: 42,
		Error:     errText,
	}, time.Now())
	if err != nil {
		t.Fatalf("building log %s: %v", requestID, err)
	}
	if err := repo.Insert(context.Background(), entry); err != nil {
		t.Fatalf("inserting log %s: %v", requestID, err)
	}
}
