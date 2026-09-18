// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/model.go
// @for       The models catalog contracts: catalog entries, custom models, the
//
//	alias set, and the disabled set (SPEC-API-001 §7.6).
//
// @uses      go-playground/validator/v10 through shared validation, internal/domain.
// @reason    AGENTS.md §2.4 requires a typed, validated struct before handler
//
//	logic, and §7.6 defines four payloads for one screen. Declaring
//	them together keeps the merge the panel renders and the writes it
//	sends describing the same object.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-17
package schema

import (
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// ModelResponse is one catalog row (GET /api/v1/models/catalog).
type ModelResponse struct {
	ID           string   `json:"id"`
	ProviderID   string   `json:"provider_id"`
	ModelID      string   `json:"model_id"`
	DisplayName  string   `json:"display_name"`
	Kind         string   `json:"kind,omitempty"`
	Capabilities []string `json:"capabilities"`
	Source       string   `json:"source"`
}

// ModelCatalogResponse is the catalog list. It is not paginated: the registry
// is a fixed embedded set and the custom rows are curated by hand, so the
// payload has no meta block to report a bound that does not exist.
type ModelCatalogResponse struct {
	Data []ModelResponse `json:"data"`
}

// CreateCustomModelRequest is the body of POST /api/v1/models/custom (§7.6).
type CreateCustomModelRequest struct {
	ProviderID   string   `json:"provider_id"  validate:"required,min=1,max=200"`
	ModelID      string   `json:"model_id"     validate:"required,min=1,max=200"`
	DisplayName  string   `json:"display_name" validate:"required,min=1,max=120"`
	Capabilities []string `json:"capabilities" validate:"omitempty,max=32,dive,min=1,max=64"`
}

// CustomModelResponse is one custom model row.
type CustomModelResponse struct {
	ID           string   `json:"id"`
	ProviderID   string   `json:"provider_id"`
	ModelID      string   `json:"model_id"`
	DisplayName  string   `json:"display_name"`
	Capabilities []string `json:"capabilities"`
	CreatedAt    string   `json:"created_at"`
}

// CustomModelList wraps the custom model rows.
type CustomModelList struct {
	Data []CustomModelResponse `json:"data"`
}

// AliasResponse is one alias → target mapping.
type AliasResponse struct {
	Alias  string `json:"alias"`
	Target string `json:"target"`
}

// AliasList is the GET /api/v1/models/aliases payload. Data is never null: an
// empty set is an empty array, so the panel's table renders its empty state
// rather than handling a missing field.
type AliasList struct {
	Data []AliasResponse `json:"data"`
}

// ReplaceAliasesRequest is the body of PUT /api/v1/models/aliases: the whole
// set, not a delta (§7.6).
//
// The element is a nested struct with its own tags, so validator reports the
// offending index the way §8.1 requires for a bulk write.
type ReplaceAliasesRequest struct {
	Aliases []AliasEntry `json:"aliases" validate:"omitempty,max=1000,dive"`
}

// AliasEntry is one alias → target pair on the wire.
type AliasEntry struct {
	Alias  string `json:"alias"  validate:"required,min=1,max=120"`
	Target string `json:"target" validate:"required,min=1,max=200"`
}

// DisabledModelResponse is one disabled pair.
type DisabledModelResponse struct {
	ProviderID string `json:"provider_id"`
	ModelID    string `json:"model_id"`
}

// DisabledList is the GET /api/v1/models/disabled payload.
type DisabledList struct {
	Data []DisabledModelResponse `json:"data"`
}

// ReplaceDisabledRequest is the body of PUT /api/v1/models/disabled.
type ReplaceDisabledRequest struct {
	Models []DisabledEntry `json:"models" validate:"omitempty,max=5000,dive"`
}

// DisabledEntry is one provider/model pair on the wire.
type DisabledEntry struct {
	ProviderID string `json:"provider_id" validate:"required,min=1,max=200"`
	ModelID    string `json:"model_id"    validate:"required,min=1,max=200"`
}

// ToModelResponses maps catalog rows onto the wire shape.
func ToModelResponses(models []domain.CatalogModel) []ModelResponse {
	out := make([]ModelResponse, 0, len(models))
	for _, model := range models {
		out = append(out, ModelResponse{
			ID:           model.Ref().String(),
			ProviderID:   model.ProviderID(),
			ModelID:      model.ModelID(),
			DisplayName:  model.DisplayName(),
			Kind:         model.Kind(),
			Capabilities: model.Capabilities().List(),
			Source:       string(model.Source()),
		})
	}
	return out
}

// ToCustomModelResponse maps one custom model row onto the wire shape.
func ToCustomModelResponse(model domain.CustomModel) CustomModelResponse {
	return CustomModelResponse{
		ID:           model.ID(),
		ProviderID:   model.ProviderID(),
		ModelID:      model.ModelID(),
		DisplayName:  model.DisplayName(),
		Capabilities: model.Capabilities().List(),
		CreatedAt:    Timestamp(model.CreatedAt()),
	}
}

// ToAliasResponses maps the alias set onto the wire shape.
func ToAliasResponses(aliases []domain.ModelAlias) []AliasResponse {
	out := make([]AliasResponse, 0, len(aliases))
	for _, alias := range aliases {
		out = append(out, AliasResponse{Alias: alias.Alias(), Target: alias.Target()})
	}
	return out
}

// ToModelAliases converts the wire entries into domain value objects.
func ToModelAliases(entries []AliasEntry) ([]domain.ModelAlias, error) {
	aliases := make([]domain.ModelAlias, 0, len(entries))
	for _, entry := range entries {
		alias, err := domain.NewModelAlias(entry.Alias, entry.Target)
		if err != nil {
			return nil, err
		}
		aliases = append(aliases, alias)
	}
	return aliases, nil
}

// ToDisabledResponses maps the disabled set onto the wire shape.
func ToDisabledResponses(refs []domain.ModelRef) []DisabledModelResponse {
	out := make([]DisabledModelResponse, 0, len(refs))
	for _, ref := range refs {
		out = append(out, DisabledModelResponse{ProviderID: ref.ProviderID(), ModelID: ref.ModelID()})
	}
	return out
}

// ToModelRefs converts the wire entries into domain references.
func ToModelRefs(entries []DisabledEntry) ([]domain.ModelRef, error) {
	refs := make([]domain.ModelRef, 0, len(entries))
	for _, entry := range entries {
		ref, err := domain.NewModelRef(entry.ProviderID, entry.ModelID)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, nil
}
