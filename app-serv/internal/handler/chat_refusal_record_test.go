// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/chat_refusal_record_test.go
// @for       The request-log row a chat call the schema refused must leave.
// @uses      context, encoding/json, net/http, net/http/httptest, strings, sync, testing, internal/domain, internal/service.
// @reason    Draft 034 F4 measured a schema refusal writing no row at all: a routing failure is recorded in request_logs, a schema failure is not, so the panel's error-rate card and request list never see a client that repeats a malformed call. The refusal's row is pinned here at the HTTP boundary over the real service, alongside the negative side (a served call leaves no refusal row) and the register G17 rule it must not break (no usage row for a call that reached no attempt).
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-09-27
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// recordingLogs captures the request-log rows a served or refused request
// writes, the way recordingUsage captures the usage half.
type recordingLogs struct {
	mu     sync.Mutex
	inputs []domain.RequestLogInput
}

func (r *recordingLogs) Record(_ context.Context, in domain.RequestLogInput) (domain.RequestLog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inputs = append(r.inputs, in)
	return domain.NewRequestLog(in, testInstant())
}

func (r *recordingLogs) rows() []domain.RequestLogInput {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.RequestLogInput, len(r.inputs))
	copy(out, r.inputs)
	return out
}

var _ service.RequestLogRecorder = (*recordingLogs)(nil)

// TestChatCompletionsHTTP_RefusalRecorded pins that a request the
// schema refuses after authentication still leaves its request-log row, with
// the same code the client was served, and still leaves no usage row.
func TestChatCompletionsHTTP_RefusalRecorded(t *testing.T) {
	const effortProbe = `{"model":"test/model","messages":[{"role":"user","content":"hello"}],"reasoning_effort":"banana"}`

	cases := []struct {
		name          string
		body          string
		wantStatus    int
		wantLogRows   int
		wantModel     string
		wantLogStatus domain.RequestLogStatus
		wantUsageRows int
	}{
		{
			// The probe shape: an effort word the validator
			// refuses, after the decoder named the model, so the row can carry
			// it. The value "none" was once refused and became a served word
			// when the vocabulary widened to the engine's own set, so the pin
			// uses a word the vocabulary still refuses.
			name:          "a refusal the reasoning vocabulary rejects is recorded with its model",
			body:          effortProbe,
			wantStatus:    http.StatusBadRequest,
			wantLogRows:   1,
			wantModel:     "test/model",
			wantLogStatus: domain.RequestLogError,
		},
		{
			// A body the decoder refuses has no model to name, but the row is
			// still the evidence the call happened.
			name:          "a body the decoder refuses is recorded without a model",
			body:          `{"model":`,
			wantStatus:    http.StatusBadRequest,
			wantLogRows:   1,
			wantModel:     "",
			wantLogStatus: domain.RequestLogError,
		},
		{
			// The negative side: a served call's one row is a success row, so
			// an error row on the Logs screen means exactly a refused or
			// failed call. The model is not asserted here: a served row names
			// the upstream model the engine resolved, which is record()'s own
			// existing semantics.
			name:          "a served request leaves no refusal row",
			body:          chatBody("test/model"),
			wantStatus:    http.StatusOK,
			wantLogRows:   1,
			wantLogStatus: domain.RequestLogSuccess,
			wantUsageRows: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logs := &recordingLogs{}
			usage := &recordingUsage{}
			fixture := newChatHTTPFixtureWith(t, fixtureDeps{Logs: logs, Usage: usage})
			handler := NewChatHandler(fixture.chat)

			request := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(tc.body))
			request.Header.Set("Authorization", "Bearer "+fixture.key)
			recorder := httptest.NewRecorder()
			handler.Completions(recorder, request)

			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, tc.wantStatus, recorder.Body.String())
			}

			rows := logs.rows()
			if len(rows) != tc.wantLogRows {
				t.Fatalf("log rows = %d, want %d: %+v", len(rows), tc.wantLogRows, rows)
			}
			if rows[0].Status != tc.wantLogStatus {
				t.Errorf("log status = %q, want %q", rows[0].Status, tc.wantLogStatus)
			}
			if tc.wantModel != "" && rows[0].Model != tc.wantModel {
				t.Errorf("log model = %q, want %q", rows[0].Model, tc.wantModel)
			}
			if rows[0].GatewayKeyID == "" {
				t.Error("log row carries no gateway key id")
			}
			if rows[0].RequestBody != tc.body {
				t.Errorf("log request body = %q, want the bytes the client sent", rows[0].RequestBody)
			}
			if got := len(usage.rows()); got != tc.wantUsageRows {
				t.Errorf("usage rows = %d, want %d (a refusal reaches no attempt, register G17)", got, tc.wantUsageRows)
			}

			if tc.wantLogStatus != domain.RequestLogError {
				if rows[0].Error != "" {
					t.Errorf("served log row carries an error: %q", rows[0].Error)
				}
				return
			}
			// The row's error text must be the code the client was served, so
			// the Logs screen and the CLI cannot disagree about what happened.
			var served struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &served); err != nil {
				t.Fatalf("response body is not the error envelope: %s", recorder.Body.String())
			}
			if served.Error.Code == "" {
				t.Fatalf("response carries no code: %s", recorder.Body.String())
			}
			if rows[0].Error != served.Error.Code {
				t.Errorf("row error = %q, want the served code %q", rows[0].Error, served.Error.Code)
			}
		})
	}
}
