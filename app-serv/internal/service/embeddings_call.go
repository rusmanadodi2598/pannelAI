// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/embeddings_call.go
// @for       The outbound half of the embeddings use case: one call, its
//
//	endpoint health, and its accounting pair.
//
// @uses      internal/dataplane, context, time.
// @reason    Embed already resolves and shapes the request; the call itself
//
//	classifies its own outcome three ways (unreachable, rejected, served)
//	and each way owes the client an error, the endpoint a health report,
//	and the accounting one usage row plus one log. Keeping that in one
//	place is what stops a fourth return path from forgetting the record,
//	and it keeps embeddings.go under the §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
)

// perform runs one outbound embeddings call, applies its result to the
// endpoint's health, and records the call's accounting.
//
// There is one exit per call, so a request is recorded exactly once whichever
// way it ended. The caller's error is the client's error; a failed health or
// accounting write is bookkeeping that retries on the next call rather than
// replacing the cause the client must see.
func (s *EmbeddingsService) perform(ctx context.Context, request dataplane.MediaRequest, selection dataplane.Selection, outcome dataplane.Outcome, keyID string) (dataplane.MediaResponse, error) {
	started := time.Now()
	answer, err := s.caller.Do(ctx, request)
	latencyMS := time.Since(started).Milliseconds()

	failure := err
	switch {
	case err != nil:
		// reason: the client's error is the upstream failure, and reporting a
		// bookkeeping failure instead would hide the cause it came from.
		_ = s.router.RecordFailure(ctx, selection, "the embeddings upstream could not be reached")
	case answer.Status < 200 || answer.Status >= 300:
		failure = dataplane.UpstreamRejected(answer.Status, upstreamMessageOf(answer.Body))
		// reason: the upstream rejection is the client's error; a failed health
		// write retries on the next call rather than replacing this one.
		_ = s.router.RecordFailure(ctx, selection, "the embeddings upstream rejected the request")
	default:
		failure = s.router.RecordSuccess(ctx, selection)
	}
	// An embeddings call has no per-query price and no token counts yet, so it
	// records the same zero cost the media routes do.
	s.recorder.record(ctx, outcome, keyID, "0", latencyMS, failure)

	if failure == nil {
		return answer, nil
	}
	if err != nil {
		return dataplane.MediaResponse{}, err
	}
	return answer, failure
}
