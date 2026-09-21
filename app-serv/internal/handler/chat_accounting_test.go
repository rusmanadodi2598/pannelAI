// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/chat_accounting_test.go
// @for       The accounting the chat route writes for one served request.
// @uses      internal/dataplane, internal/domain, internal/service, net/http,
//
//	net/http/httptest, strings, sync, testing.
//
// @reason    F4 of docs/DRAFT/009-PLAYGROUND-CHAT-ENDPOINT-READINESS.md requires
// the accounting seam to be exercised through the HTTP boundary, not only by
// calling the recorder. SPEC-API-001 §7.12 makes one usage row per call that
// reached an upstream part of the pipeline, and a refusal before any attempt
// deliberately writes none, so both halves are pinned here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-21
package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// recordingUsage captures the usage rows a served request writes.
type recordingUsage struct {
	mu     sync.Mutex
	inputs []domain.UsageRecordInput
}

func (r *recordingUsage) Record(_ context.Context, in domain.UsageRecordInput) (domain.UsageRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inputs = append(r.inputs, in)
	return domain.NewUsageRecord(in, "usr_test", testInstant())
}

func (r *recordingUsage) rows() []domain.UsageRecordInput {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.UsageRecordInput, len(r.inputs))
	copy(out, r.inputs)
	return out
}

// TestChatCompletionsHTTP_AccountingTable pins which requests leave a usage row
// and what that row says, for a success, an upstream failure, and a refusal
// that never reached an attempt.
func TestChatCompletionsHTTP_AccountingTable(t *testing.T) {
	cases := []struct {
		name        string
		model       string
		key         string
		body        string
		wantStatus  int
		wantRows    int
		wantStatusV domain.UsageStatus
		wantCode    string
	}{
		{
			name: "a served request writes one success row", model: "test/model",
			wantStatus: http.StatusOK, wantRows: 1, wantStatusV: domain.UsageStatusSuccess,
		},
		{
			name: "an upstream failure writes one error row", model: "test/" + upstreamPreFrame,
			wantStatus: http.StatusBadGateway, wantRows: 1, wantStatusV: domain.UsageStatusError,
			wantCode: dataplane.CodeUpstreamError,
		},
		{
			name: "a refusal before any attempt writes no row", model: "",
			body: `{"model":"","messages":[{"role":"user","content":"hi"}]}`,
			// An empty model is refused by the schema, which runs before any
			// upstream attempt, so there is no provider or model to name.
			wantStatus: http.StatusBadRequest, wantRows: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			usage := &recordingUsage{}
			fixture := newChatHTTPFixtureWith(t, fixtureDeps{Usage: usage})
			handler := NewChatHandler(fixture.chat)
			body := tc.body
			if body == "" {
				body = chatBody(tc.model)
			}
			request := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(body))
			request.Header.Set("Authorization", "Bearer "+fixture.key)
			recorder := httptest.NewRecorder()
			handler.Completions(recorder, request)
			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, tc.wantStatus, recorder.Body.String())
			}
			rows := usage.rows()
			if len(rows) != tc.wantRows {
				t.Fatalf("usage rows = %d, want %d", len(rows), tc.wantRows)
			}
			if tc.wantRows == 0 {
				return
			}
			if rows[0].Status != tc.wantStatusV {
				t.Errorf("usage status = %q, want %q", rows[0].Status, tc.wantStatusV)
			}
			if tc.wantCode != "" && rows[0].ErrorCode != tc.wantCode {
				t.Errorf("usage error code = %q, want %q", rows[0].ErrorCode, tc.wantCode)
			}
			if rows[0].ProviderID == "" || rows[0].Model == "" {
				t.Errorf("usage row carries no identity: %+v", rows[0])
			}
			if rows[0].GatewayKeyID == "" {
				t.Error("usage row carries no gateway key id")
			}
		})
	}
}

// TestChatCompletionsHTTP_UsageCountsTheUpstreamsNumbers pins that the row
// carries what the provider billed rather than a recomputed estimate, which is
// the property an operator reads as spend.
func TestChatCompletionsHTTP_UsageCountsTheUpstreamsNumbers(t *testing.T) {
	usage := &recordingUsage{}
	fixture := newChatHTTPFixtureWith(t, fixtureDeps{Usage: usage})
	handler := NewChatHandler(fixture.chat)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(chatBody("test/model")))
	request.Header.Set("Authorization", "Bearer "+fixture.key)
	handler.Completions(httptest.NewRecorder(), request)
	rows := usage.rows()
	if len(rows) != 1 {
		t.Fatalf("usage rows = %d, want 1", len(rows))
	}
	if rows[0].TokensIn != 1 || rows[0].TokensOut != 1 {
		t.Fatalf("tokens = (%d, %d), want (1, 1) from the upstream usage block", rows[0].TokensIn, rows[0].TokensOut)
	}
}

var _ service.UsageRecorder = (*recordingUsage)(nil)
