// How the worker treats a refusal the reference would have raised.
//
// @file      internal/service/quota_published_failure_test.go
// @for       Tests for the worker's handling of a failed (not merely soft) provider answer.
// @uses      context, testing, time.
// @reason    A provider that errors is not a provider that answered. If the worker filed the
//
//	error as a soft answer, the failure run would never grow, the backoff would never engage,
//	and a broken endpoint would be polled on the family floor forever while the cache looked
//	healthy. The sentence still has to be stored and the last good buckets kept, so this is
//	the one place where "keep the data" and "count the failure" have to hold at once.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/quotafetch"
)

func TestPublishedQuotaWorkerCountsARefusalAndKeepsTheLastGoodWindows(t *testing.T) {
	world := newPollWorld(t, 40, 4, []domain.PublishedState{pollState("ep_hard", "github", 30, 0)}, nil)
	world.store.cached["ep_hard"] = domain.PublishedQuota{Windows: []domain.PublishedWindowRow{
		{Label: "Copilot weekly", Used: "4", Total: strPtr("10")},
	}}
	world.fetcher.replies["ep_hard"] = PublishedUsage{
		EndpointID: "ep_hard", ProviderID: "github", FetchedAt: testNow,
		Message: "GitHub quota API error (502): upstream down", Failed: true,
	}

	polled := world.worker.Sweep(context.Background())

	if polled != 0 {
		t.Fatalf("sweep polled = %d, want 0: a refusal is not a successful answer", polled)
	}
	attempt := world.store.attempts["ep_hard"]
	if attempt.FailureDelta != 1 {
		t.Fatalf("failure delta = %d, want 1 so the backoff engages on the next tick", attempt.FailureDelta)
	}
	if attempt.Message == nil || *attempt.Message != "GitHub quota API error (502): upstream down" {
		t.Fatalf("stored message = %v, want the provider's own sentence kept", attempt.Message)
	}
	if kept := len(world.store.cached["ep_hard"].Windows); kept != 1 {
		t.Fatalf("stored windows = %d, want the last good bucket kept through the failure", kept)
	}
	if !attempt.NextAttemptAt.After(testNow) {
		t.Fatalf("next attempt = %v, want it scheduled after the poll that failed", attempt.NextAttemptAt)
	}
}

func TestPublishedQuotaWorkerStillTreatsASentenceAsAnAnswer(t *testing.T) {
	world := newPollWorld(t, 40, 4, []domain.PublishedState{pollState("ep_speak", "github", 30, 0)}, nil)
	world.fetcher.replies["ep_speak"] = PublishedUsage{
		EndpointID: "ep_speak", ProviderID: "github", FetchedAt: testNow,
		Message: "GitHub Copilot connected. Unable to parse quota data.",
	}

	if polled := world.worker.Sweep(context.Background()); polled != 1 {
		t.Fatalf("sweep polled = %d, want 1: a provider that spoke answered", polled)
	}
	if delta := world.store.attempts["ep_speak"].FailureDelta; delta != 0 {
		t.Fatalf("failure delta = %d, want 0, this is the difference between an error and an answer", delta)
	}
}

// The same distinction has to survive the service boundary, because the worker polls through
// `livePublishedUsage`: if the flag were dropped on the way out of the fetcher, every refusal
// would arrive here looking like an answer and no backoff would ever engage.
func TestQuotaService_PublishedUsageKeepsRefusalsApart(t *testing.T) {
	cases := []struct {
		name       string
		result     quotafetch.Result
		wantFailed bool
	}{
		{
			name:       "a provider error is a refusal",
			result:     quotafetch.Result{Message: "GitHub quota API error (502): upstream down", Failed: true},
			wantFailed: true,
		},
		{
			name:       "a sentence the provider chose is an answer",
			result:     quotafetch.Result{Message: "GitHub Copilot connected. Unable to parse quota data."},
			wantFailed: false,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			quotas, fetcher := newPublishedFixture(t,
				oauthPublishedEndpoint(t, "github", "gh-token"), nil,
				usageEntry("github", true), testCase.result)

			usage, err := quotas.PublishedUsage(context.Background(), "ep_pub", true)
			if err != nil {
				t.Fatalf("PublishedUsage() error = %v", err)
			}
			if fetcher.calls != 1 {
				t.Fatalf("provider calls = %d, want 1", fetcher.calls)
			}
			if usage.Failed != testCase.wantFailed {
				t.Fatalf("usage.Failed = %v, want %v (%+v)", usage.Failed, testCase.wantFailed, usage)
			}
			if usage.Message == "" {
				t.Fatal("usage carried no sentence, want the provider's own words kept either way")
			}
		})
	}
}
