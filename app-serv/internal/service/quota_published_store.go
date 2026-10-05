// What one provider poll leaves behind: the answer, the sentence, and the next attempt.
//
// @file      internal/service/quota_published_store.go
// @for       Caching a provider's published answer and stamping its next poll.
// @uses      context, internal/domain, time.
// @reason    A poll has two decisions that read differently: when to ask again, which the worker owns,
//
//	and what a single answer writes, figures, or only a sentence when there are no figures. This file
//	holds the second one, so the pruning rule that protects last-good numbers is stated once.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package service

import (
	"context"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// publishedAttempt starts the row one poll leaves behind. RecordAttempt adds the delta to the
// failure run rather than setting it, so a good poll passes 0 and a failed one passes 1.
func publishedAttempt(state domain.PublishedState, providerID string, now time.Time, failureDelta int) domain.PublishedAttempt {
	return domain.PublishedAttempt{
		EndpointID: state.EndpointID, ProviderID: providerID,
		AttemptedAt: now, FailureDelta: failureDelta,
	}
}

// storeAnswer caches a real answer, then stamps the schedule. StorePublished prunes labels the
// answer does not carry and resets the failure run, so it runs first: an answer that could not
// be written is a failed poll, not a number the screen is showing.
func (w *QuotaPublishedWorker) storeAnswer(ctx context.Context, state domain.PublishedState, providerID string, now time.Time, result PublishedUsage) bool {
	storeCtx, cancel := context.WithTimeout(ctx, w.policy.StoreTimeout)
	err := w.store.StorePublished(storeCtx, publishedAnswer(state.EndpointID, providerID, result))
	cancel()
	if err != nil {
		w.logger.Error("published quota worker could not cache a provider answer",
			"endpoint", state.EndpointID, "provider", providerID, "windows", len(result.Windows), "error", err)
		w.schedule(ctx, publishedAttempt(state, providerID, now, 1),
			publishedNextDelay(providerID, state.ConsecutiveFailures+1))
		return false
	}
	w.schedule(ctx, publishedAttempt(state, providerID, now, 0), publishedNextDelay(providerID, 0))
	return true
}

// storeSoftAnswer covers a sentence with no buckets: the provider refused the credential, the
// account published nothing, the family errored. StorePublished is the port's only window writer
// and it prunes every label an answer lacks, so a zero-window answer would wipe the last good
// buckets off the card. The sentence therefore travels on the attempt, the card's only honest
// content for such an account, while stored numbers keep the older stamp they really have,
// because RecordAttempt never touches fetched_at. A provider that answered in words did answer,
// so its failure run stays where it was.
// storeSoftAnswer writes an answer that carries no buckets. `failureDelta` is 0 when the
// provider spoke (a sentence is still an answer) and 1 when it refused in the way the
// reference throws: the sentence is stored either way, but only the second lengthens the
// failure run, so a family that keeps erroring backs off instead of being polled forever
// on its family floor.
func (w *QuotaPublishedWorker) storeSoftAnswer(ctx context.Context, state domain.PublishedState, providerID string, now time.Time, result PublishedUsage, failureDelta int) {
	attempt := publishedAttempt(state, providerID, now, failureDelta)
	if message := strings.TrimSpace(result.Message); message != "" {
		attempt.Message = &message
		w.logger.Info("published quota provider answered with no buckets",
			"endpoint", state.EndpointID, "provider", providerID, "message", message)
	}
	if plan := strings.TrimSpace(result.Plan); plan != "" {
		attempt.Plan = &plan
	}
	w.schedule(ctx, attempt, publishedNextDelay(providerID, state.ConsecutiveFailures+failureDelta))
}

// schedule writes one endpoint's next attempt, jittered. A schedule that cannot be written is
// logged rather than retried: the row keeps its previous stamp and is polled again.
func (w *QuotaPublishedWorker) schedule(ctx context.Context, attempt domain.PublishedAttempt, delay time.Duration) {
	attempt.NextAttemptAt = attempt.AttemptedAt.Add(w.jitter.delay(delay))
	storeCtx, cancel := context.WithTimeout(ctx, w.policy.StoreTimeout)
	err := w.store.RecordAttempt(storeCtx, attempt)
	cancel()
	if err != nil {
		w.logger.Error("published quota worker could not schedule the next poll", "endpoint", attempt.EndpointID,
			"provider", attempt.ProviderID, "failure_delta", attempt.FailureDelta, "error", err)
	}
}
