// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/settings_patch.go
// @for       The partial settings PATCH body, its per-key tags, and the
//
//	lowering into the domain mutation object.
//
// @uses      internal/domain (SettingsPatch, ParseComboStrategy).
// @reason    SPEC-API-001 §7.14 validates a PATCH per key, and the caveman key
//
//	has no field here by design: §7.9 makes it never rendered, so a DTO
//	field for it would be the control the spec removes.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-18
package schema

import "github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"

// PatchSettingsRequest is a partial PATCH body. A nil section or a nil field
// means "leave unchanged", so one request can carry any subset of the groups.
type PatchSettingsRequest struct {
	Security   *SecuritySettingsPatch   `json:"security,omitempty"`
	Routing    *RoutingSettingsPatch    `json:"routing,omitempty"`
	Network    *NetworkSettingsPatch    `json:"network,omitempty"`
	TokenSaver *TokenSaverSettingsPatch `json:"token_saver,omitempty"`
	Logging    *LoggingSettingsPatch    `json:"logging,omitempty"`
}

// SecuritySettingsPatch updates the security group.
type SecuritySettingsPatch struct {
	RequireLogin  *bool `json:"require_login,omitempty"`
	RequireAPIKey *bool `json:"require_api_key,omitempty"`
}

// RoutingSettingsPatch updates the routing group. A non-positive sticky limit
// is rejected rather than clamped: a limit of zero would rotate on every
// request, which is not what a caller sending 0 can have meant.
type RoutingSettingsPatch struct {
	ComboStrategy    *string `json:"combo_strategy,omitempty" validate:"omitempty,oneof=fallback round_robin fusion"`
	ComboStickyLimit *int    `json:"combo_sticky_limit,omitempty" validate:"omitempty,min=1,max=100"`
	StickyLimit      *int    `json:"sticky_limit,omitempty" validate:"omitempty,min=1,max=100"`
}

// NetworkSettingsPatch updates the network group.
type NetworkSettingsPatch struct {
	OutboundProxyEnabled *bool   `json:"outbound_proxy_enabled,omitempty"`
	OutboundProxyURL     *string `json:"outbound_proxy_url,omitempty" validate:"omitempty,url,max=2048"`
	OutboundNoProxy      *string `json:"outbound_no_proxy,omitempty" validate:"omitempty,max=2048"`
}

// TokenSaverSettingsPatch updates the §7.9 groups. The caveman key is absent by
// design: §7.9 marks it deprecated, never rendered, and frozen at its default,
// so accepting a value for it here would create the control the spec removes.
type TokenSaverSettingsPatch struct {
	RTK      *TokenSaverTogglePatch   `json:"rtk,omitempty"`
	Headroom *TokenSaverHeadroomPatch `json:"headroom,omitempty"`
	Ponytail *TokenSaverTogglePatch   `json:"ponytail,omitempty"`
}

// TokenSaverTogglePatch updates one saver group.
type TokenSaverTogglePatch struct {
	Enabled *bool   `json:"enabled,omitempty"`
	Level   *string `json:"level,omitempty" validate:"omitempty,oneof=lite full ultra"`
}

// TokenSaverHeadroomPatch updates the external compression saver group.
type TokenSaverHeadroomPatch struct {
	Enabled              *bool   `json:"enabled,omitempty"`
	URL                  *string `json:"url,omitempty" validate:"omitempty,url,max=2048"`
	CompressUserMessages *bool   `json:"compress_user_messages,omitempty"`
}

// LoggingSettingsPatch updates the logging group. The bounds are the ones the
// schema can state alone; the cross-key rule lives in ValidatePatch.
type LoggingSettingsPatch struct {
	RequestCaptureEnabled   *bool `json:"request_capture_enabled,omitempty"`
	RetentionDays           *int  `json:"retention_days,omitempty" validate:"omitempty,min=1,max=365"`
	CaptureBodyMaxBytes     *int  `json:"capture_body_max_bytes,omitempty" validate:"omitempty,min=1,max=1048576"`
	ObservabilityMaxRecords *int  `json:"observability_max_records,omitempty" validate:"omitempty,min=1,max=100000"`
}

// ValidatePatch applies the rules the struct tags cannot express. A tag can
// reject one field at a time; only this function can reject a patch that is
// valid per key but incoherent as a whole.
func ValidatePatch(req PatchSettingsRequest) error {
	if req.Logging != nil {
		enabling := req.Logging.RequestCaptureEnabled != nil && *req.Logging.RequestCaptureEnabled
		shrinking := req.Logging.CaptureBodyMaxBytes != nil && *req.Logging.CaptureBodyMaxBytes < 1
		if enabling && shrinking {
			return domain.NewValidationError("capture_body_max_bytes must be at least 1 when request capture is enabled")
		}
	}
	if req.TokenSaver != nil {
		if err := validateHeadroom(req.TokenSaver.Headroom); err != nil {
			return err
		}
	}
	return nil
}

// validateHeadroom rejects a headroom patch that would enable an external call
// with no destination, which would fail on every request at runtime instead of
// here (SPEC-API-001 §7.9: headroom calls the configured URL with a timeout).
func validateHeadroom(in *TokenSaverHeadroomPatch) error {
	if in == nil {
		return nil
	}
	enabling := in.Enabled != nil && *in.Enabled
	hasURL := in.URL != nil && *in.URL != ""
	if enabling && !hasURL {
		return domain.NewValidationError("token_saver.headroom.url is required when headroom is enabled")
	}
	return nil
}

// ToSettingsPatch lowers the validated DTO into the domain mutation object, so
// the service applies intent rather than the wire shape (AGENTS.md §1.5). The
// DTO layer has already rejected an out-of-set strategy, so an unparseable
// value cannot reach here.
func ToSettingsPatch(req PatchSettingsRequest) domain.SettingsPatch {
	patch := domain.SettingsPatch{}
	if req.Security != nil {
		patch.Security = &domain.SecuritySettingsPatch{
			RequireLogin:  req.Security.RequireLogin,
			RequireAPIKey: req.Security.RequireAPIKey,
		}
	}
	if req.Routing != nil {
		patch.Routing = &domain.RoutingSettingsPatch{
			ComboStrategy:    strategyPtr(req.Routing.ComboStrategy),
			ComboStickyLimit: req.Routing.ComboStickyLimit,
			StickyLimit:      req.Routing.StickyLimit,
		}
	}
	if req.Network != nil {
		patch.Network = &domain.NetworkSettingsPatch{
			OutboundProxyEnabled: req.Network.OutboundProxyEnabled,
			OutboundProxyURL:     req.Network.OutboundProxyURL,
			OutboundNoProxy:      req.Network.OutboundNoProxy,
		}
	}
	if req.TokenSaver != nil {
		patch.TokenSaver = &domain.TokenSaverSettingsPatch{
			RTK:      togglePtr(req.TokenSaver.RTK),
			Headroom: headroomPtr(req.TokenSaver.Headroom),
			Ponytail: togglePtr(req.TokenSaver.Ponytail),
		}
	}
	if req.Logging != nil {
		patch.Logging = &domain.LoggingSettingsPatch{
			RequestCaptureEnabled:   req.Logging.RequestCaptureEnabled,
			RetentionDays:           req.Logging.RetentionDays,
			CaptureBodyMaxBytes:     req.Logging.CaptureBodyMaxBytes,
			ObservabilityMaxRecords: req.Logging.ObservabilityMaxRecords,
		}
	}
	return patch
}

// SettingsResponseFrom maps the service-owned object to the wire shape.
func SettingsResponseFrom(s domain.Settings) SettingsResponse {
	return SettingsResponse{
		Security: SecuritySettingsResponse{
			RequireLogin:  s.Security.RequireLogin,
			RequireAPIKey: s.Security.RequireAPIKey,
		},
		Routing: RoutingSettingsResponse{
			ComboStrategy:    string(s.Routing.ComboStrategy),
			ComboStickyLimit: s.Routing.ComboStickyLimit,
			StickyLimit:      s.Routing.StickyLimit,
		},
		Network: NetworkSettingsResponse{
			OutboundProxyEnabled: s.Network.OutboundProxyEnabled,
			OutboundProxyURL:     s.Network.OutboundProxyURL,
			OutboundNoProxy:      s.Network.OutboundNoProxy,
		},
		TokenSaver: TokenSaverSettingsResponse{
			RTK:      TokenSaverToggleResponse{Enabled: s.TokenSaver.RTK.Enabled, Level: s.TokenSaver.RTK.Level},
			Headroom: TokenSaverHeadroomResponse{Enabled: s.TokenSaver.Headroom.Enabled, URL: s.TokenSaver.Headroom.URL, CompressUserMessages: s.TokenSaver.Headroom.CompressUserMessages},
			Ponytail: TokenSaverToggleResponse{Enabled: s.TokenSaver.Ponytail.Enabled, Level: s.TokenSaver.Ponytail.Level},
		},
		Logging: LoggingSettingsResponse{
			RequestCaptureEnabled:   s.Logging.RequestCaptureEnabled,
			RetentionDays:           s.Logging.RetentionDays,
			CaptureBodyMaxBytes:     s.Logging.CaptureBodyMaxBytes,
			ObservabilityMaxRecords: s.Logging.ObservabilityMaxRecords,
		},
	}
}

// strategyPtr parses the wire strategy into its domain value.
func strategyPtr(s *string) *domain.ComboStrategy {
	if s == nil {
		return nil
	}
	parsed, err := domain.ParseComboStrategy(*s)
	if err != nil {
		return nil
	}
	return &parsed
}

// togglePtr lowers one saver toggle group.
func togglePtr(in *TokenSaverTogglePatch) *domain.TokenSaverTogglePatch {
	if in == nil {
		return nil
	}
	return &domain.TokenSaverTogglePatch{Enabled: in.Enabled, Level: in.Level}
}

// headroomPtr lowers the headroom saver group.
func headroomPtr(in *TokenSaverHeadroomPatch) *domain.TokenSaverHeadroomPatch {
	if in == nil {
		return nil
	}
	return &domain.TokenSaverHeadroomPatch{
		Enabled:              in.Enabled,
		URL:                  in.URL,
		CompressUserMessages: in.CompressUserMessages,
	}
}
