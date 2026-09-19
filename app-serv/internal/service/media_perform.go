// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_perform.go
// @for       Running one prepared §7.10 call: the outbound request, the answer
//
//	read, the endpoint's health, and the accounting row.
//
// @uses      internal/dataplane, internal/domain, context, strings, time.
// @reason    A provider can report a failure inside a 200 — MiniMax answers a
//
//	`base_resp.status_code` envelope and Gemini can answer no audio at
//	all — so reading the answer is part of classifying it. Keeping the
//	read here means such a call is recorded the way the client
//	experienced it, rather than as a served call (the G17 rule applied
//	to the media plane), and it is separate from media_call.go so both
//	files stay inside the AGENTS.md §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
)

// mediaAnswerReader maps one upstream answer into the bytes a route returns. A
// nil reader passes the answer through, which is what the kinds whose answer is
// already the route's own shape use.
type mediaAnswerReader func(status int, body []byte) ([]byte, error)

// Perform runs one prepared call, reads its answer, records its accounting, and
// applies its answer to the endpoint's health, so a media route feeds the same
// circuit state the chat plane reads. The key id is the authenticated caller's,
// recorded on both rows the call writes (SPEC-API-001 §7.12/§7.13).
//
// A request may carry a URL of its own: a provider whose endpoint names the
// model or the voice in its path builds it from the prepared target.
func (s *MediaCallService) Perform(ctx context.Context, call MediaCall, request dataplane.MediaRequest, keyID string, read mediaAnswerReader) (dataplane.MediaResponse, error) {
	if strings.TrimSpace(request.URL) == "" {
		request.URL = call.Target
	}
	request.Headers = mergeHeaders(call.Headers, request.Headers)
	if request.TimeoutMS == 0 {
		request.TimeoutMS = call.Media.TimeoutMS
	}

	started := time.Now()
	answer, err := s.caller.Do(ctx, request)
	latencyMS := time.Since(started).Milliseconds()

	// One exit from the classification, so a call is recorded exactly once
	// whichever way it ended: unreachable, rejected, or served.
	failure := err
	switch {
	case err != nil:
		// reason: the client's error is the upstream failure; a failed health
		// write retries on the next call rather than replacing this one.
		_ = s.router.RecordFailure(ctx, call.Selection, "the media upstream could not be reached")
	case answer.Status < 200 || answer.Status >= 300:
		failure = dataplane.UpstreamRejected(answer.Status, upstreamMessageOf(answer.Body))
		// reason: same as above — the upstream rejection is what the client
		// must see, and the health write is bookkeeping.
		_ = s.router.RecordFailure(ctx, call.Selection, "the media upstream rejected the request")
	default:
		failure = s.served(ctx, call, read, &answer)
	}
	s.recorder.record(ctx, call.Outcome(), keyID, mediaCost(call.Media.CostPerQuery), latencyMS, failure)

	if failure == nil {
		return answer, nil
	}
	if err != nil {
		return dataplane.MediaResponse{}, err
	}
	return answer, failure
}

// served reads a 2xx answer and applies it to the endpoint's health. A reader
// that refuses the answer turns the 200 into the rejection the client received,
// so the usage row cannot call a failed call served.
func (s *MediaCallService) served(ctx context.Context, call MediaCall, read mediaAnswerReader, answer *dataplane.MediaResponse) error {
	if read != nil {
		body, err := read(answer.Status, answer.Body)
		if err != nil {
			// reason: the reader's error is the upstream failure the client
			// must see; the health write is bookkeeping and retries next call.
			_ = s.router.RecordFailure(ctx, call.Selection, "the media upstream rejected the request")
			return err
		}
		answer.Body = body
	}
	return s.router.RecordSuccess(ctx, call.Selection)
}
