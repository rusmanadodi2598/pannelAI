// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/dataplane_record_context_test.go
// @for       The data-plane recorder's writes outlive the client that asked
//
//	for the call, both the served pair and a refusal's single log row.
//
// @uses      context, testing, internal/dataplane, internal/domain.
// @reason    F6 was fixed for chat at its call site; the media, embeddings,
//
//	and systemone services share this recorder, so the rule lives in
//	the recorder and these two tests cover every call site. The
//	doubles refuse a write under a cancelled context the way a database
//	driver does, and each test's negative control proves they
//	discriminate before the positive assertion.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-25
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// ctxAwareLogRecorder is the log half of the ctx-aware pair: it refuses a write
// whose context is already cancelled and collects the rows that got through.
type ctxAwareLogRecorder struct {
	rows []domain.RequestLogInput
}

func (r *ctxAwareLogRecorder) Record(ctx context.Context, in domain.RequestLogInput) (domain.RequestLog, error) {
	if err := ctx.Err(); err != nil {
		return domain.RequestLog{}, err
	}
	r.rows = append(r.rows, in)
	return domain.RequestLog{}, nil
}

// requestIDFromValue answers the request id the test rode in the context, the
// way the router's own reader answers one in production.
func requestIDFromValue(ctx context.Context) string {
	if id, ok := ctx.Value(accountingKey("request")).(string); ok {
		return id
	}
	return ""
}

// cancelledClientContext is the state F6 measured: the client is gone before
// the accounting write runs.
func cancelledClientContext(requestID string) context.Context {
	ctx, cancel := context.WithCancel(context.WithValue(
		context.Background(), accountingKey("request"), requestID))
	cancel()
	return ctx
}

// TestDataPlaneRecorderRecordOutlivesTheClient pins that the pair a served
// media-family call writes lands even though the client is gone, under the
// request's own id.
func TestDataPlaneRecorderRecordOutlivesTheClient(t *testing.T) {
	clientCtx := cancelledClientContext("req_media")
	usage, logs := &ctxAwareUsageRecorder{}, &ctxAwareLogRecorder{}
	recorder := newDataPlaneRecorder(usage, logs, nil, requestIDFromValue)

	// The negative control: under the client's own context both writes are
	// refused, which is the loss F6 measured. Without it the assertions below
	// would pass for doubles that never refuse anything.
	if _, err := usage.Record(clientCtx, domain.UsageRecordInput{}); err == nil {
		t.Fatal("the usage double must refuse a write under the client's cancelled context")
	}
	if _, err := logs.Record(clientCtx, domain.RequestLogInput{}); err == nil {
		t.Fatal("the log double must refuse a write under the client's cancelled context")
	}

	recorder.record(clientCtx, dataplane.Outcome{ProviderID: "openai", Model: "whisper-1"}, "gky_1", "0", 12, nil)

	if len(usage.rows) != 1 || len(logs.rows) != 1 {
		t.Fatalf("recorded %d usage / %d log rows, want the pair under the client's cancelled context",
			len(usage.rows), len(logs.rows))
	}
	if usage.rows[0].RequestID != "req_media" || logs.rows[0].RequestID != "req_media" {
		t.Fatalf("request ids = %q/%q, want the request's own value kept by the detached context",
			usage.rows[0].RequestID, logs.rows[0].RequestID)
	}
}

// TestDataPlaneRecorderRefuseOutlivesTheClient covers the recorder's other
// write: the one log row a call refused before any dial leaves.
func TestDataPlaneRecorderRefuseOutlivesTheClient(t *testing.T) {
	clientCtx := cancelledClientContext("req_refused")
	logs := &ctxAwareLogRecorder{}
	recorder := newDataPlaneRecorder(nil, logs, nil, requestIDFromValue)

	if _, err := logs.Record(clientCtx, domain.RequestLogInput{}); err == nil {
		t.Fatal("the log double must refuse a write under the client's cancelled context")
	}

	recorder.refuse(clientCtx, dataplane.Outcome{ProviderID: "nope", Model: "voice"}, "gky_1",
		dataplane.ValidationError("no provider named"))

	if len(logs.rows) != 1 {
		t.Fatalf("recorded %d log rows, want the refusal logged under the client's cancelled context", len(logs.rows))
	}
	row := logs.rows[0]
	if row.RequestID != "req_refused" || row.Error != dataplane.CodeValidation {
		t.Fatalf("row = %q/%q, want the request's own id and the refusal's code", row.RequestID, row.Error)
	}
}
