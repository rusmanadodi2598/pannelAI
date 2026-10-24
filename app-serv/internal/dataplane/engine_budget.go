// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_budget.go
// @for       One request's failover budget: how long the walk may run and how many credentials it may spend before it reports what it already learned.
// @uses      internal/domain, internal/logx, context, time.
// @reason    The chain a combo can walk is a product, not a sum, and every step of it carried its own per-call deadline with nothing above it: 64 members, 100 accounts each, 100 keys each, two transport attempts per call, and a client that gave up an hour ago still being served upstream. Two bounds cut that, and both answer with the failure the walk already has rather than inventing a third one.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-14
package dataplane

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/logx"
)

// FailoverBudgetReader answers the walk budget one request gets. Optional on the
// engine: an unwired or failing read answers the documented budget, the same rule
// the rotation policy follows, because a settings outage must not hand a caller an
// unbounded walk and must not refuse a request it could serve.
type FailoverBudgetReader interface {
	FailoverBudget(ctx context.Context) (domain.FailoverBudget, error)
}

// RequestBudget is one client request's share of the failover walk.
//
// It bounds the pre-commit walk only. Once a frame has reached the client the stream
// is committed and the idle guard owns the bound, because a budget that fires there
// would stop paying for an answer the caller is already reading.
type RequestBudget struct {
	deadline time.Time
	// total is the configured clock, kept so the Retry-After a cut-short walk sends
	// names the wait this request was actually given rather than a default.
	total time.Duration
	// attempts is what remains. It counts credentials spent, so a leg that fails
	// over twenty times burns twenty and a leg that answers on the first spends one.
	attempts int
	// called records that at least one upstream call was actually made, which
	// decides whether the client sees a timeout or an unavailable provider.
	called bool
}

// newRequestBudget opens the budget for one request. The clock starts here, at the
// top of the pipeline, so resolver time counts against the walk that follows it.
func (e *Engine) newRequestBudget(ctx context.Context) *RequestBudget {
	budget := domain.FailoverBudget{}.Normalized()
	if e.budgets != nil {
		read, err := e.budgets.FailoverBudget(ctx)
		if err != nil {
			logx.Degraded(e.logger, "failover budget unreadable, the documented budget applies", err)
		} else {
			budget = read.Normalized()
		}
	}
	total := budget.Total()
	return &RequestBudget{
		deadline: e.clock().Add(total),
		total:    total,
		attempts: budget.Attempts(),
	}
}

// expired reports whether the walk has used its clock.
func (b *RequestBudget) expired(now time.Time) bool {
	return !now.Before(b.deadline)
}

// spend takes one credential attempt and answers false once the attempts are gone:
// another account would buy upstream time for an answer the caller stopped waiting for.
func (b *RequestBudget) spend() bool {
	if b.attempts <= 0 {
		return false
	}
	b.attempts--
	return true
}

// markCalled records that an upstream was reached, which is what tells a walk that
// ran out of accounts from one that ran out of time.
func (b *RequestBudget) markCalled() { b.called = true }

// pause pushes the walk's clock out by d: time spent waiting on a local
// resource (the per-session queue a throttled upstream needs) is not upstream
// time, so it must not be charged to the walk that bounds the upstream calls.
// The clock is the walk's own; the request's wall clock still includes the
// wait, which is the caller's price for being serialized rather than refused.
func (b *RequestBudget) pause(d time.Duration) {
	if d > 0 {
		b.deadline = b.deadline.Add(d)
	}
}

// error reports the cut short by what ran out: a walk that had already called upstream
// is a timeout, one that never reached an account is unavailability. Both are codes the
// client already knows, and Retry-After carries the budget's own length.
func (b *RequestBudget) error(providerID string, last error) error {
	if b.called {
		failure := dataPlaneError(CodeUpstreamTimeout,
			"the failover walk for provider "+providerID+" stopped at its own time or attempt budget")
		failure.RetryAfter = int(b.total.Seconds())
		if last != nil {
			failure.Wrapped = last
		}
		return failure
	}
	return domain.NewNoProviderAvailableError("the failover walk for provider " + providerID +
		" stopped at its own attempt budget before any upstream was called")
}
