// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/provider.go
// @for       The provider registry read contracts (SPEC-API-001 §7.4).
// @uses      internal/registry.
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
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// ProviderListQuery is the decoded filter of GET /api/v1/providers. The
// category is validated against the measured set rather than a hardcoded one,
// because the reference treats category as an open string.
//
// Q is the free-text search over id and display name (PORT 002 D1). It is
// bounded here rather than in the service, because a limit on external input
// belongs with the DTO that describes it (AGENTS.md §1.4, §2.4).
type ProviderListQuery struct {
	Category    string `json:"category,omitempty"`
	Routability string `json:"routability,omitempty" validate:"omitempty,oneof=native connector"`
	Q           string `json:"q,omitempty" validate:"omitempty,max=120"`
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
//
// Source states where the list came from, which is not decoration: a custom
// node's models are read from its own upstream, so a client that could not tell
// an upstream answer from a fallback would present a stale list as current. It
// replaces the constant `suggested` flag, which carried no information because
// every row answered true.
type ProviderModelList struct {
	Data []ProviderModelResponse `json:"data"`
	// Source is "upstream" or "registry" (service.ModelSource*).
	Source string `json:"source"`
	// Warning explains why Source is not upstream, in English. It is absent when
	// the list came from the upstream.
	Warning string `json:"warning,omitempty"`
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
