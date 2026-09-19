// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/combo_probe_stub_test.go
// @for       The combo test route's seam double, its outcome mapping, and the
//
//	combo it cannot read (SPEC-API-001 §7.7).
//
// @uses      testing, context, errors, internal/dataplane, internal/domain.
// @reason    The stub and the mapping cases are a different group from the
//
//	per-reference table; AGENTS.md §1.1 caps a file at 250 lines, so they
//	moved here rather than trimming the table that pins the route's shape.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// probeAnswer is one reference's configured probe result.
type probeAnswer struct {
	outcome dataplane.Outcome
	err     error
}

// stubProber is the one-method seam double: one answer per ref, and a record of
// the order the service asked in. An unconfigured ref fails loudly rather than
// reading as a healthy one.
type stubProber struct {
	answers map[string]probeAnswer
	calls   []string
}

func (p *stubProber) Ping(_ context.Context, ref string) (dataplane.Outcome, error) {
	p.calls = append(p.calls, ref)
	answer, ok := p.answers[ref]
	if !ok {
		return dataplane.Outcome{}, errors.New("the stub has no answer for " + ref)
	}
	return answer.outcome, answer.err
}

// TestComboTestService_Test_MapsTheProbeOutcome pins that the routing identity
// and latency the pipeline reported reach the wire shape unchanged.
func TestComboTestService_Test_MapsTheProbeOutcome(t *testing.T) {
	ctx := context.Background()
	service, _ := newComboFixture(t, ctx, nil)
	seedComboReferences(t, ctx, service)
	created, err := service.Create(ctx, comboDraft(t, "daily", domain.ComboFallback, 0, "",
		comboRef(t, "openai/gpt-4o", 1)))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	prober := &stubProber{answers: map[string]probeAnswer{
		"openai/gpt-4o": {outcome: dataplane.Outcome{
			ProviderID: "openai", EndpointID: "ep-1", Model: "gpt-4o", LatencyMS: 12,
		}},
	}}
	tests, err := NewComboTestService(service, prober)
	if err != nil {
		t.Fatalf("NewComboTestService() error = %v", err)
	}

	response, err := tests.Test(ctx, created.ID())
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	result := response.Results[0]
	if result.ProviderID != "openai" || result.ModelID != "gpt-4o" || result.EndpointID != "ep-1" {
		t.Fatalf("result identity = %s/%s/%s, want openai/gpt-4o/ep-1",
			result.ProviderID, result.ModelID, result.EndpointID)
	}
	if result.LatencyMS != 12 {
		t.Fatalf("result latency = %d, want 12", result.LatencyMS)
	}
}

// TestComboTestService_Test_UnknownCombo pins that only an unreadable combo is
// an error: the route cannot report references it cannot load.
func TestComboTestService_Test_UnknownCombo(t *testing.T) {
	ctx := context.Background()
	service, _ := newComboFixture(t, ctx, nil)
	tests, err := NewComboTestService(service, &stubProber{})
	if err != nil {
		t.Fatalf("NewComboTestService() error = %v", err)
	}

	if _, err := tests.Test(ctx, "cmb_missing"); !errors.Is(err, domain.ErrComboNotFound) {
		t.Fatalf("Test() error = %v, want ErrComboNotFound", err)
	}
}

// TestNewComboTestService_RequiresDeps pins the constructor's guard: a missing
// dependency must fail at boot, not as a route that reports nothing.
func TestNewComboTestService_RequiresDeps(t *testing.T) {
	ctx := context.Background()
	service, _ := newComboFixture(t, ctx, nil)

	if _, err := NewComboTestService(nil, &stubProber{}); err == nil {
		t.Fatal("NewComboTestService(nil, prober) = nil error, want a validation failure")
	}
	if _, err := NewComboTestService(service, nil); err == nil {
		t.Fatal("NewComboTestService(service, nil) = nil error, want a validation failure")
	}
}
