// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_call.go
// @for       The shared pipeline of the §7.10 data-plane media routes: resolve
//
//	the provider and kind, resolve the base URL, select an endpoint, build
//	the target, call, and record the outcome.
//
// @uses      internal/dataplane, internal/domain, internal/registry, context,
//
//	strings.
//
// @reason    Six media routes differ only in their payload and their answer
//
//	shape; the resolution rules are one rule. Keeping them here means
//	the override precedence, the format gate, and the endpoint-health
//	bookkeeping cannot drift between the routes — and §7.10's "no silent
//	cloud fallback" is enforced once rather than six times.
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
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// MediaCallServiceDeps holds the collaborators every media route needs.
type MediaCallServiceDeps struct {
	Index     ProviderIndex
	Router    MediaRouter
	Caller    dataplane.MediaCaller
	Overrides MediaOverrideReader
	// Usage and Logs write the §7.12/§7.13 accounting pair for every call that
	// reaches an upstream. Both are optional so a deployment that wires none
	// still serves.
	Usage UsageRecorder
	Logs  RequestLogRecorder
	// RequestID reads the router's request id, so a media call's usage row and
	// its log row share one identifier (SPEC-API-001 §4).
	RequestID RequestIDReader
}

// MediaCallService performs one §7.10 data-plane media call for any kind.
type MediaCallService struct {
	index     ProviderIndex
	router    MediaRouter
	caller    dataplane.MediaCaller
	overrides MediaOverrideReader
	recorder  dataPlaneRecorder
}

// NewMediaCallService validates deps and returns a ready service. The override
// reader is optional: without one the registry's own base URLs apply.
func NewMediaCallService(deps MediaCallServiceDeps) (*MediaCallService, error) {
	if deps.Index == nil {
		return nil, domain.NewValidationError("media call service requires a provider index")
	}
	if deps.Router == nil {
		return nil, domain.NewValidationError("media call service requires a media router")
	}
	if deps.Caller == nil {
		return nil, domain.NewValidationError("media call service requires a media caller")
	}
	return &MediaCallService{
		index: deps.Index, router: deps.Router, caller: deps.Caller, overrides: deps.Overrides,
		recorder: newDataPlaneRecorder(deps.Usage, deps.Logs, deps.RequestID),
	}, nil
}

// MediaCall is one resolved media call: who answers, where, with which
// credential, and under which upstream model.
type MediaCall struct {
	// ProviderID is the registry entry that answers.
	ProviderID string
	// UpstreamModel is the model id the upstream expects, with the registry's
	// default applied when the client named none.
	UpstreamModel string
	// Media is the kind's block, which fixes the credential placement and
	// carries the per-query price a search call records.
	Media registry.MediaConfig
	// BaseURL is the effective base URL: the stored override when one exists,
	// the registry's own value otherwise.
	BaseURL   string
	Target    string
	Headers   map[string]string
	Selection dataplane.Selection
}

// Outcome describes the served call for the usage and log recorders.
func (c MediaCall) Outcome() dataplane.Outcome {
	return dataplane.Outcome{
		Format:     schema.FormatOpenAI,
		ProviderID: c.ProviderID,
		EndpointID: c.Selection.Endpoint.ID(),
		Model:      c.accountingModel(),
	}
}

// accountingModel is the model id a recorded row carries: the upstream model
// when the kind resolved one, and the provider's own id when it did not — a
// search call names no model, and the usage row's model column is required.
func (c MediaCall) accountingModel() string {
	if model := strings.TrimSpace(c.UpstreamModel); model != "" {
		return model
	}
	return c.ProviderID
}

// Prepare resolves a media model string into a call target.
//
// The media plane has no aliases and no combos: §7.10's routes address a
// provider's own service, so the model string is `provider/model` and a bare
// model id is refused by name rather than guessed at.
func (s *MediaCallService) Prepare(ctx context.Context, model string, kind domain.MediaKind, query map[string]string) (MediaCall, error) {
	providerID, upstreamModel, err := splitMediaModel(model)
	if err != nil {
		return MediaCall{}, err
	}
	entry, media, err := s.providerBlock(providerID, kind)
	if err != nil {
		return MediaCall{}, err
	}
	baseURL, err := s.effectiveBaseURL(ctx, entry.ID, kind, media)
	if err != nil {
		return MediaCall{}, err
	}
	if upstreamModel == "" {
		upstreamModel = strings.TrimSpace(media.DefaultModel)
	}
	if upstreamModel == "" && kindNeedsModel(kind) {
		return MediaCall{}, dataplane.ValidationError("provider " + entry.ID + " needs a model for " + string(kind))
	}

	selection, err := s.router.Select(ctx, entry.ID)
	if err != nil {
		return MediaCall{}, err
	}
	target, headers, err := dataplane.MediaTarget(media, baseURL, selection.Credential, query)
	if err != nil {
		return MediaCall{}, err
	}
	return MediaCall{
		ProviderID: entry.ID, UpstreamModel: upstreamModel, Media: media,
		BaseURL: baseURL, Target: target, Headers: headers, Selection: selection,
	}, nil
}

// Perform runs one prepared call, records its accounting, and applies its answer
// to the endpoint's health, so a media route feeds the same circuit state the
// chat plane reads. The key id is the authenticated caller's, recorded on both
// rows the call writes (SPEC-API-001 §7.12/§7.13).
func (s *MediaCallService) Perform(ctx context.Context, call MediaCall, request dataplane.MediaRequest, keyID string) (dataplane.MediaResponse, error) {
	request.URL = call.Target
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
		failure = s.router.RecordSuccess(ctx, call.Selection)
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

// effectiveBaseURL resolves where the kind is dialed: the operator's stored
// override wins, the registry's own value is the default, and neither is a
// refusal rather than a guess (§7.10 forbids a silent cloud fallback).
func (s *MediaCallService) effectiveBaseURL(ctx context.Context, providerID string, kind domain.MediaKind, media registry.MediaConfig) (string, error) {
	baseURL := strings.TrimSpace(media.BaseURL)
	if s.overrides != nil {
		stored, err := s.overrides.MediaBaseURL(ctx, providerID, kind)
		if err != nil {
			return "", err
		}
		if stored = strings.TrimSpace(stored); stored != "" {
			baseURL = stored
		}
	}
	if baseURL == "" {
		return "", dataplane.ValidationError(
			"provider " + providerID + " has no " + string(kind) + " base_url configured")
	}
	return baseURL, nil
}
