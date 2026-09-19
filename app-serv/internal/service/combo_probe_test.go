// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/combo_probe_test.go
// @for       The combo test route's use case: per-reference probing, the judge's
//
//	place, and the order the probes run in (SPEC-API-001 §7.7).
//
// @uses      testing, context, internal/dataplane, internal/domain, internal/schema.
// @reason    §7.7 makes the answer per reference, so the test pins the order, the
//
//	roles, and that a dead member is a result rather than a refusal — the
//	three properties an operator reads the route for. The seam double and
//	the mapping cases live in the sibling file, split at the AGENTS.md
//	§1.1 line limit.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestComboTestService_Test pins the answer's shape: every reference the combo
// depends on is probed in stored order, a fusion judge is probed last, and a
// failing probe is reported as a result rather than failing the route.
func TestComboTestService_Test(t *testing.T) {
	ctx := context.Background()
	down := &dataplane.Error{Code: dataplane.CodeUpstreamError, Message: "the member is down"}
	healthy := dataplane.Outcome{ProviderID: "openai", EndpointID: "ep-1", Model: "gpt-4o", LatencyMS: 12}
	healthyMini := dataplane.Outcome{ProviderID: "openai", EndpointID: "ep-2", Model: "gpt-4o-mini", LatencyMS: 9}

	cases := []struct {
		name      string
		strategy  domain.ComboStrategy
		sticky    int
		judge     string
		models    []domain.ComboModel
		answers   map[string]probeAnswer
		wantRefs  []string
		wantRoles []string
		wantOK    []bool
		wantCodes []string
	}{
		{
			name:     "every member answers",
			strategy: domain.ComboFallback,
			models:   []domain.ComboModel{comboRef(t, "openai/gpt-4o", 1), comboRef(t, "openai/gpt-4o-mini", 2)},
			answers: map[string]probeAnswer{
				"openai/gpt-4o":      {outcome: healthy},
				"openai/gpt-4o-mini": {outcome: healthyMini},
			},
			wantRefs:  []string{"openai/gpt-4o", "openai/gpt-4o-mini"},
			wantRoles: []string{schema.ComboTestRoleModel, schema.ComboTestRoleModel},
			wantOK:    []bool{true, true},
			wantCodes: []string{"", ""},
		},
		{
			name:     "one dead member does not hide the healthy one",
			strategy: domain.ComboFallback,
			models:   []domain.ComboModel{comboRef(t, "openai/gpt-4o", 1), comboRef(t, "openai/gpt-4o-mini", 2)},
			answers: map[string]probeAnswer{
				"openai/gpt-4o":      {err: down},
				"openai/gpt-4o-mini": {outcome: healthyMini},
			},
			wantRefs:  []string{"openai/gpt-4o", "openai/gpt-4o-mini"},
			wantRoles: []string{schema.ComboTestRoleModel, schema.ComboTestRoleModel},
			wantOK:    []bool{false, true},
			wantCodes: []string{dataplane.CodeUpstreamError, ""},
		},
		{
			name:     "a fusion combo probes its judge last",
			strategy: domain.ComboFusion,
			judge:    "openai/gpt-4o-mini",
			models:   []domain.ComboModel{comboRef(t, "openai/gpt-4o", 1)},
			answers: map[string]probeAnswer{
				"openai/gpt-4o":      {outcome: healthy},
				"openai/gpt-4o-mini": {outcome: healthyMini},
			},
			wantRefs:  []string{"openai/gpt-4o", "openai/gpt-4o-mini"},
			wantRoles: []string{schema.ComboTestRoleModel, schema.ComboTestRoleJudge},
			wantOK:    []bool{true, true},
			wantCodes: []string{"", ""},
		},
		{
			name:     "a dead judge is reported too",
			strategy: domain.ComboFusion,
			judge:    "openai/gpt-4o-mini",
			models:   []domain.ComboModel{comboRef(t, "openai/gpt-4o", 1)},
			answers: map[string]probeAnswer{
				"openai/gpt-4o":      {outcome: healthy},
				"openai/gpt-4o-mini": {err: down},
			},
			wantRefs:  []string{"openai/gpt-4o", "openai/gpt-4o-mini"},
			wantRoles: []string{schema.ComboTestRoleModel, schema.ComboTestRoleJudge},
			wantOK:    []bool{true, false},
			wantCodes: []string{"", dataplane.CodeUpstreamError},
		},
		{
			name:     "a rotating combo probes the stored order",
			strategy: domain.ComboRoundRobin,
			sticky:   2,
			models:   []domain.ComboModel{comboRef(t, "openai/gpt-4o", 1), comboRef(t, "openai/gpt-4o-mini", 2)},
			answers: map[string]probeAnswer{
				"openai/gpt-4o":      {outcome: healthy},
				"openai/gpt-4o-mini": {outcome: healthyMini},
			},
			wantRefs:  []string{"openai/gpt-4o", "openai/gpt-4o-mini"},
			wantRoles: []string{schema.ComboTestRoleModel, schema.ComboTestRoleModel},
			wantOK:    []bool{true, true},
			wantCodes: []string{"", ""},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, _ := newComboFixture(t, ctx, nil)
			seedComboReferences(t, ctx, service)
			created, err := service.Create(ctx, comboDraft(t, "daily", tc.strategy, tc.sticky, tc.judge, tc.models...))
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			prober := &stubProber{answers: tc.answers}
			tests, err := NewComboTestService(service, prober)
			if err != nil {
				t.Fatalf("NewComboTestService() error = %v", err)
			}

			response, err := tests.Test(ctx, created.ID())
			if err != nil {
				t.Fatalf("Test() error = %v", err)
			}
			if response.ComboID != created.ID() || response.Combo != "daily" || response.Strategy != string(tc.strategy) {
				t.Fatalf("Test() identity = %s/%s/%s, want %s/daily/%s",
					response.ComboID, response.Combo, response.Strategy, created.ID(), tc.strategy)
			}
			if !equalStrings(prober.calls, tc.wantRefs) {
				t.Fatalf("probed refs = %v, want %v", prober.calls, tc.wantRefs)
			}
			if len(response.Results) != len(tc.wantRefs) {
				t.Fatalf("results = %d, want %d", len(response.Results), len(tc.wantRefs))
			}
			for index, result := range response.Results {
				if result.Ref != tc.wantRefs[index] || result.Role != tc.wantRoles[index] {
					t.Fatalf("result[%d] = %s/%s, want %s/%s",
						index, result.Ref, result.Role, tc.wantRefs[index], tc.wantRoles[index])
				}
				if result.OK != tc.wantOK[index] || result.ErrorCode != tc.wantCodes[index] {
					t.Fatalf("result[%d] ok/code = %v/%q, want %v/%q",
						index, result.OK, result.ErrorCode, tc.wantOK[index], tc.wantCodes[index])
				}
			}
		})
	}
}
