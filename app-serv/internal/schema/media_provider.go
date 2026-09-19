// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/media_provider.go
// @for       The media provider contracts: the kind filter, the detail answer,
//
//	and the per-kind override save (SPEC-API-001 §7.10).
//
// @uses      nothing beyond the standard library; the mappers live in handler.
// @reason    §7.10's save is a partial update of two values, and each reports
//
//	its own source (`registry` or `override`) because a provider may
//	override its default model while keeping the registry's base URL —
//	one shared "overridden" flag would mislabel that case and the panel
//	would offer a reset that does nothing.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-19
package schema

// MediaOverrideRequest is the body of PATCH /api/v1/media-providers/{provider_id}.
//
// The kind is in the body rather than the path because the route addresses a
// provider and the panel's page is per kind; an empty base_url means "use the
// registry's", which is how an operator undoes an override.
type MediaOverrideRequest struct {
	Kind         string `json:"kind"          validate:"required,oneof=tts stt embedding image video search"`
	BaseURL      string `json:"base_url"      validate:"omitempty,max=2048"`
	DefaultModel string `json:"default_model" validate:"omitempty,max=200"`
}

// MediaModelResponse is one model a media service declares.
type MediaModelResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name,omitempty"`
	Dimensions int    `json:"dimensions,omitempty"`
}

// MediaKindBlock is one provider's offering of one media kind, resolved.
type MediaKindBlock struct {
	Kind               string               `json:"kind"`
	BaseURL            string               `json:"base_url"`
	BaseURLSource      string               `json:"base_url_source"`
	DefaultModel       string               `json:"default_model"`
	DefaultModelSource string               `json:"default_model_source"`
	EndpointCount      int64                `json:"endpoint_count"`
	Models             []MediaModelResponse `json:"models"`
}

// MediaProviderResponse is one list row: the provider's identity plus its kind
// block.
type MediaProviderResponse struct {
	ProviderID   string `json:"provider_id"`
	ProviderName string `json:"provider_name"`
	MediaKindBlock
}

// MediaProviderList wraps the list. §7.10 lists media providers without
// pagination: a deployment configures a handful of them.
type MediaProviderList struct {
	Data []MediaProviderResponse `json:"data"`
}

// MediaProviderDetail is the detail answer: the provider's identity and every
// media kind it offers.
type MediaProviderDetail struct {
	ProviderID   string           `json:"provider_id"`
	ProviderName string           `json:"provider_name"`
	Media        []MediaKindBlock `json:"media"`
}
