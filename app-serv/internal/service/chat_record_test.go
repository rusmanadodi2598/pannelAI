// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/chat_record_test.go
// @for       The §7.12/§7.13 accounting pair one chat call writes, on the
//
//	served path and on the failed one.
//
// @uses      internal/dataplane, internal/domain, internal/schema, context,
//
//	testing, time.
//
// @reason    Register G17 and G18 (2026-09-19) are both about what a chat call
//
//	leaves behind: the failed call wrote no usage row because the engine
//	handed back a zero outcome, and no call wrote a request log at all.
//	These tests pin the pair's shape — one row of each, one shared
//	request id, identity and code on the failed one, bodies handed over
//	untouched — so a later change to the pipeline cannot quietly drop
//	either half again.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// chatRecordFixture builds a ChatService over the collecting doubles with a
// fixed request id, so one call's rows can be read field by field.
func chatRecordFixture() (*ChatService, *stubUsageRecorder, *stubLogRecorder) {
	usage, logs := &stubUsageRecorder{}, &stubLogRecorder{}
	return &ChatService{
		usage:     usage,
		logs:      logs,
		requestID: func(context.Context) string { return "req_chat" },
		clock:     time.Now,
	}, usage, logs
}

// chatRecordRequest is a client request as the handler hands it to the service:
// the decoded shape plus the raw body the client sent.
func chatRecordRequest() dataplane.Request {
	return dataplane.Request{
		Route:        dataplane.RouteChatCompletions,
		ClientFormat: schema.FormatOpenAI,
		Model:        "alpha/broken",
		Chat:         &schema.ChatRequest{Model: "alpha/broken"},
		Raw:          []byte(`{"model":"alpha/broken","messages":[{"role":"user","content":"ping"}]}`),
	}
}

// TestChatService_RecordWritesTheAccountingPairOnFailure pins G17 and G18
// together: a call that failed at the upstream still leaves one usage row
// (`status: error`, the code, the attempted identity) and one request log, both
// under the same request id — and the stored error text is the code alone,
// because a chat upstream's message can quote the credential back.
func TestChatService_RecordWritesTheAccountingPairOnFailure(t *testing.T) {
	svc, usage, logs := chatRecordFixture()
	request := chatRecordRequest()
	outcome := dataplane.Outcome{
		Format:     schema.FormatOpenAI,
		ProviderID: "alpha",
		EndpointID: "ep-alpha",
		Model:      "broken",
		Combo:      "daily",
		LatencyMS:  42,
	}

	svc.record(context.Background(), request, outcome, "key_1", dataplane.CodeUpstreamError)

	if len(usage.rows) != 1 || len(logs.rows) != 1 {
		t.Fatalf("recorded %d usage / %d log rows, want one of each", len(usage.rows), len(logs.rows))
	}
	row, entry := usage.rows[0], logs.rows[0]

	if row.RequestID != "req_chat" || entry.RequestID != "req_chat" {
		t.Fatalf("request ids = %q / %q, want both req_chat", row.RequestID, entry.RequestID)
	}
	if row.Status != domain.UsageStatusError || row.ErrorCode != dataplane.CodeUpstreamError {
		t.Fatalf("usage status/code = %s/%s, want error/%s", row.Status, row.ErrorCode, dataplane.CodeUpstreamError)
	}
	if row.ProviderID != "alpha" || row.EndpointID != "ep-alpha" || row.Model != "broken" || row.Combo != "daily" {
		t.Fatalf("usage identity = %s/%s/%s/%s, want alpha/ep-alpha/broken/daily",
			row.ProviderID, row.EndpointID, row.Model, row.Combo)
	}
	if row.GatewayKeyID != "key_1" || entry.GatewayKeyID != "key_1" {
		t.Fatalf("gateway key = %q / %q, want both key_1", row.GatewayKeyID, entry.GatewayKeyID)
	}
	if row.LatencyMS != 42 || entry.LatencyMS != 42 {
		t.Fatalf("latency = %d / %d, want both 42", row.LatencyMS, entry.LatencyMS)
	}
	if entry.Status != domain.RequestLogError {
		t.Fatalf("log status = %s, want error", entry.Status)
	}
	if entry.Error != dataplane.CodeUpstreamError {
		t.Fatalf("log error = %q, want the code alone (%s)", entry.Error, dataplane.CodeUpstreamError)
	}
	if entry.RequestBody != string(request.Raw) {
		t.Fatalf("log request body = %q, want the request's raw body", entry.RequestBody)
	}
	if entry.ResponseBody != "" {
		t.Fatalf("log response body = %q, want empty for a call that failed", entry.ResponseBody)
	}
}

// TestChatService_RecordRefusedBeforeAnAttemptLogsWithoutUsage pins the boundary:
// a request refused before the pipeline ran (an unknown model, an invalid body)
// has no identity, so it leaves no usage row — the aggregate would reject one —
// but its failure still appears in the logs, which is where §7.13 shows it.
func TestChatService_RecordRefusedBeforeAnAttemptLogsWithoutUsage(t *testing.T) {
	svc, usage, logs := chatRecordFixture()

	svc.record(context.Background(), chatRecordRequest(), dataplane.Outcome{}, "", dataplane.CodeModelNotFound)

	if len(usage.rows) != 0 {
		t.Fatalf("recorded %d usage rows, want none for a request with no attempted provider", len(usage.rows))
	}
	if len(logs.rows) != 1 {
		t.Fatalf("recorded %d log rows, want the refused call logged", len(logs.rows))
	}
	entry := logs.rows[0]
	if entry.Status != domain.RequestLogError || entry.Error != dataplane.CodeModelNotFound {
		t.Fatalf("log status/error = %s/%q, want error/%s", entry.Status, entry.Error, dataplane.CodeModelNotFound)
	}
}

// TestChatService_RecordWritesTheServedAnswer pins the other half: a served call
// leaves the same pair with `success`, the upstream's token accounting on the
// usage row, and the translated answer handed to the log for the capture rule to
// decide about.
func TestChatService_RecordWritesTheServedAnswer(t *testing.T) {
	svc, usage, logs := chatRecordFixture()
	request := chatRecordRequest()
	outcome := dataplane.Outcome{
		Format:     schema.FormatOpenAI,
		ProviderID: "alpha",
		EndpointID: "ep-alpha",
		Model:      "broken",
		Body:       []byte(`{"choices":[{"message":{"content":"pong"}}]}`),
		Usage: &schema.Usage{
			PromptTokens: 5, CompletionTokens: 2,
			PromptTokensDetails: &schema.PromptTokensDetails{CachedTokens: 3, CacheCreationTokens: 1},
		},
		LatencyMS: 7,
	}

	svc.record(context.Background(), request, outcome, "key_1", "")

	if len(usage.rows) != 1 || len(logs.rows) != 1 {
		t.Fatalf("recorded %d usage / %d log rows, want one of each", len(usage.rows), len(logs.rows))
	}
	row, entry := usage.rows[0], logs.rows[0]

	if row.Status != domain.UsageStatusSuccess || row.ErrorCode != "" {
		t.Fatalf("usage status/code = %s/%q, want success and no code", row.Status, row.ErrorCode)
	}
	if row.TokensIn != 5 || row.TokensOut != 2 || row.TokensCacheRead != 3 || row.TokensCacheWrite != 1 {
		t.Fatalf("usage tokens = %d/%d cache %d/%d, want 5/2 cache 3/1",
			row.TokensIn, row.TokensOut, row.TokensCacheRead, row.TokensCacheWrite)
	}
	if entry.Status != domain.RequestLogSuccess || entry.Error != "" {
		t.Fatalf("log status/error = %s/%q, want success and no error", entry.Status, entry.Error)
	}
	if entry.ResponseBody != string(outcome.Body) {
		t.Fatalf("log response body = %q, want the served answer handed to the capture rule", entry.ResponseBody)
	}
}
