// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/vision_adapter.go
// @for       The vision adapter contracts: the configuration payload and its
//
//	replacement body (SPEC-API-001 §7.8).
//
// @uses      go-playground/validator/v10 through shared validation, internal/domain.
// @reason    AGENTS.md §2.4 requires the contract before the handler, and §7.8
//
//	defines one shape for both the read and the write so the panel can
//	PUT back exactly what it GET — a round-trip a differing write shape
//	would break.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-17
package schema

import (
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// VisionAdapterResponse is GET /api/v1/vision-adapter (§7.8).
type VisionAdapterResponse struct {
	Enabled    bool     `json:"enabled"`
	RoundRobin bool     `json:"round_robin"`
	Models     []string `json:"models"`
	UpdatedAt  string   `json:"updated_at,omitempty"`
}

// ReplaceVisionAdapterRequest is the body of PUT /api/v1/vision-adapter.
//
// Models is required rather than optional: a PUT replaces the configuration, so
// omitting the list would be indistinguishable from clearing it, and "enabled
// with no models" is a state §7.8 renders as a warning rather than as intent.
type ReplaceVisionAdapterRequest struct {
	Enabled    bool     `json:"enabled"`
	RoundRobin bool     `json:"round_robin"`
	Models     []string `json:"models" validate:"required,max=64,dive,min=3,max=200"`
}

// ToVisionAdapterResponse maps the aggregate onto the wire shape.
func ToVisionAdapterResponse(adapter domain.VisionAdapter) VisionAdapterResponse {
	response := VisionAdapterResponse{
		Enabled:    adapter.Enabled(),
		RoundRobin: adapter.RoundRobin(),
		Models:     adapter.ModelStrings(),
	}
	if updated := adapter.UpdatedAt(); !updated.IsZero() {
		response.UpdatedAt = Timestamp(updated)
	}
	return response
}

// ToVisionRefs converts the wire model strings into parsed references. A value
// that is not "provider/model" is a validation error here rather than a silently
// skipped entry, so the client learns which one was wrong.
func ToVisionRefs(models []string) ([]domain.ModelRef, error) {
	refs := make([]domain.ModelRef, 0, len(models))
	for _, model := range models {
		ref, err := domain.ParseModelRef(model)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, nil
}
