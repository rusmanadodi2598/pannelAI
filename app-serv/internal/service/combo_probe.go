// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/combo_probe.go
// @for       The combo test route: a bounded probe of every reference a combo
//
//	depends on (SPEC-API-001 §7.7).
//
// @uses      internal/dataplane, internal/domain, internal/schema, context.
// @reason    §7.7 makes the test a diagnostic, and "the combo works" is not
//
//	actionable when one of five models is dead: each reference's own
//	result is the answer, so the probe reports per reference instead of
//	the combo's aggregate outcome. It is a service of its own because it
//	needs the data plane, and the data plane is built after the combo
//	CRUD service — the engine asks that service for the round-robin
//	order.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// ComboProber runs one probe against one model reference and reports what the
// data plane answered. It is a one-method seam so this service is testable
// without an engine, a registry, or an upstream.
type ComboProber interface {
	Ping(ctx context.Context, ref string) (dataplane.Outcome, error)
}

// ComboTestService implements the combo test route (§7.7).
type ComboTestService struct {
	combos *ComboService
	prober ComboProber
}

// NewComboTestService validates deps and returns a ready service. Both are
// required: without the combo service there is nothing to probe, and without a
// prober the route would report a result it never obtained.
func NewComboTestService(combos *ComboService, prober ComboProber) (*ComboTestService, error) {
	if combos == nil {
		return nil, domain.NewValidationError("combo test service requires the combo service")
	}
	if prober == nil {
		return nil, domain.NewValidationError("combo test service requires a data plane prober")
	}
	return &ComboTestService{combos: combos, prober: prober}, nil
}

// Test probes every reference the combo depends on, in the order the combo
// stores them, and reports each result.
//
// The probes run one at a time: the route is a diagnostic an operator waits on,
// so a fan-out would multiply the accounts one click spends while making the
// reported order depend on which model happened to answer first. A failed probe
// is a result, not an error — refusing the whole answer because one member is
// down would hide the members that are up.
func (s *ComboTestService) Test(ctx context.Context, id string) (schema.ComboTestResponse, error) {
	combo, err := s.combos.Get(ctx, id)
	if err != nil {
		return schema.ComboTestResponse{}, err
	}
	results := make([]schema.ComboTestResult, 0, combo.ModelCount()+1)
	for _, ref := range combo.Refs() {
		results = append(results, s.probe(ctx, ref, schema.ComboTestRoleModel))
	}
	// A fusion combo's judge is part of the chain the request depends on, so it
	// is probed too — a dead judge is the failure a panel test would otherwise
	// report as five healthy models.
	if combo.Strategy() == domain.ComboFusion && combo.JudgeModel() != "" {
		results = append(results, s.probe(ctx, combo.JudgeModel(), schema.ComboTestRoleJudge))
	}
	return schema.ComboTestResponse{
		ComboID:  combo.ID(),
		Combo:    combo.Name(),
		Strategy: string(combo.Strategy()),
		Results:  results,
	}, nil
}

// probe runs one probe and maps its outcome onto the wire shape.
func (s *ComboTestService) probe(ctx context.Context, ref, role string) schema.ComboTestResult {
	outcome, err := s.prober.Ping(ctx, ref)
	result := schema.ComboTestResult{
		Ref:        ref,
		Role:       role,
		OK:         err == nil,
		ProviderID: outcome.ProviderID,
		ModelID:    outcome.Model,
		EndpointID: outcome.EndpointID,
		LatencyMS:  outcome.LatencyMS,
	}
	if err != nil {
		failure := dataplane.AsError(err)
		result.ErrorCode = failure.Code
		result.Error = failure.Message
	}
	return result
}
