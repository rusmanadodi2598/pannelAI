// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/systemone.go
// @for       The System One (Jev) decision use case: resolve the model, select
//
//	an account, forward the decision payload, and record the call.
//
// @uses      internal/dataplane, internal/domain, internal/registry,
//
//	internal/schema, context, net/http, strings, time.
//
// @reason    SPEC-API-001 §7.15 serves POST /api/v1/systemone for models
//
//	declaring `kind: "systemone"`. The payload is the provider's own
//	vocabulary, so it cannot ride the chat plane: the reference serves it
//	from a separate handler that forwards the body untouched
//	(open-sse/handlers/systemoneCore.js). The use case follows the media
//	plane's shape rather than the chat plane's (one call, no combo chain,
//	one usage row and one log row) because a decision model answers once
//	rather than streaming, which is what the reference's own route does.
//
//	It lives in two files for the AGENTS.md §1.1 budget: this one owns
//	the service and the call, and systemone_target.go owns the URL and
//	header assembly the entry's own block declares.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// SystemOneService implements the decision-model half of SPEC-API-001 §7.15.
type SystemOneService struct {
	resolver KindModelResolver
	router   MediaRouter
	caller   dataplane.MediaCaller
	recorder dataPlaneRecorder
	active   dataplane.ActiveRequests
}

// SystemOneServiceDeps holds the collaborators the service needs.
type SystemOneServiceDeps struct {
	// Resolver asks the plane's own question, so a chat model is refused here
	// rather than served with a decision payload.
	Resolver KindModelResolver
	Router   MediaRouter
	Caller   dataplane.MediaCaller
	// Usage, Logs, Quotas, and RequestID write the §7.12/§7.13 accounting pair
	// and advance the §7.12 counters. All are optional so a deployment that
	// wires none still serves.
	Usage     UsageRecorder
	Logs      RequestLogRecorder
	Quotas    *QuotaCounter
	RequestID RequestIDReader
	// ActiveRequests marks one provider in flight while its call runs, which is
	// what the §7.12 live stream draws.
	ActiveRequests dataplane.ActiveRequests
}

// NewSystemOneService validates deps and returns a ready service.
func NewSystemOneService(deps SystemOneServiceDeps) (*SystemOneService, error) {
	if deps.Resolver == nil {
		return nil, domain.NewValidationError("model resolver is required")
	}
	if deps.Router == nil {
		return nil, domain.NewValidationError("media router is required")
	}
	if deps.Caller == nil {
		return nil, domain.NewValidationError("media caller is required")
	}
	return &SystemOneService{
		resolver: deps.Resolver, router: deps.Router, caller: deps.Caller,
		recorder: newDataPlaneRecorder(deps.Usage, deps.Logs, deps.Quotas, deps.RequestID),
		active:   deps.ActiveRequests,
	}, nil
}

// Decide resolves the model, selects an account, forwards the decision payload,
// and returns the upstream's answer verbatim.
//
// The model must resolve to a provider that declares a `systemone` endpoint: a
// chat model named on this route is refused by name, because the upstream would
// read a decision payload it never receives. The reference applies the same rule
// in its core, which refuses a provider without the block
// (systemoneCore.js:20-26).
//
// A refusal before the call leaves one request log row and no usage row, the same
// shape the embeddings plane gives a request refused before the pipeline ran.
func (s *SystemOneService) Decide(ctx context.Context, req schema.SystemOneRequest, keyID string) (json.RawMessage, error) {
	call, err := s.resolveCall(ctx, req)
	if err != nil {
		s.recorder.refuse(ctx, call.outcome, keyID, err)
		return nil, err
	}
	answer, err := s.perform(ctx, call, keyID)
	if err != nil {
		return nil, err
	}
	return answer, nil
}

// systemOneCall is one resolved decision call: the request to send, the account
// it uses, and the identity its accounting rows carry. The outcome is built
// before the first fallible step so a refusal still names what it reached.
type systemOneCall struct {
	outcome   dataplane.Outcome
	request   dataplane.MediaRequest
	selection dataplane.Selection
}

// resolveCall runs everything before the upstream call: resolve the model, read
// the provider's decision endpoint, and select an account.
func (s *SystemOneService) resolveCall(ctx context.Context, req schema.SystemOneRequest) (systemOneCall, error) {
	resolution, err := s.resolver.ResolveForSystemOne(ctx, req.Model)
	if err != nil {
		return systemOneCall{}, err
	}
	call := systemOneCall{
		outcome: dataplane.Outcome{
			Format:     schema.FormatOpenAI,
			ProviderID: resolution.Provider.ID,
			Model:      resolution.ModelID,
		},
	}
	if resolution.IsCombo() {
		return call, dataplane.ValidationError(
			"model " + req.Model + " is a combo; the decision route requires a single model")
	}
	// The kind guard runs inside resolution (resolve_kind.go), so this route and
	// the chat plane cannot disagree about which models they accept: the resolver
	// refuses a model that does not declare this plane's kind.
	if resolution.Provider.SystemOne == nil || strings.TrimSpace(resolution.Provider.SystemOne.BaseURL) == "" {
		return call, dataplane.ProviderNotRoutable(
			"provider " + resolution.Provider.ID + " does not offer the decision endpoint")
	}

	selection, err := s.router.Select(ctx, resolution.Provider.ID)
	if err != nil {
		return call, err
	}
	call.selection = selection
	call.outcome.EndpointID = selection.Endpoint.ID()

	body, err := req.UpstreamBody(resolution.UpstreamID)
	if err != nil {
		return call, err
	}
	target, headers, err := systemOneTarget(resolution.Provider.SystemOne, selection)
	if err != nil {
		return call, err
	}
	call.request = dataplane.MediaRequest{
		Method: http.MethodPost, URL: target, Headers: headers, Body: body,
	}
	return call, nil
}

// perform runs the outbound call, applies its result to the endpoint's health,
// and records the accounting pair. There is one exit per call, so a request is
// recorded exactly once whichever way it ended.
func (s *SystemOneService) perform(ctx context.Context, call systemOneCall, keyID string) (json.RawMessage, error) {
	started := time.Now()
	release := markActiveRequest(ctx, s.active, call.outcome.ProviderID, call.outcome.EndpointID, call.outcome.Model)
	defer release()

	answer, err := s.caller.Do(ctx, call.request)
	latencyMS := time.Since(started).Milliseconds()

	failure := err
	switch {
	case err != nil:
		// reason: the client's error is the upstream failure, and reporting a
		// bookkeeping failure instead would hide the cause it came from.
		_ = s.router.RecordFailure(ctx, call.selection, "the decision upstream could not be reached", domain.KeyFailureTransient)
	case answer.Status < 200 || answer.Status >= 300:
		failure = dataplane.UpstreamRejected(answer.Status, upstreamMessageOf(answer.Body))
		// reason: the upstream rejection is the client's error; a failed health
		// write retries on the next call rather than replacing this one.
		_ = s.router.RecordFailure(ctx, call.selection, "the decision upstream rejected the request", dataplane.FailureClass(answer.Status))
	default:
		failure = s.router.RecordSuccess(ctx, call.selection)
	}
	// A decision call reports its tokens in the answer rather than a price, so
	// the cost stays zero and the counts come from the answer's own usage block.
	s.recorder.record(ctx, call.outcome, keyID, "0", latencyMS, failure)

	if failure == nil {
		return json.RawMessage(answer.Body), nil
	}
	if err != nil {
		return nil, err
	}
	return nil, failure
}
