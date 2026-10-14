// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_budget_test.go
// @for       Tests for the failover budget: the attempt count that stops a walk, the clock that stops a leg, and the code each answers with.
// @uses      context, testing, time, internal/domain.
// @reason    The chain a combo can walk is a product, and every step of it carried its own per-call deadline with nothing above it, so a caller that gave up an hour ago could still be served upstream. Two bounds cut that, and which one fired decides whether the client is told the provider was unavailable or the walk timed out: a distinction a retry policy acts on differently.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-14
package dataplane

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestRequestBudget_SpendCountsEveryCredential(t *testing.T) {
	cases := []struct {
		name     string
		attempts int
		wantOK   []bool
	}{
		{name: "two attempts allow two picks", attempts: 2, wantOK: []bool{true, true, false}},
		{name: "one attempt allows exactly one", attempts: 1, wantOK: []bool{true, false}},
		{name: "a spent budget never allows another", attempts: 0, wantOK: []bool{false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			budget := &RequestBudget{deadline: now.Add(time.Minute), attempts: tc.attempts}
			for i, want := range tc.wantOK {
				if got := budget.spend(); got != want {
					t.Fatalf("spend() call %d = %v, want %v", i+1, got, want)
				}
			}
		})
	}
}

// TestRequestBudget_DeadlineIsExclusive pins that the instant the budget was opened
// is still inside it, and the exact expiry moment is outside: a walk stopped one
// second early is a request refused for no reason.
func TestRequestBudget_DeadlineIsExclusive(t *testing.T) {
	budget := &RequestBudget{deadline: now.Add(time.Minute), attempts: 8}
	if budget.expired(now) {
		t.Fatal("expired() = true at the opening instant")
	}
	if budget.expired(now.Add(time.Minute - time.Millisecond)) {
		t.Fatal("expired() = true one millisecond inside the budget")
	}
	if !budget.expired(now.Add(time.Minute)) {
		t.Fatal("expired() = false at the deadline, want the window closed")
	}
}

// TestRequestBudget_ErrorNamesWhatTheWalkLearned pins the code split: a walk that
// reached an upstream is a timeout, one that never left the selector is an
// unavailable provider.
func TestRequestBudget_ErrorNamesWhatTheWalkLearned(t *testing.T) {
	cases := []struct {
		name     string
		called   bool
		wantCode string
		wantWait int
	}{
		{name: "a walk that called upstream times out", called: true,
			wantCode: CodeUpstreamTimeout, wantWait: domain.DefaultFailoverTotalMS / 1000},
		{name: "a walk that never called reports no provider", called: false,
			wantCode: CodeNoProvider, wantWait: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			budget := &RequestBudget{
				deadline: now.Add(time.Minute), total: time.Duration(domain.DefaultFailoverTotalMS) * time.Millisecond,
				attempts: 1, called: tc.called,
			}
			failure := AsError(budget.error("alpha", errors.New("the upstream refused")))
			if failure.Code != tc.wantCode {
				t.Fatalf("code = %s, want %s", failure.Code, tc.wantCode)
			}
			if failure.RetryAfter != tc.wantWait {
				t.Fatalf("Retry-After = %d, want %d", failure.RetryAfter, tc.wantWait)
			}
		})
	}
}

// stubBudgets answers one configured budget, so the walk's bound is a fact a test
// sets rather than a default it inherits by accident.
type stubBudgets struct {
	budget domain.FailoverBudget
	err    error
}

func (s stubBudgets) FailoverBudget(context.Context) (domain.FailoverBudget, error) {
	if s.err != nil {
		return domain.FailoverBudget{}, s.err
	}
	return s.budget, nil
}

func TestEngine_newRequestBudget(t *testing.T) {
	fixed := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name         string
		reader       FailoverBudgetReader
		wantTotal    time.Duration
		wantAttempts int
		wantDegraded bool
	}{
		{name: "an unwired reader answers the documented budget", reader: nil,
			wantTotal:    time.Duration(domain.DefaultFailoverTotalMS) * time.Millisecond,
			wantAttempts: domain.DefaultFailoverAttempts},
		{name: "a configured budget is applied",
			reader:    stubBudgets{budget: domain.FailoverBudget{TotalMS: 45_000, MaxAttempts: 3}},
			wantTotal: 45 * time.Second, wantAttempts: 3},
		{name: "an out-of-range stored value normalizes instead of walking unbounded",
			reader:       stubBudgets{budget: domain.FailoverBudget{TotalMS: 999_999, MaxAttempts: 0}},
			wantTotal:    time.Duration(domain.DefaultFailoverTotalMS) * time.Millisecond,
			wantAttempts: domain.DefaultFailoverAttempts, wantDegraded: true},
		{name: "a failed read answers the documented budget",
			reader:       stubBudgets{err: errors.New("settings row unreadable")},
			wantTotal:    time.Duration(domain.DefaultFailoverTotalMS) * time.Millisecond,
			wantAttempts: domain.DefaultFailoverAttempts},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine := &Engine{budgets: tc.reader, clock: func() time.Time { return fixed }}
			budget := engine.newRequestBudget(context.Background())
			if budget.total != tc.wantTotal {
				t.Fatalf("total = %v, want %v", budget.total, tc.wantTotal)
			}
			if budget.attempts != tc.wantAttempts {
				t.Fatalf("attempts = %d, want %d", budget.attempts, tc.wantAttempts)
			}
			if !budget.deadline.Equal(fixed.Add(tc.wantTotal)) {
				t.Fatalf("deadline = %v, want %v from the walk's own opening", budget.deadline, fixed.Add(tc.wantTotal))
			}
		})
	}
}
