// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/accounting_context_test.go
// @for       The accounting write's context: it survives the client and keeps its
//
//	own deadline (draft 021 F6).
//
// @uses      context, testing, time, internal/domain.
// @reason    A call that died mid-flight left no row while its key counter still
//
//	counted it. The write ran under the client's context, so the test
//	carries a recorder that fails exactly the way a database driver does
//	on a cancelled context, and the negative control proves the recorder
//	would have refused the write the client's own context asked for.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-25
package service

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// ctxAwareUsageRecorder fails a write whose context is already cancelled, which
// is what a database driver does, and collects the rows that got through.
type ctxAwareUsageRecorder struct {
	rows []domain.UsageRecordInput
}

func (r *ctxAwareUsageRecorder) Record(ctx context.Context, in domain.UsageRecordInput) (domain.UsageRecord, error) {
	if err := ctx.Err(); err != nil {
		return domain.UsageRecord{}, err
	}
	r.rows = append(r.rows, in)
	return domain.UsageRecord{}, nil
}

// accountingKey is the context value the request id rides in.
type accountingKey string

// TestAccountingContextOutlivesTheClient pins F6's rule: a write under the
// accounting context lands even though the client is gone, and it keeps the
// request's values so the rows stay reachable from the request id.
func TestAccountingContextOutlivesTheClient(t *testing.T) {
	clientCtx, cancel := context.WithCancel(context.WithValue(
		context.Background(), accountingKey("request"), "req_chat"))
	cancel()

	writeCtx, done := accountingContext(clientCtx)
	defer done()

	if err := writeCtx.Err(); err != nil {
		t.Fatalf("the accounting context must not be cancelled with the client: %v", err)
	}
	if got := writeCtx.Value(accountingKey("request")); got != "req_chat" {
		t.Fatalf("the accounting context must keep the request's values, got %v", got)
	}
	deadline, ok := writeCtx.Deadline()
	if !ok {
		t.Fatal("the accounting context must carry its own deadline (AGENTS.md §1.6)")
	}
	if remaining := time.Until(deadline); remaining <= 0 || remaining > accountingTimeout {
		t.Fatalf("deadline in %v, want a positive bound within %v", remaining, accountingTimeout)
	}
}

// TestAccountingContextCarriesAWriteTheClientCouldNot is the behavioural half:
// the same recorder that refuses a write under the cancelled client context
// accepts one under the accounting context, so the row a mid-flight death used
// to lose is written.
func TestAccountingContextCarriesAWriteTheClientCouldNot(t *testing.T) {
	clientCtx, cancel := context.WithCancel(context.Background())
	cancel()
	recorder := &ctxAwareUsageRecorder{}

	// The negative control: under the client's own context the write is lost,
	// which is the state F6 measured. Without this the assertion below would
	// pass for a recorder that never refuses anything.
	if _, err := recorder.Record(clientCtx, domain.UsageRecordInput{RequestID: "req_chat"}); err == nil {
		t.Fatal("the recorder must refuse a write under the client's cancelled context")
	}
	if len(recorder.rows) != 0 {
		t.Fatalf("the cancelled client's write landed %d rows, want none", len(recorder.rows))
	}

	writeCtx, done := accountingContext(clientCtx)
	defer done()
	if _, err := recorder.Record(writeCtx, domain.UsageRecordInput{RequestID: "req_chat"}); err != nil {
		t.Fatalf("the accounting context must carry the write: %v", err)
	}
	if len(recorder.rows) != 1 || recorder.rows[0].RequestID != "req_chat" {
		t.Fatalf("rows = %+v, want the one write under the request id", recorder.rows)
	}
}
