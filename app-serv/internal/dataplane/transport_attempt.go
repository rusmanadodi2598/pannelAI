// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/transport_attempt.go
// @for       One outbound call to one upstream target: its deadlines, its reading of the answer, and the classification stamped while the refusal is still in hand.
// @uses      bytes, context, errors, io, net/http, net/url, time, internal/provider.
// @reason    Do owns the decision of whether to ask again; this file owns the asking. Splitting them keeps the retry loop readable as a policy, and puts the classification where the status, the headers and the body are all still present, which is the only moment a refusal can be recognised without guessing again later.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package dataplane

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
)

// attempt performs exactly one outbound call and classifies its outcome, so Do
// only has to decide. deadline is what attemptDeadline reported: zero for a
// streamed client, which SPEC-API-001 §4 bounds by the idle read instead.
// firstByte bounds how long a streamed answer may stay silent before its first
// byte; zero disables the wait (a connector whose provider is known to answer
// promptly never needs it, and a test may pin the other bounds alone).
func (t *Transport) attempt(ctx context.Context, plugin provider.Plugin, call Call, url string, deadline, firstByte time.Duration) (*Upstream, *UpstreamError, error) {
	var (
		attemptCtx context.Context
		cancel     context.CancelFunc
	)
	if deadline > 0 {
		attemptCtx, cancel = context.WithTimeout(ctx, deadline)
	} else {
		attemptCtx, cancel = context.WithCancel(ctx)
	}

	request, err := http.NewRequestWithContext(attemptCtx, http.MethodPost, url, bytes.NewReader(call.Body))
	if err != nil {
		cancel()
		return nil, nil, wrapDataPlaneError(CodeUpstreamError, "upstream request could not be built", err)
	}
	applyHeaders(request.Header, call)
	if err := plugin.ApplyAuth(request, call.Credential); err != nil {
		cancel()
		return nil, nil, wrapDataPlaneError(CodeUpstreamError, "upstream credential could not be applied", err)
	}

	// The attempt's context travels with the request: a proxy walk clones the
	// request with the context it is handed, so passing the caller's context
	// here would replace the attempt deadline (and the idle guard's cancel)
	// with one that has neither.
	response, err := t.dialer.Do(attemptCtx, request, call.Provider.ID)
	if err != nil {
		cancel()
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, nil, timeoutError(err)
		}
		return nil, nil, wrapDataPlaneError(CodeUpstreamError, "the upstream could not be reached", err)
	}

	usage := plugin.DecodeUsage(response.StatusCode, response.Header)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		failure := readUpstreamError(response)
		cancel()
		classifyFailure(failure, call.Provider.ID, url)
		return nil, failure, nil
	}

	if call.Stream {
		// A provider that wraps its answer in an envelope states its real status
		// inside the body, so the first frame is read before anything is piped:
		// a refusal there is an upstream failure the router can act on, not a
		// served answer that gets billed.
		stream := response.Body
		if envelope, ok := plugin.(provider.StreamEnvelope); ok {
			unwrapped, failure := envelope.OpenStream(response.Body)
			if failure != nil {
				cancel()
				// Nothing was piped, so the call is safe to repeat: that is what
				// Replayable below tells the budget.
				refusal := &UpstreamError{
					Status:     failure.Status,
					Header:     response.Header,
					Message:    failure.Message,
					Body:       []byte(failure.Message),
					Replayable: true,
					// Quota is the connector's own reading of the envelope: a
					// spent account wearing a generic status, which no pattern
					// over the body could tell from a throttle.
					Quota: failure.Quota,
				}
				classifyFailure(refusal, call.Provider.ID, url)
				return nil, refusal, nil
			}
			stream = unwrapped
		}
		// The first byte is awaited here, inside the attempt, because only the
		// transport can turn a silent upstream into a retry against the same
		// target and a failover to the next credential: once a body is handed
		// out, a read that stalls is the caller's to report, not the walk's.
		if firstByte > 0 {
			guarded, err := firstByteWithin(stream, firstByte, cancel)
			if err != nil {
				cancel()
				// reason: the read already failed, so a close error adds
				// nothing to the timeout being reported.
				_ = stream.Close()
				return nil, nil, err
			}
			stream = guarded
		}
		// The guard owns the cancel: it fires when reads stall, and it releases
		// the context when the caller closes the body.
		return &Upstream{
			Status: response.StatusCode,
			Header: response.Header,
			Usage:  usage,
			Body:   newIdleGuard(stream, idleTimeout(call.Provider), cancel),
		}, nil, nil
	}

	// A non-streamed body is buffered so the context can be released before the
	// caller reads it, which lets the total deadline bound the whole exchange.
	body, readErr := readBounded(response.Body, maxUpstreamBodyBytes)
	cancel()
	if readErr != nil {
		// reason: the body is fully consumed or the limit was hit, so a close
		// error adds nothing to the read error already reported.
		_ = response.Body.Close()
		return nil, nil, wrapDataPlaneError(CodeUpstreamError, "the upstream response could not be read", readErr)
	}
	if closeErr := response.Body.Close(); closeErr != nil {
		return nil, nil, wrapDataPlaneError(CodeUpstreamError, "the upstream response could not be read", closeErr)
	}
	return &Upstream{
		Status: response.StatusCode,
		Header: response.Header,
		Usage:  usage,
		Body:   io.NopCloser(bytes.NewReader(body)),
	}, nil, nil
}

// classifyFailure stamps a refusal with what it was recognised as, once, where
// the status, the headers and the body are all still in hand. The park and the
// client's error both read the stamp, so they cannot disagree about one
// failure: a 429 that is really a spent allowance is parked to the vendor's own
// reset and reported as one, not parked for five minutes and reported as a
// throttle.
func classifyFailure(failure *UpstreamError, providerID, raw string) {
	if failure == nil {
		return
	}
	verdict := provider.Classify(provider.Failure{
		Provider: providerID,
		Host:     hostOf(raw),
		Status:   failure.Status,
		Header:   failure.Header,
		Body:     failure.Body,
		Quota:    failure.Quota,
	})
	failure.Class = verdict.Class
	failure.RetryAfter = verdict.RetryAfter
	failure.ResetAt = verdict.ResetAt
	failure.ResetSource = verdict.ResetSource
	failure.SignatureID = verdict.SignatureID
	failure.Policy = verdict.Policy
	failure.Retryable = verdict.Retryable()
}

// hostOf answers the hostname an upstream URL names, which is what scopes a
// signature to a vendor whose node id the operator chose.
func hostOf(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}
