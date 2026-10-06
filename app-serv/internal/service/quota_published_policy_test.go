// The two sweep limits an operator can move from configuration.
//
// @file      internal/service/quota_published_policy_test.go
// @for       Tests for PublishedPollPolicy.WithSweepLimits and the boot guard around it.
// @uses      testing, time.
// @reason    The budget and the concurrency cap are the only provider-traffic knobs a deployment can turn without a code change, so the merge rule (a value set wins, a value not set keeps the default) and the constructor's refusal to run on a zero are both worth pinning: the alternative is a sweep that silently asks nothing, or asks everything at once.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package service

import (
	"testing"
	"time"
)

func TestPublishedPollPolicyWithSweepLimits(t *testing.T) {
	base := DefaultPublishedPollPolicy()

	cases := []struct {
		name          string
		budget        int
		concurrency   int
		wantBudget    int
		wantConcurren int
	}{
		{name: "both set", budget: 12, concurrency: 2, wantBudget: 12, wantConcurren: 2},
		{name: "unset keeps the default", budget: 0, concurrency: 0, wantBudget: 40, wantConcurren: 4},
		{name: "negative keeps the default", budget: -5, concurrency: -1, wantBudget: 40, wantConcurren: 4},
		{name: "only the budget moves", budget: 7, concurrency: 0, wantBudget: 7, wantConcurren: 4},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := base.WithSweepLimits(testCase.budget, testCase.concurrency)
			if got.Budget != testCase.wantBudget || got.Concurrency != testCase.wantConcurren {
				t.Fatalf("limits = %d/%d, want %d/%d",
					got.Budget, got.Concurrency, testCase.wantBudget, testCase.wantConcurren)
			}
			// The timeouts and the seed page are not operator knobs; a merge that dropped them
			// would silently change how long one tick can run.
			if got.FetchTimeout != base.FetchTimeout || got.StoreTimeout != base.StoreTimeout ||
				got.SeedPageSize != base.SeedPageSize {
				t.Fatalf("untouched limits changed: %+v, want the defaults %+v", got, base)
			}
		})
	}
}

func TestNewQuotaPublishedWorkerRejectsAnUnsetPolicy(t *testing.T) {
	// A policy that never received its defaults would sweep nothing at all, which reads as
	// "the providers are quiet" rather than as a wiring mistake. Refusing it at boot is what
	// makes a missing WithSweepLimits call impossible to ship silently.
	empty := PublishedPollPolicy{FetchTimeout: time.Second, StoreTimeout: time.Second}
	if _, err := NewQuotaPublishedWorker(QuotaPublishedWorkerDeps{
		Quotas: &QuotaService{}, Policy: empty,
	}); err == nil {
		t.Fatal("NewQuotaPublishedWorker() = nil error, want the unset policy refused")
	}
}
