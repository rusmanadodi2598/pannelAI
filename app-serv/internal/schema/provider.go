// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/provider.go
// @for       The provider registry read contracts (SPEC-API-001 §7.4).
// @uses      internal/registry, internal/domain.
// @reason    §7.4 serves the embedded registry over HTTP, and §8.1 requires the
//
//	panel to read a provider's routability before configuring an endpoint that
//	can never answer. The wire shape is fixed here rather than derived from the
//	YAML struct so a registry field rename cannot silently change the API.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-18
package schema

import (
	"sort"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// ProviderListQuery is the decoded filter of GET /api/v1/providers. The
// category is validated against the measured set rather than a hardcoded one,
// because the reference treats category as an open string.
type ProviderListQuery struct {
	Category    string `json:"category,omitempty"`
	Routability string `json:"routability,omitempty" validate:"omitempty,oneof=native connector"`
}

// ProviderResponse is one provider row. StatusSummary reports how the stored
// accounts for this provider are doing, which is why it is a nested object and
// not three sibling fields: a client renders it as one block.
type ProviderResponse struct {
	ID            string                   `json:"id"`
	Name          string                   `json:"name"`
	Category      string                   `json:"category"`
	AuthType      string                   `json:"auth_type"`
	AuthModes     []string                 `json:"auth_modes"`
	HasOAuth      bool                     `json:"has_oauth"`
	NoAuth        bool                     `json:"no_auth"`
	Routability   string                   `json:"routability"`
	EndpointCount int64                    `json:"endpoint_count"`
	StatusSummary ProviderStatusSummaryDTO `json:"status_summary"`
}

// ProviderStatusSummaryDTO counts the stored endpoints of one provider by
// state. The authoritative per-endpoint state stays on the endpoint resource;
// this is a roll-up so the list screen does not have to fetch every account.
type ProviderStatusSummaryDTO struct {
	Total     int64 `json:"total"`
	Active    int64 `json:"active"`
	Disabled  int64 `json:"disabled"`
	Error     int64 `json:"error"`
	RateLimit int64 `json:"rate_limited"`
}

// ProviderList is the §7.4 list body. It carries a meta block because the
// registry is a large filtered set (94 entries in the P1 document), unlike the
// provider-node list which is hand-configured.
type ProviderList struct {
	Data []ProviderResponse `json:"data"`
	Meta Page               `json:"meta"`
}

// ProviderModelResponse is one model inside a provider. Kind is carried because
// a non-chat model is not routable through the chat data plane (§7.15), so a
// client that hid it would offer a model string that cannot answer.
type ProviderModelResponse struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Kind         string   `json:"kind"`
	Capabilities []string `json:"capabilities"`
	Dimensions   int      `json:"dimensions,omitempty"`
	Suggested    bool     `json:"suggested"`
}

// ProviderMediaResponse is one non-chat service kind's endpoint and credential
// placement (§7.4, §7.10). AuthHeader is reported because `key` means a query
// parameter, not a header.
type ProviderMediaResponse struct {
	Kind         string `json:"kind"`
	BaseURL      string `json:"base_url"`
	AuthType     string `json:"auth_type"`
	AuthHeader   string `json:"auth_header"`
	Format       string `json:"format"`
	DefaultModel string `json:"default_model"`
	ModelCount   int    `json:"model_count"`
}

// ProviderDetailResponse is the §7.4 detail body. It carries the transport
// defaults a client needs to reason about the provider plus the media blocks,
// because reachability is decided by format and credential placement rather
// than by the provider's display name.
type ProviderDetailResponse struct {
	ProviderResponse
	BaseURL         string                  `json:"base_url"`
	Format          string                  `json:"format"`
	URLSuffix       string                  `json:"url_suffix"`
	ValidateURL     string                  `json:"validate_url"`
	TimeoutMS       int                     `json:"timeout_ms"`
	ModelCount      int                     `json:"model_count"`
	ChatModelCount  int                     `json:"chat_model_count"`
	Media           []ProviderMediaResponse `json:"media"`
	Deprecated      bool                    `json:"deprecated"`
	DeprecationNote string                  `json:"deprecation_notice,omitempty"`
	Website         string                  `json:"website,omitempty"`
}

// ProviderModelList is the §7.4 model list body.
type ProviderModelList struct {
	Data []ProviderModelResponse `json:"data"`
}

// ProviderResponseFrom maps a registry entry and its stored account counts onto
// the list shape.
func ProviderResponseFrom(entry registry.Provider, summary ProviderStatusSummaryDTO) ProviderResponse {
	return ProviderResponse{
		ID:            entry.ID,
		Name:          providerDisplayName(entry),
		Category:      entry.Category,
		AuthType:      entry.AuthType,
		AuthModes:     append([]string(nil), entry.AuthModes...),
		HasOAuth:      entry.HasOAuth,
		NoAuth:        entry.NoAuth,
		Routability:   entry.ChatRoutability(),
		EndpointCount: summary.Total,
		StatusSummary: summary,
	}
}

// ProviderDetailFrom maps a registry entry onto the detail shape.
func ProviderDetailFrom(entry registry.Provider, summary ProviderStatusSummaryDTO) ProviderDetailResponse {
	chatModels := 0
	for _, model := range entry.Models {
		if model.IsChat() {
			chatModels++
		}
	}
	media := make([]ProviderMediaResponse, 0, len(entry.Media))
	for kind, config := range entry.Media {
		media = append(media, ProviderMediaResponse{
			Kind:         string(kind),
			BaseURL:      config.BaseURL,
			AuthType:     config.AuthType,
			AuthHeader:   config.AuthHeader,
			Format:       config.Format,
			DefaultModel: config.DefaultModel,
			ModelCount:   len(config.Models),
		})
	}
	sortMedia(media)
	return ProviderDetailResponse{
		ProviderResponse: ProviderResponseFrom(entry, summary),
		BaseURL:          entry.Transport.BaseURL,
		Format:           entry.Transport.Format,
		URLSuffix:        entry.Transport.URLSuffix,
		ValidateURL:      entry.Transport.ValidateURL,
		TimeoutMS:        entry.Transport.TimeoutMS,
		ModelCount:       len(entry.Models),
		ChatModelCount:   chatModels,
		Media:            media,
		Deprecated:       entry.Display.Deprecated,
		DeprecationNote:  entry.Display.DeprecationNotice,
		Website:          entry.Display.Website,
	}
}

// ProviderModelsFrom maps a registry entry's models onto the wire shape.
func ProviderModelsFrom(entry registry.Provider) []ProviderModelResponse {
	out := make([]ProviderModelResponse, 0, len(entry.Models))
	for _, model := range entry.Models {
		out = append(out, ProviderModelResponse{
			ID:           model.ID,
			Name:         model.Name,
			Kind:         model.Kind,
			Capabilities: append([]string(nil), model.Capabilities...),
			Dimensions:   model.Dimensions,
			// The registry carries no suggestion flag, so every model the
			// document lists is offered as suggested; a hardcoded subset here
			// would be a guess the panel then renders as advice.
			Suggested: true,
		})
	}
	return out
}

// ProviderCountsFrom maps the repository's per-status counts onto the DTO.
func ProviderCountsFrom(counts domain.EndpointStatusCounts) ProviderStatusSummaryDTO {
	return ProviderStatusSummaryDTO{
		Total:     counts.Total,
		Active:    counts.Active,
		Disabled:  counts.Disabled,
		Error:     counts.Error,
		RateLimit: counts.RateLimited,
	}
}

// providerDisplayName prefers the display name and falls back to the id, so a
// minimal entry still renders a label instead of a blank row.
func providerDisplayName(entry registry.Provider) string {
	if entry.Display.Name != "" {
		return entry.Display.Name
	}
	return entry.ID
}

// sortMedia orders the media blocks by kind so the detail response is stable
// across reads; the registry holds them in a map, whose iteration order is
// deliberately random.
func sortMedia(media []ProviderMediaResponse) {
	sort.Slice(media, func(a, b int) bool { return media[a].Kind < media[b].Kind })
}
