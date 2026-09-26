// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/settings_response.go
// @for       Mapping the service-owned settings document to the wire shape.
// @uses      internal/domain.
// @reason    SPEC-API-001 §7.14's response is the one place a stored document
//
//	becomes a wire object, and every group that grows needs a line here.
//	It lives apart from settings_patch.go because that file states the
//	write rules, and the two directions of the contract are reviewed
//	against different things: the request against what a caller may say,
//	the response against what the panel must render.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-26
package schema

import "github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"

// SettingsResponseFrom maps the service-owned object to the wire shape.
func SettingsResponseFrom(s domain.Settings) SettingsResponse {
	return SettingsResponse{
		Security: SecuritySettingsResponse{
			RequireLogin:  s.Security.RequireLogin,
			RequireAPIKey: s.Security.RequireAPIKey,
		},
		Routing: RoutingSettingsResponse{
			ComboStrategy:      string(s.Routing.ComboStrategy),
			ComboStickyLimit:   s.Routing.ComboStickyLimit,
			StickyLimit:        s.Routing.StickyLimit,
			FallbackStrategy:   string(s.Routing.FallbackStrategy),
			ProviderStrategies: strategiesOrEmpty(s.Routing.ProviderStrategies),
		},
		Network: NetworkSettingsResponse{
			OutboundProxyEnabled:  s.Network.OutboundProxyEnabled,
			OutboundProxyURL:      s.Network.OutboundProxyURL,
			OutboundNoProxy:       s.Network.OutboundNoProxy,
			OutboundProxyStrategy: proxyStrategyOrDefault(s.Network.OutboundProxyStrategy),
			ProviderProxies:       providerProxiesOrEmpty(s.Network.ProviderProxies),
		},
		TokenSaver: TokenSaverSettingsResponse{
			RTK:      TokenSaverRTKResponse{Enabled: s.TokenSaver.RTK.Enabled, Filters: filtersOrEmpty(s.TokenSaver.RTK.Filters)},
			Headroom: TokenSaverHeadroomResponse{Enabled: s.TokenSaver.Headroom.Enabled, URL: s.TokenSaver.Headroom.URL, CompressUserMessages: s.TokenSaver.Headroom.CompressUserMessages},
			Ponytail: TokenSaverToggleResponse{Enabled: s.TokenSaver.Ponytail.Enabled, Level: s.TokenSaver.Ponytail.Level},
		},
		Logging: LoggingSettingsResponse{
			RequestCaptureEnabled:   s.Logging.RequestCaptureEnabled,
			RetentionDays:           s.Logging.RetentionDays,
			CaptureBodyMaxBytes:     s.Logging.CaptureBodyMaxBytes,
			ObservabilityMaxRecords: s.Logging.ObservabilityMaxRecords,
		},
		Reasoning: ReasoningSettingsResponse{
			ProviderThinking: thinkingOrEmpty(s.Reasoning.ProviderThinking),
		},
	}
}

// proxyStrategyOrDefault names the strategy the engine will run, so a read of
// a document stored before the key existed answers fallback (D2) instead of a
// blank the panel would have to interpret.
func proxyStrategyOrDefault(stored string) string {
	if stored == "" {
		return domain.DefaultProxyStrategy
	}
	return stored
}
