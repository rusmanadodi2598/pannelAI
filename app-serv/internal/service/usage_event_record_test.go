// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/usage_event_record_test.go
// @for       Table-driven tests for the recorder's event emission: that the
//
//	choke point emits exactly one event per stored row, and emits
//	nothing for a row it refused.
//
// @uses      internal/domain, internal/repository, context, errors, sync,
//
//	testing, time.
//
// @reason    F4 of draft 010 is closed by "the event has a real publisher and a
//
//	real subscriber". The subscriber's own tests prove it consumes; this
//	file proves the publisher is actually reached from the production
//	path rather than only from a test that calls it directly. It is the
//	seam test for the whole finding: if Record stops emitting, the
//	channel goes quiet and nothing else fails.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-22
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestUsageService_RecordEmitsOneEventPerStoredRow is the F4 seam test: the
// production path stores a row and emits its event, and the event that reaches
// the channel describes the row that was stored.
func TestUsageService_RecordEmitsOneEventPerStoredRow(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(*domain.UsageRecordInput)
		wantLine []string
	}{
		{
			name:     "a successful call emits its identity and counters",
			wantLine: []string{"req_1", "openai", "ep_1", "gpt-4o", "tokens=7", "status=success"},
		},
		{
			name: "a failed call emits the error status",
			mutate: func(in *domain.UsageRecordInput) {
				in.Status = domain.UsageStatusError
				in.ErrorCode = "MODEL_NOT_FOUND"
			},
			wantLine: []string{"req_1", "status=error"},
		},
		{
			name: "a call with no endpoint still emits",
			mutate: func(in *domain.UsageRecordInput) {
				in.EndpointID = ""
			},
			wantLine: []string{"req_1", "endpoint=", "status=success"},
		},
		{
			name: "a call that reported no usage still emits",
			mutate: func(in *domain.UsageRecordInput) {
				in.TokensIn, in.TokensOut, in.CostUSD = 0, 0, "0"
			},
			wantLine: []string{"req_1", "tokens=0", "cost=0.00000000"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bus := &usageEventBusDouble{}
			svc, repo := newRecordingUsageService(t, bus)
			input := usageRecordInputFixture()
			if tc.mutate != nil {
				tc.mutate(&input)
			}

			record, err := svc.Record(context.Background(), input)
			if err != nil {
				t.Fatalf("Record() = %v, want nil", err)
			}
			if repo.count() != 1 {
				t.Fatalf("stored rows = %d, want 1", repo.count())
			}
			if record.RequestID() != input.RequestID {
				t.Fatalf("stored request id = %q, want %q", record.RequestID(), input.RequestID)
			}

			// The publisher queues; the drain is what puts it on the broker.
			payloads := drainEvents(t, svc, bus)
			if len(payloads) != 1 {
				t.Fatalf("published events = %d, want 1", len(payloads))
			}
			event, err := domain.DecodeUsageEvent(payloads[0])
			if err != nil {
				t.Fatalf("the emitted event does not decode: %v", err)
			}
			if event.RequestID != input.RequestID {
				t.Fatalf("event request id = %q, want %q", event.RequestID, input.RequestID)
			}
			if event.TotalTokens != input.TokensIn+input.TokensOut {
				t.Fatalf("event tokens = %d, want %d", event.TotalTokens, input.TokensIn+input.TokensOut)
			}
			line := event.SummaryLine()
			for _, want := range tc.wantLine {
				if !contains(line, want) {
					t.Fatalf("event line %q does not contain %q", line, want)
				}
			}
		})
	}
}

// TestUsageService_RecordEmitsNothingForARowItRefused asserts the event means
// "this request is recorded": an input the aggregate rejects stores nothing and
// emits nothing, so a consumer cannot observe a request that was never written.
func TestUsageService_RecordEmitsNothingForARowItRefused(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*domain.UsageRecordInput)
	}{
		{name: "no request id", mutate: func(in *domain.UsageRecordInput) { in.RequestID = "" }},
		{name: "no provider", mutate: func(in *domain.UsageRecordInput) { in.ProviderID = "" }},
		{name: "no model", mutate: func(in *domain.UsageRecordInput) { in.Model = "" }},
		{name: "an unknown status", mutate: func(in *domain.UsageRecordInput) { in.Status = "banana" }},
		{name: "a negative token count", mutate: func(in *domain.UsageRecordInput) { in.TokensIn = -1 }},
		{name: "an unparseable cost", mutate: func(in *domain.UsageRecordInput) { in.CostUSD = "abc" }},
		{name: "a negative cost", mutate: func(in *domain.UsageRecordInput) { in.CostUSD = "-0.01" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bus := &usageEventBusDouble{}
			svc, repo := newRecordingUsageService(t, bus)
			input := usageRecordInputFixture()
			tc.mutate(&input)

			if _, err := svc.Record(context.Background(), input); err == nil {
				t.Fatal("Record() = nil, want a refusal")
			}
			if repo.count() != 0 {
				t.Fatalf("stored rows = %d, want 0", repo.count())
			}
			if payloads := drainEvents(t, svc, bus); len(payloads) != 0 {
				t.Fatalf("published events = %d, want 0", len(payloads))
			}
		})
	}
}

// TestUsageService_RecordEmitsNothingWhenTheStoreFailed asserts a failed write
// emits no event: a consumer must never be told about a row that is not there.
func TestUsageService_RecordEmitsNothingWhenTheStoreFailed(t *testing.T) {
	bus := &usageEventBusDouble{}
	svc, repo := newRecordingUsageService(t, bus)
	repo.err = errors.New("postgres is down")

	if _, err := svc.Record(context.Background(), usageRecordInputFixture()); err == nil {
		t.Fatal("Record() = nil, want the storage failure")
	}
	if payloads := drainEvents(t, svc, bus); len(payloads) != 0 {
		t.Fatalf("published events = %d, want 0 after a failed write", len(payloads))
	}
}
