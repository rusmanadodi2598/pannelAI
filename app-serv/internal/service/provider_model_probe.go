// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_model_probe.go
// @for       The §7.4 model test: one provider's model probed by id, and a bounded sweep over every chat model that provider offers (draft 017 §4.10, F10).
// @uses      internal/dataplane, internal/domain, internal/registry, internal/schema, context, time.
// @reason    The connection test answers "can this credential reach the provider"; the operator's next question is "does this model answer", which only a per-model probe can tell them. The probe is the real data plane call rather than a second HTTP path, so a model the gateway cannot route never reports healthy from a route the gateway would not take.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-27
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// ProviderModelTestService implements the §7.4 model test routes.
type ProviderModelTestService struct {
	providers *ProviderService
	prober    ModelProber

	// probeTimeout is the ceiling one model may take. It is a field rather than
	// only the constant so a test can exercise "ran out of budget" in
	// milliseconds; the zero value is never used, the constructor fills it.
	probeTimeout time.Duration
}

// NewProviderModelTestService validates deps and returns a ready service. Both
// are required: without the provider read there is no model list to walk, and
// without a prober the route would report a result it never obtained.
func NewProviderModelTestService(providers *ProviderService, prober ModelProber) (*ProviderModelTestService, error) {
	if providers == nil {
		return nil, domain.NewValidationError("provider model test service requires the provider read")
	}
	if prober == nil {
		return nil, domain.NewValidationError("provider model test service requires a data plane prober")
	}
	return &ProviderModelTestService{
		providers: providers, prober: prober, probeTimeout: ProviderModelProbeTimeout,
	}, nil
}

// TestModel probes exactly one model of one provider and reports its row.
//
// The model id is not required to appear in the catalog. A custom node is a
// passthrough: its upstream accepts model strings the registry has never heard
// of, and refusing them would make the node untestable, the pipeline's own
// MODEL_NOT_FOUND is the honest answer for a registry provider that does not
// carry the id. What IS refused is a model the catalog declares as non-chat: the
// probe is a chat call, so an embedding model's failure would describe the probe
// rather than the model.
func (s *ProviderModelTestService) TestModel(
	ctx context.Context, providerID, modelID string,
) (schema.ProviderModelTestResult, error) {
	provider, model, err := s.chatModel(ctx, providerID, modelID)
	if err != nil {
		return schema.ProviderModelTestResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return schema.ProviderModelTestResult{}, err
	}
	return s.probe(ctx, sweepEnd(ctx), provider, model, modelID), nil
}

// TestModels sweeps a provider's chat models in catalog order and reports every
// row, bounded by the limit and the sweep budget.
//
// The probes run one at a time for the reason the combo test states (§7.7): a
// fan-out multiplies what one click spends against the operator's quota and
// makes the reported order depend on which model happened to answer first. A
// model whose probe failed is still a row, the sweep exists to say which ones.
func (s *ProviderModelTestService) TestModels(
	ctx context.Context, providerID string, limit int,
) (schema.ProviderModelTestResponse, error) {
	trimmed := strings.TrimSpace(providerID)
	if trimmed == "" {
		return schema.ProviderModelTestResponse{}, domain.NewValidationError("a provider id is required")
	}
	list, targets, err := s.providers.ProbeTargets(ctx, trimmed)
	if err != nil {
		return schema.ProviderModelTestResponse{}, err
	}
	candidates := chatModels(targets)
	if len(candidates) == 0 {
		return schema.ProviderModelTestResponse{}, domain.NewValidationError(
			"provider " + list.Entry.ID + " offers no chat model to test")
	}

	budget := providerModelTestBudget(limit)
	end := sweepEnd(ctx)
	results := make([]schema.ProviderModelTestResult, 0, budget)
	stopped := ""
	for index, model := range candidates {
		if index >= budget {
			break
		}
		// A model is never started without time left for it: half a probe would
		// report a failure the model did not cause.
		if !time.Now().Before(end) {
			stopped = ProviderModelStoppedDeadline
			break
		}
		results = append(results, s.probe(ctx, end, list.Entry, model, model.ID))
	}

	return schema.ProviderModelTestResponse{
		ProviderID: list.Entry.ID,
		Source:     list.Source,
		Warning:    list.Warning,
		Tested:     len(results),
		Total:      len(candidates),
		Stopped:    stopped,
		Results:    results,
	}, nil
}

// chatModel resolves the provider read and the named model's declaration.
func (s *ProviderModelTestService) chatModel(
	ctx context.Context, providerID, modelID string,
) (registry.Provider, registry.Model, error) {
	trimmedProvider := strings.TrimSpace(providerID)
	if trimmedProvider == "" {
		return registry.Provider{}, registry.Model{}, domain.NewValidationError("a provider id is required")
	}
	trimmedModel := strings.TrimSpace(modelID)
	if trimmedModel == "" {
		return registry.Provider{}, registry.Model{}, domain.NewValidationError("a model id is required")
	}

	list, targets, err := s.providers.ProbeTargets(ctx, trimmedProvider)
	if err != nil {
		return registry.Provider{}, registry.Model{}, err
	}
	model, found := modelByID(targets, trimmedModel)
	if found && !model.IsChat() {
		return registry.Provider{}, registry.Model{}, domain.NewValidationError(
			"model " + trimmedModel + " is not a chat model, so the chat probe does not test it")
	}
	return list.Entry, model, nil
}

// probe runs one bounded data plane call and maps its answer onto a row.
func (s *ProviderModelTestService) probe(
	ctx context.Context, end time.Time, provider registry.Provider, model registry.Model, modelID string,
) schema.ProviderModelTestResult {
	result := schema.ProviderModelTestResult{ModelID: modelID, Name: model.Name}
	ref := provider.ID + "/" + modelID

	probeCtx, cancel := context.WithTimeout(ctx, probeBudget(time.Until(end), s.probeTimeout))
	defer cancel()

	outcome, err := s.prober.Ping(probeCtx, ref)
	result.LatencyMS = outcome.LatencyMS
	result.EndpointID = outcome.EndpointID
	if err == nil {
		result.OK = true
		return result
	}
	// A probe that ran out of its budget is reported as what it is. Calling that
	// an internal failure would tell the operator the gateway broke, when the
	// true answer is that the model did not reply inside the seconds they agreed
	// to wait.
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		result.ErrorCode = ProviderModelTimeoutCode
		result.Error = providerModelTimeoutMessage
		return result
	}
	failure := dataplane.AsError(err)
	result.ErrorCode = failure.Code
	result.Error = failure.Message
	result.Status = failure.Status
	return result
}
