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

// RoutingSettings is the §7.14 routing group.
type RoutingSettings struct {
	ComboStrategy    ComboStrategy `json:"combo_strategy"`
	ComboStickyLimit int           `json:"combo_sticky_limit"`
	StickyLimit      int           `json:"sticky_limit"`
}

// NetworkSettings is the §7.14 network group.
type NetworkSettings struct {
	OutboundProxyEnabled bool   `json:"outbound_proxy_enabled"`
	OutboundProxyURL     string `json:"outbound_proxy_url"`
	OutboundNoProxy      string `json:"outbound_no_proxy"`
}

// TokenSaverFilters is the canonical filter vocabulary of the native RTK engine
// (SPEC-API-002 §4). It is the single source for the twelve names: the schema's
// oneof tag is pinned against it by a test, and the engine's registry resolves
// exactly these.
var TokenSaverFilters = []string{
	"git-diff",
	"git-status",
	"git-log",
	"grep",
	"find",
	"ls",
	"tree",
	"dedup-log",
	"smart-truncate",
	"read-numbered",
	"search-list",
	"build-output",
}

// ValidTokenSaverFilter reports whether a name is one the engine implements.
// Aliases the reference accepts on a command line (rg, fd) are not configuration
// values: the panel picks from the canonical list, so the stored document
// carries only names the registry resolves by itself.
func ValidTokenSaverFilter(name string) bool {
	for _, known := range TokenSaverFilters {
		if name == known {
			return true
		}
	}
	return false
}

// TokenSaverLevels is the strength vocabulary the ponytail saver accepts, in
// the order the levels escalate. The native engine's prompt table is keyed by
// exactly these words, and a test in that package fails when the two drift
// (SPEC-API-002 §7).
var TokenSaverLevels = []string{"lite", "full", "ultra"}

// ValidTokenSaverLevel reports whether a level is one the engine implements.
func ValidTokenSaverLevel(level string) bool {
	for _, known := range TokenSaverLevels {
		if level == known {
			return true
		}
	}
	return false
}

// TokenSaverRTK is the native engine's group (§7.9): an enable flag and the
// filter allowlist. An empty list means every filter is eligible; the engine
// still autodetects which one applies to a given tool result (SPEC-API-002 §4).
type TokenSaverRTK struct {
	Enabled bool     `json:"enabled"`
	Filters []string `json:"filters"`
}

// TokenSaverToggle is one enable/level saver group (§7.9).
type TokenSaverToggle struct {
	Enabled bool   `json:"enabled"`
	Level   string `json:"level"`
}

// TokenSaverHeadroom is the external compression saver group (§7.9).
type TokenSaverHeadroom struct {
	Enabled              bool   `json:"enabled"`
	URL                  string `json:"url"`
	CompressUserMessages bool   `json:"compress_user_messages"`
}

// TokenSaverSettings is the §7.9 saver configuration, minus caveman.
type TokenSaverSettings struct {
	RTK      TokenSaverRTK      `json:"rtk"`
	Headroom TokenSaverHeadroom `json:"headroom"`
	Ponytail TokenSaverToggle   `json:"ponytail"`
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
}

// DefaultSettings is the §7.14 default document, merged at read so a stored row
// that predates a key still answers with the documented value.
func DefaultSettings() Settings {
	return Settings{
		Security: SecuritySettings{RequireLogin: true, RequireAPIKey: true},
		Routing: RoutingSettings{
			ComboStrategy:    ComboFallback,
			ComboStickyLimit: 1,
			StickyLimit:      3,
		},
		Network: NetworkSettings{OutboundProxyEnabled: false},
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
	}
}
