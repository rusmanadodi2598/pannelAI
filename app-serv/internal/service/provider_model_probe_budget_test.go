// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_model_probe_budget_test.go
// @for       The refusal and budget rules the single-model probe applies
//
//	(SPEC-API-001 §7.4, draft 017 §4.10).
//
// @uses      internal/dataplane, internal/domain, testing, time.
// @reason    These are the cases where a probe must NOT be reported as a model
//
//	answer: the deps are missing, the caller left, the budget ran out, or the
//	failure belongs to the gateway. They read differently from the per-model
//	table, so they are tested apart.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-27
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
)

// TestProviderModelTestService_AProbeThatRunsOutItsBudgetIsATimeout keeps a
// cancelled context from being reported as an internal failure: the operator
// asked how long the model takes, and the honest answer is "longer than allowed".
//
// The ceiling is shortened rather than the delay lengthened: at the real twenty
// seconds the assertion depended on a 20ms margin between two timers, which the
// race detector running this package beside nineteen others sometimes loses.
func TestProviderModelTestService_AProbeThatRunsOutItsBudgetIsATimeout(t *testing.T) {
	prober := newProbeStub()
	prober.delay = 2 * time.Second
	svc := newModelTestService(t, prober)
	svc.probeTimeout = 20 * time.Millisecond

	result, err := svc.TestModel(context.Background(), "openai", "gpt-4o")
	if err != nil {
		t.Fatalf("TestModel() error = %v, want a result", err)
	}
	if result.OK {
		t.Fatal("a probe that hit its budget must not report ok")
	}
	if result.ErrorCode != ProviderModelTimeoutCode {
		t.Fatalf("error_code = %q, want %q", result.ErrorCode, ProviderModelTimeoutCode)
	}
	if result.Error == "" {
		t.Fatal("a timed-out probe must name the budget it ran out")
	}
}

// TestProviderModelTestService_ACancelledCallerStopsTheProbe is AGENTS.md §1.6:
// a request the client abandons must not keep spending upstream budget.
func TestProviderModelTestService_ACancelledCallerStopsTheProbe(t *testing.T) {
	svc := newModelTestService(t, newProbeStub())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := svc.TestModel(ctx, "openai", "gpt-4o"); err == nil {
		t.Fatal("TestModel() on a cancelled context error = nil, want the cancellation")
	}
}

// TestProviderModelTestService_UnattributedProbeFailureStaysAnError distinguishes
// the two ways a probe can fail: the upstream said no (a row) and the service
// itself broke (an error the handler turns into a 5xx).
func TestProviderModelTestService_UnattributedProbeFailureStaysAnError(t *testing.T) {
	prober := newProbeStub()
	prober.fail = map[string]error{"openai/gpt-4o": errors.New("the engine vanished")}
	svc := newModelTestService(t, prober)

	result, err := svc.TestModel(context.Background(), "openai", "gpt-4o")
	if err != nil {
		t.Fatalf("TestModel() error = %v, want a row", err)
	}
	if result.OK || result.ErrorCode != dataplane.CodeInternal {
		t.Fatalf("row = %+v, want a failed probe carrying the internal code", result)
	}
}

// TestProviderModelTestService_ResultShape is the wire contract the panel reads:
// a healthy row carries no error fields, so the table cannot render a stale
// reason beside a pass.
func TestProviderModelTestService_ResultShape(t *testing.T) {
	svc := newModelTestService(t, newProbeStub())
	result, err := svc.TestModel(context.Background(), "openai", "gpt-4o")
	if err != nil {
		t.Fatalf("TestModel() error = %v", err)
	}
	if result.Error != "" || result.ErrorCode != "" || result.Status != 0 {
		t.Fatalf("healthy row = %+v, want no error fields", result)
	}
	if result.ModelID == "" || result.Name == "" {
		t.Fatalf("row = %+v, want the model named", result)
	}
}
