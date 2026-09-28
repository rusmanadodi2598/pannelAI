// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_model_probe_test.go
// @for       Table-driven tests for the provider model test service (draft 017 §4.10, F10).
// @uses      internal/dataplane, internal/registry, internal/schema, context, testing, time.
// @reason    The route's contract is "one result per model, whatever the upstream
//
//	did": a dead model is an answer, an unknown provider is an error, and
//	a sweep that runs out of budget reports what it managed to test. Only
//	a stubbed prober can show all three without a live provider.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-27
package service

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// probeStub is the model test seam double: a scripted answer per reference, plus
// a per-reference failure and an optional delay so a budget case can run out.
type probeStub struct {
	answers map[string]dataplane.Outcome
	fail    map[string]error
	delay   time.Duration
	refs    []string
}

func (p *probeStub) Ping(ctx context.Context, ref string) (dataplane.Outcome, error) {
	p.refs = append(p.refs, ref)
	if p.delay > 0 {
		select {
		case <-time.After(p.delay):
		case <-ctx.Done():
			return dataplane.Outcome{}, ctx.Err()
		}
	}
	if err := p.fail[ref]; err != nil {
		return dataplane.Outcome{}, err
	}
	return p.answers[ref], nil
}

// newProbeStub seeds the two models the shared fixture declares as healthy.
func newProbeStub() *probeStub {
	return &probeStub{answers: map[string]dataplane.Outcome{
		"openai/gpt-4o":      {ProviderID: "openai", EndpointID: "ep-1", Model: "gpt-4o", LatencyMS: 14},
		"openai/gpt-4o-mini": {ProviderID: "openai", EndpointID: "ep-2", Model: "gpt-4o-mini", LatencyMS: 9},
	}}
}

// newModelTestService wires the service over a registry holding one chat
// provider (two models), one media provider, and one provider whose only model
// is an embedding.
func newModelTestService(t *testing.T, prober ModelProber) *ProviderModelTestService {
	t.Helper()
	index := testIndex(t,
		testProvider("openai", "api",
			testModel("gpt-4o", "GPT-4o", "llm"),
			testModel("gpt-4o-mini", "GPT-4o mini", "llm")),
		testProvider("black-forest-labs", "media", testModel("flux-kontext-pro", "FLUX Kontext Pro", "image")),
		testProvider("voyage", "api", testModel("voyage-3", "Voyage 3", "embedding")),
	)
	providers, err := NewProviderService(ProviderServiceDeps{Index: index})
	if err != nil {
		t.Fatalf("NewProviderService() error = %v", err)
	}
	svc, err := NewProviderModelTestService(providers, prober)
	if err != nil {
		t.Fatalf("NewProviderModelTestService() error = %v", err)
	}
	return svc
}

// TestProviderModelTestService_NewRejectsMissingDeps pins the constructor: a
// service without a prober would answer a test it never ran.
func TestProviderModelTestService_NewRejectsMissingDeps(t *testing.T) {
	providers, err := NewProviderService(ProviderServiceDeps{Index: testIndex(t, testProvider("openai", "api"))})
	if err != nil {
		t.Fatalf("NewProviderService() error = %v", err)
	}
	if _, err := NewProviderModelTestService(nil, newProbeStub()); err == nil {
		t.Fatal("NewProviderModelTestService(nil prober) error = nil, want a refusal")
	}
	if _, err := NewProviderModelTestService(providers, nil); err == nil {
		t.Fatal("NewProviderModelTestService(nil providers) error = nil, want a refusal")
	}
}

// TestProviderModelTestService_TestModel covers one model per call: the healthy
// answer, a typed upstream failure, and the refusals.
func TestProviderModelTestService_TestModel(t *testing.T) {
	unauthorized := &dataplane.Error{Code: dataplane.CodeUnauthorized, Message: "the key is revoked", Status: 401}
	rateLimited := &dataplane.Error{Code: dataplane.CodeRateLimited, Message: "quota spent", Status: 429}

	cases := []struct {
		name         string
		providerID   string
		modelID      string
		fail         map[string]error
		wantOK       bool
		wantCode     string
		wantStatus   int
		wantErrCode  string
		wantLatency  int64
		wantName     string
		wantEndpoint string
	}{
		{name: "healthy model", providerID: "openai", modelID: "gpt-4o", wantOK: true,
			wantLatency: 14, wantName: "GPT-4o", wantEndpoint: "ep-1"},
		{name: "second model keeps its own identity", providerID: "openai", modelID: "gpt-4o-mini",
			wantOK: true, wantLatency: 9, wantName: "GPT-4o mini", wantEndpoint: "ep-2"},
		{name: "unauthorized upstream", providerID: "openai", modelID: "gpt-4o",
			fail:     map[string]error{"openai/gpt-4o": unauthorized},
			wantCode: dataplane.CodeUnauthorized, wantStatus: 401, wantName: "GPT-4o"},
		{name: "rate limited upstream", providerID: "openai", modelID: "gpt-4o",
			fail:     map[string]error{"openai/gpt-4o": rateLimited},
			wantCode: dataplane.CodeRateLimited, wantStatus: 429, wantName: "GPT-4o"},
		{name: "unknown provider", providerID: "ghost", modelID: "gpt-4o", wantErrCode: "NOT_FOUND"},
		{name: "blank model id", providerID: "openai", modelID: "  ", wantErrCode: "VALIDATION_ERROR"},
		{name: "blank provider id", providerID: "", modelID: "gpt-4o", wantErrCode: "VALIDATION_ERROR"},
		// A media model is not reachable by the chat probe, so the service says so
		// instead of reporting the pipeline's confusing failure as the model's.
		{name: "image model", providerID: "black-forest-labs", modelID: "flux-kontext-pro",
			wantErrCode: "VALIDATION_ERROR"},
		{name: "embedding model", providerID: "voyage", modelID: "voyage-3", wantErrCode: "VALIDATION_ERROR"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prober := newProbeStub()
			prober.fail = tc.fail
			svc := newModelTestService(t, prober)

			result, err := svc.TestModel(context.Background(), tc.providerID, tc.modelID)
			if tc.wantErrCode != "" {
				if domain.AsAppError(err).Code != tc.wantErrCode {
					t.Fatalf("error code = %v, want %s (err %v)", domain.AsAppError(err), tc.wantErrCode, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("TestModel() error = %v, want a result", err)
			}
			if result.OK != tc.wantOK {
				t.Fatalf("ok = %v, want %v", result.OK, tc.wantOK)
			}
			if result.ModelID != tc.modelID {
				t.Fatalf("model_id = %q, want %q", result.ModelID, tc.modelID)
			}
			if result.Name != tc.wantName {
				t.Fatalf("name = %q, want %q", result.Name, tc.wantName)
			}
			if result.LatencyMS != tc.wantLatency {
				t.Fatalf("latency_ms = %d, want %d", result.LatencyMS, tc.wantLatency)
			}
			if result.EndpointID != tc.wantEndpoint {
				t.Fatalf("endpoint_id = %q, want %q", result.EndpointID, tc.wantEndpoint)
			}
			if result.ErrorCode != tc.wantCode {
				t.Fatalf("error_code = %q, want %q", result.ErrorCode, tc.wantCode)
			}
			if result.Status != tc.wantStatus {
				t.Fatalf("status = %d, want %d", result.Status, tc.wantStatus)
			}
			if tc.wantCode != "" && result.Error == "" {
				t.Fatal("a failed probe must carry an English message, got none")
			}
		})
	}
}

// TestProviderModelTestService_TestModelPassesTheReferenceThrough pins the string
// the data plane is asked to route: the catalog's own `provider/model`. A model
// whose upstream id differs must be probed by the id the panel shows, because
// that is the string the operator will put in a client.
func TestProviderModelTestService_TestModelPassesTheReferenceThrough(t *testing.T) {
	prober := newProbeStub()
	svc := newModelTestService(t, prober)
	if _, err := svc.TestModel(context.Background(), "openai", "gpt-4o-mini"); err != nil {
		t.Fatalf("TestModel() error = %v", err)
	}
	if len(prober.refs) != 1 || prober.refs[0] != "openai/gpt-4o-mini" {
		t.Fatalf("refs = %v, want exactly [openai/gpt-4o-mini]", prober.refs)
	}
}
