// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/combo_probe_test.go
// @for       HTTP tests for POST /api/v1/combos/{id}/test (SPEC-API-001 §7.7).
// @uses      net/http, testing, context, internal/dataplane.
// @reason    AGENTS.md §2.1 requires a happy path and a validation-failure path
//
//	per route. This route's failure path is unusual — a dead member must
//	still answer 200, because the failure is the finding — so the test
//	pins that distinction explicitly rather than leaving it to the
//	service tests.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
)

// stubProber is the combo test route's one-method seam double: a healthy answer
// per ref, plus per-ref failures a case can turn on.
type stubProber struct {
	answers map[string]dataplane.Outcome
	fail    map[string]error
}

func (p *stubProber) Ping(_ context.Context, ref string) (dataplane.Outcome, error) {
	if err := p.fail[ref]; err != nil {
		return dataplane.Outcome{}, err
	}
	return p.answers[ref], nil
}

// newStubProber is the fixture's default: the seeded combo's one member
// answers healthy, so a case that wants a failure asks for one.
func newStubProber() *stubProber {
	return &stubProber{answers: map[string]dataplane.Outcome{
		"openai/gpt-4o": {ProviderID: "openai", EndpointID: "ep-1", Model: "gpt-4o", LatencyMS: 12},
	}}
}

// TestComboTestHandler_ReportsEveryReference covers the happy path: the route
// answers 200 with the combo's identity and one result per reference.
func TestComboTestHandler_ReportsEveryReference(t *testing.T) {
	f := newManagementFixture(t)
	rr := do(t, http.MethodPost, "/api/v1/combos/cmb_seeded/test", "",
		withPathID(f.comboTest.Test, "cmb_seeded"))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}

	body := decodeBody(t, rr)
	if body["combo_id"] != "cmb_seeded" || body["combo"] != "seeded-combo" || body["strategy"] != "fallback" {
		t.Fatalf("identity = %v/%v/%v, want cmb_seeded/seeded-combo/fallback",
			body["combo_id"], body["combo"], body["strategy"])
	}
	results, ok := body["results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("results = %v, want one entry", body["results"])
	}
	result, _ := results[0].(map[string]any)
	if result["ref"] != "openai/gpt-4o" || result["role"] != "model" || result["ok"] != true {
		t.Fatalf("result = %v, want the healthy member reported as such", result)
	}
	if result["provider_id"] != "openai" || result["endpoint_id"] != "ep-1" || result["model_id"] != "gpt-4o" {
		t.Fatalf("result identity = %v, want openai/ep-1/gpt-4o", result)
	}
	if result["latency_ms"] != float64(12) {
		t.Fatalf("result latency = %v, want 12", result["latency_ms"])
	}
}

// TestComboTestHandler_ADeadMemberIsAResult pins the route's contract: a member
// that failed its probe is reported in the answer, and the route still succeeds,
// because which member is down is the finding the caller asked for.
func TestComboTestHandler_ADeadMemberIsAResult(t *testing.T) {
	f := newManagementFixture(t)
	f.prober.fail = map[string]error{
		"openai/gpt-4o": &dataplane.Error{Code: dataplane.CodeUpstreamError, Message: "the member is down"},
	}
	rr := do(t, http.MethodPost, "/api/v1/combos/cmb_seeded/test", "",
		withPathID(f.comboTest.Test, "cmb_seeded"))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}

	result, _ := decodeBody(t, rr)["results"].([]any)[0].(map[string]any)
	if result["ok"] != false {
		t.Fatalf("result ok = %v, want false", result["ok"])
	}
	if result["error_code"] != dataplane.CodeUpstreamError || result["error"] != "the member is down" {
		t.Fatalf("result error = %v/%v, want the data plane code and message",
			result["error_code"], result["error"])
	}
}

// TestComboTestHandler_UnknownCombo pins that an unreadable combo is the route's
// only error: there are no references to report.
func TestComboTestHandler_UnknownCombo(t *testing.T) {
	f := newManagementFixture(t)
	rr := do(t, http.MethodPost, "/api/v1/combos/cmb_missing/test", "",
		withPathID(f.comboTest.Test, "cmb_missing"))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body: %s)", rr.Code, rr.Body.String())
	}
}

// TestComboTestHandler_MissingID covers the path-value guard.
func TestComboTestHandler_MissingID(t *testing.T) {
	f := newManagementFixture(t)
	rr := do(t, http.MethodPost, "/api/v1/combos//test", "", f.comboTest.Test)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
	}
}
