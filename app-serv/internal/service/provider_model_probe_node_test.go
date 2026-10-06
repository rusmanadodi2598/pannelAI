// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_model_probe_node_test.go
// @for       Tests that the model test walks a custom node's own model set, including the rows the operator declared (SPEC-API-001 §7.4, draft 017 §4.2).
// @uses      internal/dataplane, internal/domain, internal/registry, testing.
// @reason    A compatible node is the case the registry provider does not exercise: its model list is the upstream's answer, which is empty when the upstream is unreachable, while the operator's declared rows are on screen and routable. A sweep that walked only the first would tell the operator there was nothing to test beside a table full of models.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-29
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// nodeFixture wires the model test service over one custom node whose entry
// carries `entryModels` (what the upstream answered, empty when it did not) and
// whose declared rows are `declared`.
func nodeFixture(
	t *testing.T, prober ModelProber, entryModels []registry.Model, declared CustomModelLister,
) *ProviderModelTestService {
	t.Helper()
	node := testProvider("openai-compatible-01TEST", "apikey", entryModels...)
	node.Custom = true
	node.Alias = "mycorp"
	providers, err := NewProviderService(ProviderServiceDeps{
		Index: testIndex(t, node), Custom: declared,
	})
	if err != nil {
		t.Fatalf("NewProviderService() error = %v", err)
	}
	svc, err := NewProviderModelTestService(providers, prober)
	if err != nil {
		t.Fatalf("NewProviderModelTestService() error = %v", err)
	}
	return svc
}

// declaredRows builds the operator's rows for one node.
func declaredRows(t *testing.T, providerID string, modelIDs ...string) []domain.CustomModel {
	t.Helper()
	rows := make([]domain.CustomModel, 0, len(modelIDs))
	for _, id := range modelIDs {
		rows = append(rows, detailRow(t, providerID, id))
	}
	return rows
}

// TestProviderModelTestService_NodeWalksTheDeclaredRows is the gap a registry
// provider cannot show: the node's upstream did not answer, so its entry holds
// no models, and the only set worth testing is the one the operator declared.
func TestProviderModelTestService_NodeWalksTheDeclaredRows(t *testing.T) {
	prober := newProbeStub()
	svc := nodeFixture(t, prober, nil, &stubCustomModels{
		rows: declaredRows(t, "openai-compatible-01TEST", "llama-3.3-70b", "qwen-coder"),
	})

	response, err := svc.TestModels(context.Background(), "openai-compatible-01TEST", 0)
	if err != nil {
		t.Fatalf("TestModels() error = %v, want the declared rows tested", err)
	}
	if response.Tested != 2 || response.Total != 2 {
		t.Fatalf("budget = %d/%d, want 2/2", response.Tested, response.Total)
	}
	wantRefs := []string{
		"openai-compatible-01TEST/llama-3.3-70b",
		"openai-compatible-01TEST/qwen-coder",
	}
	for i, ref := range prober.refs {
		if ref != wantRefs[i] {
			t.Fatalf("ref %d = %q, want %q", i, ref, wantRefs[i])
		}
	}
	for _, row := range response.Results {
		if row.Name == "" {
			t.Fatalf("row %+v lost the display name the operator gave it", row)
		}
	}
}

// TestProviderModelTestService_NodeUnionsWithoutDuplicates pins the order and
// the dedupe: what the upstream answered comes first, the operator's extra rows
// follow, and a model both halves name is probed once.
func TestProviderModelTestService_NodeUnionsWithoutDuplicates(t *testing.T) {
	prober := newProbeStub()
	entry := []registry.Model{
		{ID: "gpt-4o", Name: "GPT-4o", Kind: "llm"},
		{ID: "gpt-4o-mini", Name: "GPT-4o mini", Kind: "llm"},
	}
	svc := nodeFixture(t, prober, entry, &stubCustomModels{
		rows: declaredRows(t, "openai-compatible-01TEST", "gpt-4o-mini", "deepseek-v3"),
	})

	response, err := svc.TestModels(context.Background(), "openai-compatible-01TEST", 0)
	if err != nil {
		t.Fatalf("TestModels() error = %v", err)
	}
	if response.Tested != 3 || response.Total != 3 {
		t.Fatalf("budget = %d/%d, want 3/3", response.Tested, response.Total)
	}
	want := []string{"gpt-4o", "gpt-4o-mini", "deepseek-v3"}
	for i, row := range response.Results {
		if row.ModelID != want[i] {
			t.Fatalf("row %d = %q, want %q", i, row.ModelID, want[i])
		}
	}
	if prober.refs[len(prober.refs)-1] != "openai-compatible-01TEST/deepseek-v3" {
		t.Fatalf("refs = %v, want the declared extra probed last", prober.refs)
	}
}

// TestProviderModelTestService_RegistryProviderAlsoWalksItsDeclaredRows keeps
// the rule one rule: a registry provider's supplement is routable too, so a
// sweep that skipped it would report fewer models than the screen lists.
func TestProviderModelTestService_RegistryProviderAlsoWalksItsDeclaredRows(t *testing.T) {
	index := testIndex(t,
		testProvider("openai", "api", testModel("gpt-4o", "GPT-4o", "llm")),
	)
	providers, err := NewProviderService(ProviderServiceDeps{
		Index:  index,
		Custom: &stubCustomModels{rows: declaredRows(t, "openai", "local-llama")},
	})
	if err != nil {
		t.Fatalf("NewProviderService() error = %v", err)
	}
	svc, err := NewProviderModelTestService(providers, newProbeStub())
	if err != nil {
		t.Fatalf("NewProviderModelTestService() error = %v", err)
	}

	response, err := svc.TestModels(context.Background(), "openai", 0)
	if err != nil {
		t.Fatalf("TestModels() error = %v", err)
	}
	if response.Tested != 2 || response.Total != 2 {
		t.Fatalf("budget = %d/%d, want 2/2", response.Tested, response.Total)
	}
}

// TestProviderModelTestService_DeclaredReadFailureIsAnError refuses rather than
// sweeping half the set: an empty answer is a fact, a failed read is not.
func TestProviderModelTestService_DeclaredReadFailureIsAnError(t *testing.T) {
	svc := nodeFixture(t, newProbeStub(), nil,
		&stubCustomModels{err: errors.New("models_custom is unreadable")})

	if _, err := svc.TestModels(context.Background(), "openai-compatible-01TEST", 0); err == nil {
		t.Fatal("TestModels() error = nil, want the read failure")
	}
}

// TestProviderModelTestService_NodeTestsItsDeclaredModelByName covers the single
// row on a node: the name comes from the declared row, and a model string the
// node never declared is still probed, because a compatible node is a
// passthrough.
func TestProviderModelTestService_NodeTestsItsDeclaredModelByName(t *testing.T) {
	cases := []struct {
		name     string
		modelID  string
		wantName string
	}{
		{name: "a declared row", modelID: "llama-3.3-70b", wantName: "llama-3.3-70b"},
		{name: "a passthrough string", modelID: "anything-else", wantName: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := nodeFixture(t, newProbeStub(), nil, &stubCustomModels{
				rows: declaredRows(t, "openai-compatible-01TEST", "llama-3.3-70b"),
			})

			result, err := svc.TestModel(context.Background(), "openai-compatible-01TEST", tc.modelID)
			if err != nil {
				t.Fatalf("TestModel() error = %v", err)
			}
			if result.Name != tc.wantName {
				t.Fatalf("name = %q, want %q", result.Name, tc.wantName)
			}
			if result.ModelID != tc.modelID {
				t.Fatalf("model_id = %q, want %q", result.ModelID, tc.modelID)
			}
		})
	}
}

// TestProviderModelTestService_NodeSweepReportsTheUpstreamWarning keeps the
// origin honest on a node: the upstream did not answer, so the sweep ran over
// the declared and saved list, and the answer says so.
func TestProviderModelTestService_NodeSweepReportsTheUpstreamWarning(t *testing.T) {
	svc := nodeFixture(t, newProbeStub(), nil, &stubCustomModels{
		rows: declaredRows(t, "openai-compatible-01TEST", "llama-3.3-70b"),
	})

	response, err := svc.TestModels(context.Background(), "openai-compatible-01TEST", 0)
	if err != nil {
		t.Fatalf("TestModels() error = %v", err)
	}
	if response.Source != ModelSourceRegistry {
		t.Fatalf("source = %q, want the list the sweep actually walked", response.Source)
	}
	if len(response.Results) == 0 {
		t.Fatal("results = empty, want the declared row probed")
	}
}
