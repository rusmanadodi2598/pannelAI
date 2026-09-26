// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/settings.go
// @for       The settings read/patch contract and its per-key validation.
// @uses      internal/domain (Settings, SettingsPatch, ParseComboStrategy).
// @reason    SPEC-API-001 §7.14 fixes the settings surface and §2.4 requires the
//
//	contract as typed structs with validation tags before the handler,
//	so a PATCH is validated per key here rather than by the service
//	guessing what a client meant.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-18
package schema

// SettingsResponse is the body of GET and PATCH /api/v1/settings. It mirrors
// domain.Settings so the service and the wire shape stay one document, and it
// carries no secret: the v1 settings surface has none to return (§7.14).
type SettingsResponse struct {
	Security   SecuritySettingsResponse   `json:"security"`
	Routing    RoutingSettingsResponse    `json:"routing"`
	Network    NetworkSettingsResponse    `json:"network"`
	TokenSaver TokenSaverSettingsResponse `json:"token_saver"`
	Logging    LoggingSettingsResponse    `json:"logging"`
}

// SecuritySettingsResponse is the §7.14 security group.
type SecuritySettingsResponse struct {
	RequireLogin  bool `json:"require_login"`
	RequireAPIKey bool `json:"require_api_key"`
}

// RoutingSettingsResponse is the §7.14 routing group. The credential policy
// keys answer the effective values, never an empty string: a read of a document
// that predates them still names fill-first and an empty override map.
type RoutingSettingsResponse struct {
	ComboStrategy      string                              `json:"combo_strategy"`
	ComboStickyLimit   int                                 `json:"combo_sticky_limit"`
	StickyLimit        int                                 `json:"sticky_limit"`
	FallbackStrategy   string                              `json:"fallback_strategy"`
	ProviderStrategies map[string]ProviderStrategyResponse `json:"provider_strategies"`
}

// ProviderStrategyResponse is one provider's rotation override. An absent field
// inherits the global default, so the two fields carry `omitempty` and the
// panel renders what is stored rather than a filled-in copy.
type ProviderStrategyResponse struct {
	FallbackStrategy string `json:"fallback_strategy,omitempty"`
	StickyLimit      *int   `json:"sticky_limit,omitempty"`
}

// NetworkSettingsResponse is the §7.14 network group. The strategy answers the
// effective value, never an empty string, on the routing group's precedent: a
// read of a document that predates the key still names fallback, so the panel
// renders what the engine will do rather than a blank it must guess about.
type NetworkSettingsResponse struct {
	OutboundProxyEnabled  bool   `json:"outbound_proxy_enabled"`
	OutboundProxyURL      string `json:"outbound_proxy_url"`
	OutboundNoProxy       string `json:"outbound_no_proxy"`
	OutboundProxyStrategy string `json:"outbound_proxy_strategy"`
}

// TokenSaverSettingsResponse carries the §7.9 groups.
//
// It has no caveman field and never will in v1: the key is DEPRECATED (§7.9) and
// stays frozen at its default inside the domain object so a configuration
// exported from the reference still round-trips, but §7.9 forbids rendering a
// control for it, so no response exposes it either.
type TokenSaverSettingsResponse struct {
	RTK      TokenSaverRTKResponse      `json:"rtk"`
	Headroom TokenSaverHeadroomResponse `json:"headroom"`
	Ponytail TokenSaverToggleResponse   `json:"ponytail"`
}

// TokenSaverRTKResponse is the native engine's group: the enable flag and the
// filter allowlist. An empty list is rendered as `[]`, never `null`, so a panel
// round-trip cannot turn "no allowlist" into a missing member (SPEC-API-002 §4).
type TokenSaverRTKResponse struct {
	Enabled bool     `json:"enabled"`
	Filters []string `json:"filters"`
}

// TokenSaverToggleResponse is one enable/level saver group.
type TokenSaverToggleResponse struct {
	Enabled bool   `json:"enabled"`
	Level   string `json:"level,omitempty"`
}

// TokenSaverHeadroomResponse is the external compression saver group.
type TokenSaverHeadroomResponse struct {
	Enabled              bool   `json:"enabled"`
	URL                  string `json:"url"`
	CompressUserMessages bool   `json:"compress_user_messages"`
}

// LoggingSettingsResponse is the §7.14 logging group.
type LoggingSettingsResponse struct {
	RequestCaptureEnabled   bool `json:"request_capture_enabled"`
	RetentionDays           int  `json:"retention_days"`
	CaptureBodyMaxBytes     int  `json:"capture_body_max_bytes"`
	ObservabilityMaxRecords int  `json:"observability_max_records"`
}
