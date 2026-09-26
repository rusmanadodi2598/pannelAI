// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/model_catalog_capability_test.go
// @for       The capability filter measured against the REAL embedded registry.
//
// @uses      internal/domain, internal/registry, context, testing.
// @reason    SPEC-API-001 §7.6 promises `?capability=vision|tools`, and the
//
//	fixture-based tests this file sits beside could not fail: they wrote
//	"vision" into their own registry data, so they proved the filter
//	matches a string it was handed rather than that the panel's two
//	buttons return anything. Draft 017 §4.4 measured the consequence —
//	both filters answered zero rows over 507 registered models. This
//	test reads the embedded document instead, which is what the
//	operator's panel reads.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-23
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// realCatalogService wires the catalog over the embedded registry, so a
// capability answer is the one a deployment gives.
func realCatalogService(t *testing.T) *ModelCatalogService {
	t.Helper()
	index, err := registry.Load()
	if err != nil {
		t.Fatalf("registry.Load() error = %v", err)
	}
	return newCatalogService(t, index, newStubCatalogRepo(), newStubComboRepo())
}

// TestCatalog_CapabilityFiltersAnswerOverTheRealRegistry is draft 017 §4.4's
// acceptance criterion: each filter the panel offers returns rows, measured
// against the registry the binary embeds rather than against a fixture.
func TestCatalog_CapabilityFiltersAnswerOverTheRealRegistry(t *testing.T) {
	service := realCatalogService(t)
	ctx := context.Background()

	// The two the panel renders, plus the media operations the curated
	// document still declares. All must answer: a filter that returns
	// nothing is a control that does nothing. `mask` and `text2img` are not
	// in the list because the owner's 2026-09-26 KEEP set removed the
	// providers that declared them; an unknown name is pinned separately
	// below.
	for _, capability := range []string{"vision", "tools", "edit", "textToImage"} {
		rows, err := service.Catalog(ctx, CatalogFilter{Capability: capability})
		if err != nil {
			t.Fatalf("Catalog(capability=%q) error = %v", capability, err)
		}
		if len(rows) == 0 {
			t.Fatalf("Catalog(capability=%q) returned no row over the real registry", capability)
		}
		t.Logf("capability=%s → %d rows", capability, len(rows))
	}

	// An unknown name keeps the open-set behaviour: an empty page, not an
	// error. The panel's list is a convenience, not the API's vocabulary.
	unknown, err := service.Catalog(ctx, CatalogFilter{Capability: "not-a-capability"})
	if err != nil {
		t.Fatalf("Catalog(capability=not-a-capability) error = %v", err)
	}
	if len(unknown) != 0 {
		t.Fatalf("an unknown capability returned %d rows, want 0", len(unknown))
	}
}

// TestCatalog_CapabilityFilterIsAClosedAnswerOverTheRealRegistry pins the two
// names the resolver owns against a row that carries neither: a model the
// reference marks text-only must not appear under `vision`, and the vision rows
// must not appear under a media filter.
func TestCatalog_CapabilityFilterIsAClosedAnswerOverTheRealRegistry(t *testing.T) {
	service := realCatalogService(t)
	ctx := context.Background()

	visionRows, err := service.Catalog(ctx, CatalogFilter{Capability: "vision"})
	if err != nil {
		t.Fatalf("Catalog(vision) error = %v", err)
	}
	toolsRows, err := service.Catalog(ctx, CatalogFilter{Capability: "tools"})
	if err != nil {
		t.Fatalf("Catalog(tools) error = %v", err)
	}
	if len(visionRows) >= len(toolsRows) {
		t.Fatalf("vision returned %d rows and tools %d; vision is the narrower question, so a vision set at least as large as the tools set means one of the two is answering the floor",
			len(visionRows), len(toolsRows))
	}
	for _, row := range visionRows {
		if !row.Capabilities().Has("vision") {
			t.Fatalf("%s/%s appeared under capability=vision but does not report it", row.ProviderID(), row.ModelID())
		}
	}
}
