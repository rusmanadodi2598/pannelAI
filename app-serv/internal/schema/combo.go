// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/combo.go
// @for       The combo contracts: list, detail, create, patch, and the ordered
//
//	model list they all carry (SPEC-API-001 §7.7).
//
// @uses      go-playground/validator/v10 through shared validation, internal/domain.
// @reason    AGENTS.md §2.4 requires the typed contract before the handler, and
//
//	§7.7 makes create and patch the same shape: the panel edits the
//	model list as a set and saves the whole combo. One request struct
//	serves both routes, so the two can never diverge.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-17
package schema

import (
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// ComboModelEntry is one model reference plus its priority.
type ComboModelEntry struct {
	Ref      string `json:"ref"      validate:"required,min=1,max=200"`
	Priority int    `json:"priority" validate:"gte=0,lte=100000"`
}

// ComboRequest is the body of POST /api/v1/combos and PATCH /api/v1/combos/{id}.
//
// StickyLimit and JudgeModel are plain values rather than pointers: §7.7's
// strategy rules decide whether each may be present, and the domain rejects the
// combinations that are not allowed, which is the one place that rule can live.
type ComboRequest struct {
	Name        string            `json:"name"         validate:"required,min=1,max=120"`
	Strategy    string            `json:"strategy"     validate:"required,oneof=fallback round_robin fusion"`
	Models      []ComboModelEntry `json:"models"       validate:"required,min=1,max=64,dive"`
	StickyLimit int               `json:"sticky_limit" validate:"gte=0,lte=100000"`
	JudgeModel  string            `json:"judge_model"  validate:"omitempty,max=200"`
}

// ComboResponse is one combo on the wire.
type ComboResponse struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Strategy    string            `json:"strategy"`
	StickyLimit int               `json:"sticky_limit"`
	JudgeModel  string            `json:"judge_model,omitempty"`
	Models      []ComboModelEntry `json:"models"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

// ComboList wraps a page of combos with the pagination meta block (§4).
type ComboList struct {
	Data []ComboResponse `json:"data"`
	Meta Page            `json:"meta"`
}

// Combo test roles: which part of a combo a probed reference plays.
const (
	ComboTestRoleModel = "model"
	ComboTestRoleJudge = "judge"
)

// ComboTestResult is one reference's probe outcome (§7.7). A failure is a result
// rather than an error, because the route's purpose is to say which references
// fail while still reporting the ones that answered.
type ComboTestResult struct {
	Ref        string `json:"ref"`
	Role       string `json:"role"`
	OK         bool   `json:"ok"`
	ProviderID string `json:"provider_id,omitempty"`
	ModelID    string `json:"model_id,omitempty"`
	EndpointID string `json:"endpoint_id,omitempty"`
	LatencyMS  int64  `json:"latency_ms"`
	ErrorCode  string `json:"error_code,omitempty"`
	Error      string `json:"error,omitempty"`
}

// ComboTestResponse is the combo test route's answer: the combo's identity and
// one result per reference, in the order the combo stores them, with a fusion
// combo's judge last.
type ComboTestResponse struct {
	ComboID  string            `json:"combo_id"`
	Combo    string            `json:"combo"`
	Strategy string            `json:"strategy"`
	Results  []ComboTestResult `json:"results"`
}

// ToComboModels converts the wire entries into the ordered domain list,
// validating each reference's shape. Whether a reference resolves to a model, a
// combo, or an alias is the service's check, because it spans three sources.
func ToComboModels(entries []ComboModelEntry) ([]domain.ComboModel, error) {
	models := make([]domain.ComboModel, 0, len(entries))
	for _, entry := range entries {
		model, err := domain.NewComboModel(entry.Ref, entry.Priority)
		if err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	return models, nil
}

// ToComboResponse maps a combo aggregate onto the wire shape.
func ToComboResponse(combo domain.Combo) ComboResponse {
	entries := make([]ComboModelEntry, 0, combo.ModelCount())
	for _, model := range combo.Models() {
		entries = append(entries, ComboModelEntry{Ref: model.Ref(), Priority: model.Priority()})
	}
	return ComboResponse{
		ID:          combo.ID(),
		Name:        combo.Name(),
		Strategy:    string(combo.Strategy()),
		StickyLimit: combo.StickyLimit(),
		JudgeModel:  combo.JudgeModel(),
		Models:      entries,
		CreatedAt:   Timestamp(combo.CreatedAt()),
		UpdatedAt:   Timestamp(combo.UpdatedAt()),
	}
}

// ToComboResponses maps a page of combos onto the wire shape.
func ToComboResponses(combos []domain.Combo) []ComboResponse {
	out := make([]ComboResponse, 0, len(combos))
	for _, combo := range combos {
		out = append(out, ToComboResponse(combo))
	}
	return out
}
