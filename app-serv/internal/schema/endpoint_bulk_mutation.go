// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/endpoint_bulk_mutation.go
// @for       The wire shapes of the bulk mutation routes: the selector that names the rows, and the three bodies that act on them (SPEC-API-001 §7.5, §8.1).
// @uses      strings, go-playground/validator/v10, internal/domain.
// @reason    A bulk action is the one place where a malformed request destroys rows the caller never meant to touch, so the selector carries the rule that decides it: an empty selection is a refusal, never a wildcard, and the count the caller saw is echoed back so a list that changed between the render and the click stops there instead of landing on the new set.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-10-16
package schema

import (
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// MaxBulkSelectorIDs bounds the ids one request may name. It is narrower than the
// resolved-set ceiling on purpose: an explicit list is typed or click-built, while a
// wide set arrives through `all` and is measured after the server resolves it.
const MaxBulkSelectorIDs = 200

// BulkSelector names the rows one bulk action addresses. Exactly one of IDs, Tag or
// All is a target; ProviderID and AuthType narrow whichever one it is.
type BulkSelector struct {
	IDs        []string `json:"ids,omitempty" validate:"omitempty,max=200,dive,required,max=64"`
	Tag        string   `json:"tag,omitempty" validate:"omitempty,max=48"`
	AuthType   string   `json:"auth_type,omitempty" validate:"omitempty,oneof=api_key oauth no_auth service_account apikey none"`
	ProviderID string   `json:"provider_id,omitempty" validate:"omitempty,max=120"`
	All        bool     `json:"all,omitempty"`
}

// BulkSelection is a selector after the boundary has read it: the tag normalized to the
// domain's canonical spelling and the auth type mapped from either vocabulary, so the
// service never sees a string it has to guess the meaning of.
type BulkSelection struct {
	IDs        []string
	Tag        domain.Tag
	AuthType   domain.UpstreamAuthType
	ProviderID string
	All        bool
}

// HasTargets reports whether the selection names a set at all.
func (s BulkSelection) HasTargets() bool {
	return s.All || s.Tag != "" || len(s.IDs) > 0
}

// ParseBulkSelector lowers a wire selector into a resolved selection and enforces the
// rule that makes a bulk action safe to build a client against: zero selectors is a
// refusal rather than "everything", and two is a refusal rather than an intersection
// nobody agreed to.
func ParseBulkSelector(in BulkSelector) (BulkSelection, error) {
	ids := make([]string, 0, len(in.IDs))
	for _, id := range in.IDs {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			return BulkSelection{}, domain.NewValidationError("ids must not be blank")
		}
		ids = append(ids, trimmed)
	}
	tag := strings.TrimSpace(in.Tag)
	providerID := strings.TrimSpace(in.ProviderID)

	named := 0
	for _, set := range []bool{len(ids) > 0, tag != "", in.All} {
		if set {
			named++
		}
	}
	if named == 0 {
		return BulkSelection{}, domain.NewBulkSelectorRequiredError(
			"select rows by ids, name one tag, or send all=true")
	}
	if named > 1 {
		return BulkSelection{}, domain.NewBulkSelectorAmbiguousError(
			"ids, tag and all name the same set: send exactly one of them")
	}

	selection := BulkSelection{IDs: ids, ProviderID: providerID, All: in.All}
	if tag != "" {
		parsed, err := domain.NewTag(tag)
		if err != nil {
			return BulkSelection{}, err
		}
		selection.Tag = parsed
	}
	if auth := strings.TrimSpace(in.AuthType); auth != "" {
		parsed, err := ParseAuthType(auth)
		if err != nil {
			return BulkSelection{}, err
		}
		selection.AuthType = parsed
	}
	return selection, nil
}

// BulkStatusRequest is the body of the two bulk-status routes. `confirm_count` is the
// number the caller displayed: the server resolves the selector, recounts, and refuses
// when the two differ.
type BulkStatusRequest struct {
	Selector     BulkSelector `json:"selector" validate:"required"`
	Status       string       `json:"status" validate:"required,oneof=active disabled"`
	WindowHours  int          `json:"window_hours,omitempty" validate:"omitempty,min=1,max=168"`
	ConfirmCount int          `json:"confirm_count" validate:"required,min=1"`
}

// BulkDeleteRequest is the body of the two bulk-delete routes.
type BulkDeleteRequest struct {
	Selector     BulkSelector `json:"selector" validate:"required"`
	ConfirmCount int          `json:"confirm_count" validate:"required,min=1"`
}

// BulkTagsRequest is the body of the two bulk-tags routes. Add and remove may both be
// filled in one call, because retagging a set is one intent and making an operator send
// two requests splits it across two resolutions of the same selector.
type BulkTagsRequest struct {
	Selector     BulkSelector `json:"selector" validate:"required"`
	Add          []string     `json:"add,omitempty" validate:"omitempty,max=16,dive,required,max=48"`
	Remove       []string     `json:"remove,omitempty" validate:"omitempty,max=16,dive,required,max=48"`
	Clear        bool         `json:"clear,omitempty"`
	ConfirmCount int          `json:"confirm_count" validate:"required,min=1"`
}

// BulkProxyStatusRequest is the body of POST /proxies/bulk-status. `enabled` is a
// pointer because switching a set off is a different instruction from not naming the
// field at all, and a plain bool cannot tell the two apart.
type BulkProxyStatusRequest struct {
	Selector     BulkSelector `json:"selector" validate:"required"`
	Enabled      *bool        `json:"enabled" validate:"required"`
	ConfirmCount int          `json:"confirm_count" validate:"required,min=1"`
}

// BulkProxyDeleteRequest is the body of POST /proxies/bulk-delete. `confirm_endpoints`
// is the number of connections the operator saw lose their egress path: a pool row is
// pinned by an id string rather than a foreign key, so nothing else can tell them the
// erasure is not free.
type BulkProxyDeleteRequest struct {
	Selector         BulkSelector `json:"selector" validate:"required"`
	ConfirmCount     int          `json:"confirm_count" validate:"required,min=1"`
	ConfirmEndpoints int          `json:"confirm_endpoints" validate:"omitempty,min=0"`
}

// ProxyPinsResponse is what a bulk erase takes down with it.
type ProxyPinsResponse struct {
	Endpoints int      `json:"endpoints"`
	Providers []string `json:"providers"`
}

// BulkProxyResponse is the answer to a proxy bulk route, with the pin report beside the
// row results.
type BulkProxyResponse struct {
	Matched int               `json:"matched"`
	Changed int               `json:"changed"`
	Results []BulkResultRow   `json:"results"`
	Pins    ProxyPinsResponse `json:"pins"`
}
