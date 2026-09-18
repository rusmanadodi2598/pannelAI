// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/types_media.go
// @for       The non-chat service configuration a registry entry may declare:
//
//	embeddings, speech, images, search, and their per-kind auth.
//
// @uses      standard library only.
// @reason    A media service often authenticates differently from the same
//
//	provider's chat transport: Gemini reads a query-param `key` for
//	embeddings while its chat transport uses a header. Porting only
//	`transport` would send the wrong credential form, so the media
//	blocks are carried explicitly and the router chooses by kind.
//	§7.10 lists embeddings as P1, which is why this lands now.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package registry

// MediaKind is a non-chat service a provider can offer. The set is closed
// because each kind maps to one route and one request shape (SPEC-API-001 §7.10).
type MediaKind string

const (
	MediaEmbedding MediaKind = "embedding"
	MediaImage     MediaKind = "image"
	MediaImageText MediaKind = "imageToText"
	MediaTTS       MediaKind = "tts"
	MediaSTT       MediaKind = "stt"
	MediaWebSearch MediaKind = "webSearch"
	MediaWebFetch  MediaKind = "webFetch"
	MediaVideo     MediaKind = "video"
	MediaMusic     MediaKind = "music"
	MediaLLM       MediaKind = "llm"
)

// MediaConfig is one service kind's endpoint and credential placement.
//
// AuthHeader matters as much as AuthType here: a query-param key
// (`auth_header: key`) cannot be sent as a header at all, so a caller that
// assumed a header would authenticate incorrectly rather than fail loudly.
type MediaConfig struct {
	BaseURL           string            `yaml:"base_url"`
	AuthType          string            `yaml:"auth_type"`
	AuthHeader        string            `yaml:"auth_header"`
	Format            string            `yaml:"format"`
	DefaultModel      string            `yaml:"default_model"`
	Method            string            `yaml:"method"`
	PollURL           string            `yaml:"poll_url"`
	TimeoutMS         int               `yaml:"timeout_ms"`
	CostPerQuery      float64           `yaml:"cost_per_query"`
	FreeMonthlyQuota  int               `yaml:"free_monthly_quota"`
	CacheTTLMS        int               `yaml:"cache_ttl_ms"`
	DefaultMaxResults int               `yaml:"default_max_results"`
	MaxMaxResults     int               `yaml:"max_max_results"`
	MaxCharacters     int               `yaml:"max_characters"`
	BodyFields        []string          `yaml:"body_fields"`
	Formats           []string          `yaml:"formats"`
	SearchTypes       []string          `yaml:"search_types"`
	Headers           map[string]string `yaml:"headers"`
	Models            []MediaModel      `yaml:"models"`
}

// MediaModel is one model a media service offers. Dimensions applies to an
// embedding model; the other fields are carried so the port does not silently
// drop what the reference declares.
type MediaModel struct {
	ID         string `yaml:"id"`
	Name       string `yaml:"name"`
	Dimensions int    `yaml:"dimensions"`
}

// MediaConfigs is the per-kind configuration of one provider, keyed by kind.
// A nil map means the provider offers no non-chat service.
type MediaConfigs map[MediaKind]MediaConfig

// Supports reports whether the provider offers a service kind.
func (m MediaConfigs) Supports(kind MediaKind) bool {
	_, ok := m[kind]
	return ok
}

// For returns one kind's configuration.
func (m MediaConfigs) For(kind MediaKind) (MediaConfig, bool) {
	config, ok := m[kind]
	return config, ok
}
