// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/settings_patch.go
// @for       The partial settings mutation, its per-key application, and the
//
//	whole-document validation a patch is checked against.
//
// @uses      internal/domain (Settings, ComboStrategy, AppError constructors),
//
//	(net/url).
//
// @reason    SPEC-API-001 §7.14 validates a PATCH per key, and the rules that
//
//	matter are the cross-key ones: a per-field tag can reject one value,
//	but only the assembled document can reject a combination that is
//	incoherent as a whole. Keeping the apply-and-validate here is what
//	stops a layer from writing a document its own rules forbid.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-18
package domain

import (
	"net/url"
	"strings"
)

// SettingsPatch is a partial mutation. A nil section or nil field means "leave
// unchanged", so a caller can send any subset without having read the rest.
type SettingsPatch struct {
	Security   *SecuritySettingsPatch
	Routing    *RoutingSettingsPatch
	Network    *NetworkSettingsPatch
	TokenSaver *TokenSaverSettingsPatch
	Logging    *LoggingSettingsPatch
}

// SecuritySettingsPatch updates the security group.
type SecuritySettingsPatch struct {
	RequireLogin  *bool
	RequireAPIKey *bool
}

// RoutingSettingsPatch updates the routing group.
type RoutingSettingsPatch struct {
	ComboStrategy    *ComboStrategy
	ComboStickyLimit *int
	StickyLimit      *int
}

// NetworkSettingsPatch updates the network group.
type NetworkSettingsPatch struct {
	OutboundProxyEnabled *bool
	OutboundProxyURL     *string
	OutboundNoProxy      *string
}

// TokenSaverSettingsPatch updates the §7.9 groups. It has no caveman field: the
// only way to reach that key is through SettingsKeyCaveman, which no route
// writes.
type TokenSaverSettingsPatch struct {
	RTK      *TokenSaverRTKPatch
	Headroom *TokenSaverHeadroomPatch
	Ponytail *TokenSaverTogglePatch
}

// TokenSaverRTKPatch updates the native engine's group. Filters is a whole
// replacement, so a patch carrying an empty list clears the allowlist back to
// "every filter is eligible".
type TokenSaverRTKPatch struct {
	Enabled *bool
	Filters *[]string
}

// TokenSaverTogglePatch updates one saver group.
type TokenSaverTogglePatch struct {
	Enabled *bool
	Level   *string
}

// TokenSaverHeadroomPatch updates the external compression saver group.
type TokenSaverHeadroomPatch struct {
	Enabled              *bool
	URL                  *string
	CompressUserMessages *bool
}

// LoggingSettingsPatch updates the logging group.
type LoggingSettingsPatch struct {
	RequestCaptureEnabled   *bool
	RetentionDays           *int
	CaptureBodyMaxBytes     *int
	ObservabilityMaxRecords *int
}

// Validate checks the whole document's cross-key rules, which are the ones a
// per-field check cannot see. It runs after every patch is applied.
func (s Settings) Validate() error {
	if s.Logging.RetentionDays < 1 {
		return NewValidationError("logging.retention_days must be at least 1")
	}
	if s.Logging.CaptureBodyMaxBytes < 1 {
		return NewValidationError("logging.capture_body_max_bytes must be at least 1")
	}
	if s.Logging.ObservabilityMaxRecords < 1 {
		return NewValidationError("logging.observability_max_records must be at least 1")
	}
	if s.Routing.ComboStickyLimit < 1 {
		return NewValidationError("routing.combo_sticky_limit must be at least 1")
	}
	if s.Routing.StickyLimit < 1 {
		return NewValidationError("routing.sticky_limit must be at least 1")
	}
	if !s.Routing.ComboStrategy.IsValid() {
		return NewValidationError("routing.combo_strategy is invalid")
	}
	if s.TokenSaver.Headroom.Enabled && s.TokenSaver.Headroom.URL == "" {
		return NewValidationError("token_saver.headroom.url is required when headroom is enabled")
	}
	if err := validateHeadroomURL(s.TokenSaver.Headroom.URL); err != nil {
		return err
	}
	for _, name := range s.TokenSaver.RTK.Filters {
		if !ValidTokenSaverFilter(name) {
			return NewValidationError("token_saver.rtk.filters entries must be one of the twelve engine filters")
		}
	}
	if !ValidTokenSaverLevel(s.TokenSaver.Ponytail.Level) {
		return NewValidationError("token_saver levels must be one of " + strings.Join(TokenSaverLevels, ", "))
	}
	return nil
}

// validateHeadroomURL requires an absolute http(s) URL whenever the compression
// saver carries one. The saver dials the URL later (§7.9), so a scheme that dial
// cannot speak is refused at write time, whichever write path stored it: the
// §7.14 PATCH and the §7.9 PUT both land in Settings.Validate.
func validateHeadroomURL(raw string) error {
	if raw == "" {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return NewValidationError("token_saver.headroom.url must be an absolute http or https URL")
	}
	return nil
}

// Update applies a validated patch in place and re-checks the whole document,
// so a patch that is valid per field but incoherent as a whole is rejected
// before it is stored.
func (s *Settings) Update(patch SettingsPatch) error {
	if p := patch.Security; p != nil {
		applyBool(&s.Security.RequireLogin, p.RequireLogin)
		applyBool(&s.Security.RequireAPIKey, p.RequireAPIKey)
	}
	if p := patch.Routing; p != nil {
		if p.ComboStrategy != nil {
			s.Routing.ComboStrategy = *p.ComboStrategy
		}
		applyInt(&s.Routing.ComboStickyLimit, p.ComboStickyLimit)
		applyInt(&s.Routing.StickyLimit, p.StickyLimit)
	}
	if p := patch.Network; p != nil {
		applyBool(&s.Network.OutboundProxyEnabled, p.OutboundProxyEnabled)
		applyString(&s.Network.OutboundProxyURL, p.OutboundProxyURL)
		applyString(&s.Network.OutboundNoProxy, p.OutboundNoProxy)
	}
	if p := patch.TokenSaver; p != nil {
		applyRTK(&s.TokenSaver.RTK, p.RTK)
		applyToggle(&s.TokenSaver.Ponytail, p.Ponytail)
		if h := p.Headroom; h != nil {
			applyBool(&s.TokenSaver.Headroom.Enabled, h.Enabled)
			applyString(&s.TokenSaver.Headroom.URL, h.URL)
			applyBool(&s.TokenSaver.Headroom.CompressUserMessages, h.CompressUserMessages)
		}
	}
	if p := patch.Logging; p != nil {
		applyBool(&s.Logging.RequestCaptureEnabled, p.RequestCaptureEnabled)
		applyInt(&s.Logging.RetentionDays, p.RetentionDays)
		applyInt(&s.Logging.CaptureBodyMaxBytes, p.CaptureBodyMaxBytes)
		applyInt(&s.Logging.ObservabilityMaxRecords, p.ObservabilityMaxRecords)
	}
	return s.Validate()
}

// applyBool writes a value only when the patch carried one.
func applyBool(dst *bool, src *bool) {
	if src != nil {
		*dst = *src
	}
}

// applyInt writes a value only when the patch carried one.
func applyInt(dst *int, src *int) {
	if src != nil {
		*dst = *src
	}
}

// applyString writes a value only when the patch carried one.
func applyString(dst *string, src *string) {
	if src != nil {
		*dst = *src
	}
}

// applyToggle writes one saver toggle group only when the patch carried it.
func applyToggle(dst *TokenSaverToggle, src *TokenSaverTogglePatch) {
	if src == nil {
		return
	}
	applyBool(&dst.Enabled, src.Enabled)
	applyString(&dst.Level, src.Level)
}

// applyRTK writes the native engine's group. The filter list is replaced
// wholesale, so an empty list in the patch is a real value (clear the allowlist)
// rather than "leave unchanged", which the nil pointer already means.
func applyRTK(dst *TokenSaverRTK, src *TokenSaverRTKPatch) {
	if src == nil {
		return
	}
	applyBool(&dst.Enabled, src.Enabled)
	if src.Filters != nil {
		replaced := make([]string, len(*src.Filters))
		copy(replaced, *src.Filters)
		dst.Filters = replaced
	}
}
