// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_call.go
// @for       The shared pipeline of the §7.10 data-plane media routes: resolve the provider and kind, resolve the base URL, select an endpoint, build the target, call, and record the outcome.
// @uses      internal/dataplane, internal/domain, internal/registry, context, strings.
// @reason    Six media routes differ only in their payload and their answer shape; the resolution rules are one rule. Keeping them here means the override precedence, the format gate, and the endpoint-health bookkeeping cannot drift between the routes, and §7.10's "no silent cloud fallback" is enforced once rather than six times.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-19
package service

import (
	"context"
	"strings"

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
	// Quotas advances the §7.12 window counters for every served call. Optional
	// for the same reason as the other two: a deployment without Redis serves
	// and leaves the quota screen empty rather than failing to boot.
	Quotas *QuotaCounter
	// RequestID reads the router's request id, so a media call's usage row and
	// its log row share one identifier (SPEC-API-001 §4).
	RequestID RequestIDReader
	// ActiveRequests marks one provider as in flight while its call runs, which
	// is what the §7.12 live stream draws. Optional: without one the drawing
	// shows no active node rather than the call failing.
	ActiveRequests dataplane.ActiveRequests
}

// MediaCallService performs one §7.10 data-plane media call for any kind.
type MediaCallService struct {
	index     ProviderIndex
	router    MediaRouter
	caller    dataplane.MediaCaller
	overrides MediaOverrideReader
	recorder  dataPlaneRecorder
	active    dataplane.ActiveRequests
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
		recorder: newDataPlaneRecorder(deps.Usage, deps.Logs, deps.Quotas, deps.RequestID),
		active:   deps.ActiveRequests,
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
	// SkipHealth marks a call whose outcome must not reach the endpoint's
	// circuit state. A video poll is the case it exists for: it reads a job that
	// already exists, and a stale task id answering 404 would otherwise park a
	// healthy key for the auth-failure window
	// (docs/PORT/011-PORT-VIDEO-ASYNC-JOB.md D5). The call still writes its
	// usage and log rows.
	SkipHealth bool
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
// when the kind resolved one, and the provider's own id when it did not, a
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
	call, err := s.resolveTarget(ctx, model, kind, true)
	if err != nil {
		return MediaCall{}, err
	}
	selection, err := s.router.Select(ctx, call.ProviderID)
	if err != nil {
		return MediaCall{}, err
	}
	return s.bindCredential(call, selection, query)
}

// resolveTarget resolves the model string, the provider's block for the kind,
// and the effective base URL, without selecting an account: a caller that
// already knows which account it must use (a video poll reads a job that lives
// on the account that created it) resolves the same way and selects its own,
// so the rotation cursor is not moved by a call that never consulted it.
// requireModel is false for that caller, which addresses a service rather than
// one of its models.
func (s *MediaCallService) resolveTarget(ctx context.Context, model string, kind domain.MediaKind, requireModel bool) (MediaCall, error) {
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
	if upstreamModel == "" && requireModel && kindNeedsModel(kind) {
		return MediaCall{}, dataplane.ValidationError("provider " + entry.ID + " needs a model for " + string(kind))
	}
	return MediaCall{ProviderID: entry.ID, UpstreamModel: upstreamModel, Media: media, BaseURL: baseURL}, nil
}

// bindCredential builds the target and headers around one selected account, so
// the credential placement comes from the kind's own block whichever way that
// account was chosen.
func (s *MediaCallService) bindCredential(call MediaCall, selection dataplane.Selection, query map[string]string) (MediaCall, error) {
	target, headers, err := dataplane.MediaTarget(call.Media, call.BaseURL, selection.Credential, query)
	if err != nil {
		return MediaCall{}, err
	}
	call.Selection = selection
	call.Target = target
	call.Headers = headers
	return call, nil
}

// Perform runs one prepared call, reads its answer, records its accounting, and
// applies its answer to the endpoint's health. It lives in media_perform.go,
// with the answer-reading seam a provider-specific failure needs.

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
