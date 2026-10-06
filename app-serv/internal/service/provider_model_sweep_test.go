// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_model_sweep_test.go
// @for       Table-driven tests for the bounded sweep over a provider's chat models (SPEC-API-001 §7.4, draft 017 §4.10).
// @uses      internal/dataplane, internal/domain, internal/registry, testing, time.
// @reason    The sweep is the route with a budget, an order, and a truncation to report, which are three rules the single-model tests cannot see. They are tested apart so a change to the walk cannot pass by breaking the count.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-27
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestProviderModelTestService_TestModels sweeps the catalog in its own order and
// reports every model it was given, failures included.
func TestProviderModelTestService_TestModels(t *testing.T) {
	cases := []struct {
		name        string
		providerID  string
		limit       int
		fail        map[string]error
		delay       time.Duration
		sweepBudget time.Duration
		wantTested  int
		wantTotal   int
		wantStopped string
		wantSource  string
		wantErr     string
		wantCodes   map[string]string
	}{
		{name: "every chat model answers", providerID: "openai", wantTested: 2, wantTotal: 2,
			wantSource: ModelSourceRegistry},
		{name: "one dead model is still a row", providerID: "openai",
			fail:       map[string]error{"openai/gpt-4o-mini": &dataplane.Error{Code: dataplane.CodeUpstreamError, Message: "gone", Status: 503}},
			wantTested: 2, wantTotal: 2, wantSource: ModelSourceRegistry,
			wantCodes: map[string]string{"gpt-4o-mini": dataplane.CodeUpstreamError}},
		{name: "a provider whose models are none of them chat has nothing to test",
			providerID: "black-forest-labs", wantErr: "VALIDATION_ERROR"},
		{name: "limit below the catalog truncates", providerID: "openai", limit: 1,
			wantTested: 1, wantTotal: 2, wantSource: ModelSourceRegistry},
		{name: "zero limit means the default", providerID: "openai",
			wantTested: 2, wantTotal: 2, wantSource: ModelSourceRegistry},
		{name: "a limit above the ceiling is clamped", providerID: "openai", limit: 5_000,
			wantTested: 2, wantTotal: 2, wantSource: ModelSourceRegistry},
		{name: "a sweep that runs out of budget reports what it tested", providerID: "openai",
			delay: 20 * time.Millisecond, sweepBudget: 10 * time.Millisecond,
			wantTested: 1, wantTotal: 2, wantStopped: ProviderModelStoppedDeadline,
			wantSource: ModelSourceRegistry,
			wantCodes:  map[string]string{"gpt-4o": ProviderModelTimeoutCode}},
		{name: "unknown provider", providerID: "ghost", wantErr: "NOT_FOUND"},
		{name: "blank provider", providerID: "  ", wantErr: "VALIDATION_ERROR"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prober := newProbeStub()
			prober.fail = tc.fail
			prober.delay = tc.delay
			svc := newModelTestService(t, prober)

			ctx := context.Background()
			if tc.sweepBudget > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tc.sweepBudget)
				defer cancel()
			}

			response, err := svc.TestModels(ctx, tc.providerID, tc.limit)
			if tc.wantErr != "" {
				if domain.AsAppError(err).Code != tc.wantErr {
					t.Fatalf("error code = %v, want %s (err %v)", domain.AsAppError(err), tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("TestModels() error = %v, want a report", err)
			}
			if response.Tested != tc.wantTested {
				t.Fatalf("tested = %d, want %d", response.Tested, tc.wantTested)
			}
			if len(response.Results) != tc.wantTested {
				t.Fatalf("results = %d rows, want %d", len(response.Results), tc.wantTested)
			}
			if response.Total != tc.wantTotal {
				t.Fatalf("total = %d, want %d", response.Total, tc.wantTotal)
			}
			if response.Stopped != tc.wantStopped {
				t.Fatalf("stopped = %q, want %q", response.Stopped, tc.wantStopped)
			}
			if response.Source != tc.wantSource {
				t.Fatalf("source = %q, want %q", response.Source, tc.wantSource)
			}
			if response.ProviderID != tc.providerID {
				t.Fatalf("provider_id = %q, want %q", response.ProviderID, tc.providerID)
			}
			for _, row := range response.Results {
				want, seeded := tc.wantCodes[row.ModelID]
				if seeded && want != row.ErrorCode {
					t.Fatalf("%s error_code = %q, want %q", row.ModelID, row.ErrorCode, want)
				}
			}
		})
	}
}

// TestProviderModelTestService_SweepKeepsRegistryOrder pins that the rows arrive
// in the order the document lists them, so the panel's table and the answer
// agree without sorting on either side.
func TestProviderModelTestService_SweepKeepsRegistryOrder(t *testing.T) {
	svc := newModelTestService(t, newProbeStub())
	response, err := svc.TestModels(context.Background(), "openai", 0)
	if err != nil {
		t.Fatalf("TestModels() error = %v", err)
	}
	want := []string{"gpt-4o", "gpt-4o-mini"}
	for i, row := range response.Results {
		if row.ModelID != want[i] {
			t.Fatalf("row %d = %q, want %q", i, row.ModelID, want[i])
		}
	}
}

// quietNodeSource answers like a node whose upstream did not reply: the registry
// list with a warning, which is the contract NodeModelSource states.
type quietNodeSource struct{}

func (quietNodeSource) ListNodeModels(context.Context, string) (NodeModelList, error) {
	return NodeModelList{}, errors.New("the node's upstream is unreachable")
}

// TestProviderModelTestService_TestModelsCarriesTheListWarning keeps the origin
// honest: a node whose upstream did not answer is tested from the registry's
// list, and the operator has to be told that list may be stale.
func TestProviderModelTestService_TestModelsCarriesTheListWarning(t *testing.T) {
	node := testProvider("custom-node", "api", testModel("gpt-4o", "GPT-4o", "llm"))
	node.Custom = true
	index := testIndex(t, node)
	providers, err := NewProviderService(ProviderServiceDeps{Index: index, Source: quietNodeSource{}})
	if err != nil {
		t.Fatalf("NewProviderService() error = %v", err)
	}
	svc, err := NewProviderModelTestService(providers, newProbeStub())
	if err != nil {
		t.Fatalf("NewProviderModelTestService() error = %v", err)
	}

	response, err := svc.TestModels(context.Background(), "custom-node", 0)
	if err != nil {
		t.Fatalf("TestModels() error = %v", err)
	}
	if response.Warning == "" {
		t.Fatal("warning = empty, want the upstream-unavailable notice the list reported")
	}
	if response.Source != ModelSourceRegistry {
		t.Fatalf("source = %q, want the registry the answer actually came from", response.Source)
	}
}
