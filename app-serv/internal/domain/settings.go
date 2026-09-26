// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/settings.go
// @for       The typed settings object, its documented defaults, and the
//
//	partial-update rules a PATCH is applied through.
//
// @uses      internal/domain (ComboStrategy, AppError constructors).
// @reason    SPEC-API-001 §7.14 fixes the v1 settings surface and its defaults,
//
//	and §7.9 deprecates the caveman saver key while requiring it to
//	stay accepted and frozen so an exported reference configuration
//	round-trips. Keeping that rule here, beside the defaults, is what
//	stops any layer from rendering or mutating a key the spec removed.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-18
package domain

// SettingsKey is a stored settings row's key. One row per key makes a partial
// PATCH a single-row upsert (SPEC-API-001 §6).
type SettingsKey string

const (
	// SettingsKeySecurity holds security.require_login and require_api_key.
	SettingsKeySecurity SettingsKey = "security"
	// SettingsKeyRouting holds the routing defaults.
	SettingsKeyRouting SettingsKey = "routing"
	// SettingsKeyNetwork holds the outbound proxy configuration.
	SettingsKeyNetwork SettingsKey = "network"
	// SettingsKeyTokenSaver holds the §7.9 saver configuration.
	SettingsKeyTokenSaver SettingsKey = "token_saver"
	// SettingsKeyLogging holds capture, retention, and buffer bounds.
	SettingsKeyLogging SettingsKey = "logging"
	// SettingsKeyReasoning holds the per-provider thinking modes.
	SettingsKeyReasoning SettingsKey = "reasoning"
	// SettingsKeyCaveman is the DEPRECATED saver key. It is a stored key rather
	// than a field of TokenSaver: the point of freezing it is that an exported
	// reference configuration keeps its value untouched, and a field on a
	// rendered object is a field some future renderer will eventually emit.
	SettingsKeyCaveman SettingsKey = "token_saver.caveman"
)

// SettingsKeys is every key the store round-trips, in a stable order so a test
// can assert the set rather than a map iteration order. The deprecated caveman
// key is included because an exported reference configuration must survive a
// read-and-write cycle unchanged (SPEC-API-001 §7.9).
var SettingsKeys = []SettingsKey{
	SettingsKeySecurity,
	SettingsKeyRouting,
	SettingsKeyNetwork,
	SettingsKeyTokenSaver,
	SettingsKeyLogging,
	SettingsKeyReasoning,
	SettingsKeyCaveman,
}

// DefaultCavemanLevel is the frozen level of the deprecated caveman saver.
// SPEC-API-001 §7.9 shows the default as `{enabled: false, level: "full"}` and
// requires it to stay at that value for the life of v1.
const DefaultCavemanLevel = "full"

// DefaultSaverLevel is the level the ponytail group starts at and the word the
// reference registry uses for that saver's default. The RTK group has no level:
// its strength is the filter allowlist (SPEC-API-002 §4).
const DefaultSaverLevel = "full"

// CavemanSetting is the deprecated saver's stored shape. It is decoded and
// re-encoded so an exported configuration round-trips, and it has no accessor
// on Settings: nothing in v1 may read it to make a decision.
type CavemanSetting struct {
	Enabled bool   `json:"enabled"`
	Level   string `json:"level"`
}

// DefaultCavemanSetting is the frozen default the key stays at.
func DefaultCavemanSetting() CavemanSetting {
	return CavemanSetting{Enabled: false, Level: DefaultCavemanLevel}
}

// SecuritySettings is the §7.14 security group.
type SecuritySettings struct {
	RequireLogin  bool `json:"require_login"`
	RequireAPIKey bool `json:"require_api_key"`
}

// NetworkSettings is the §7.14 network group.
type NetworkSettings struct {
	OutboundProxyEnabled bool   `json:"outbound_proxy_enabled"`
	OutboundProxyURL     string `json:"outbound_proxy_url"`
	OutboundNoProxy      string `json:"outbound_no_proxy"`
	// OutboundProxyStrategy is how the pool's enabled candidates route traffic
	// when proxying is on (docs/PORT/008-PORT-PROXY-ENGINE.md D2): fallback
	// walks them in insertion order, round_robin rotates the head per request.
	// An empty stored value reads as the default, so documents written before
	// the key existed stay valid.
	OutboundProxyStrategy string `json:"outbound_proxy_strategy"`
	// ProviderProxies is the per-provider binding (docs/PORT/
	// 009-PORT-PROVIDER-PROXY.md D1): the pool one provider's calls egress
	// through, and the strategy that orders them. It is written whole, like
	// the routing group's override map.
	ProviderProxies map[string]ProviderProxy `json:"provider_proxies,omitempty"`
}

// LoggingSettings is the §7.14 logging group.
type LoggingSettings struct {
	RequestCaptureEnabled   bool `json:"request_capture_enabled"`
	RetentionDays           int  `json:"retention_days"`
	CaptureBodyMaxBytes     int  `json:"capture_body_max_bytes"`
	ObservabilityMaxRecords int  `json:"observability_max_records"`
}

// Settings is the whole typed object. Every field group has an exported field
// because this is a configuration document, not an entity with invariants: a
// PATCH is validated per key by Update, which is where the rules live.
type Settings struct {
	Security   SecuritySettings   `json:"security"`
	Routing    RoutingSettings    `json:"routing"`
	Network    NetworkSettings    `json:"network"`
	TokenSaver TokenSaverSettings `json:"token_saver"`
	Logging    LoggingSettings    `json:"logging"`
	Reasoning  ReasoningSettings  `json:"reasoning"`
}

// DefaultSettings is the §7.14 default document, merged at read so a stored row
// that predates a key still answers with the documented value.
func DefaultSettings() Settings {
	return Settings{
		Security: SecuritySettings{RequireLogin: true, RequireAPIKey: true},
		Routing:  defaultRoutingSettings(),
		Network: NetworkSettings{
			OutboundProxyEnabled:  false,
			OutboundProxyStrategy: DefaultProxyStrategy,
			ProviderProxies:       map[string]ProviderProxy{},
		},
		TokenSaver: TokenSaverSettings{
			// Every saver ships off (owner decision, 2026-09-19): the pipeline
			// must not rewrite a request until an operator turns a group on.
			RTK:      TokenSaverRTK{Enabled: false, Filters: []string{}},
			Headroom: TokenSaverHeadroom{Enabled: false},
			Ponytail: TokenSaverToggle{Enabled: false, Level: DefaultSaverLevel},
		},
		Logging: LoggingSettings{
			RequestCaptureEnabled:   false,
			RetentionDays:           7,
			CaptureBodyMaxBytes:     65536,
			ObservabilityMaxRecords: 1000,
		},
		Reasoning: defaultReasoningSettings(),
	}
}
