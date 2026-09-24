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

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

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
	// QueryParam and MaxResultsParam name the fields a search provider reads
	// its query and its result count from. Search APIs disagree on the
	// spelling (`q` and `num` at Serper, `query` and `max_results` at Tavily),
	// and the reference carries the same distinction in its per-provider
	// builders, so it is declared here rather than guessed per request. An
	// empty value means the default spelling (`query`, `max_results`).
	QueryParam      string `yaml:"query_param"`
	MaxResultsParam string `yaml:"max_results_param"`
	// Voices is the speech catalog the provider documents, used by
	// GET /api/v1/audio/voices. A provider that fetches its catalog live in
	// the reference declares none here, and the route names that gap rather
	// than answering with a model list the client would misread as voices.
	Voices []MediaVoice `yaml:"voices"`
	// ValidateURL is the endpoint the reference calls to check a credential
	// before saving it, and CreditsPerResult is a metered search provider's own
	// unit cost. Both are carried because the reference declares and reads
	// them; a dropped field is a check that silently stops happening.
	ValidateURL      string `yaml:"validate_url"`
	CreditsPerResult int    `yaml:"credits_per_result"`
	// ModelMap renames the model a request names onto the id the upstream
	// expects, for a media service whose public ids differ from its own.
	ModelMap map[string]ModelMapEntry `yaml:"model_map"`
}

// MediaVoice is one speech voice a provider documents.
type MediaVoice struct {
	ID     string `yaml:"id"`
	Name   string `yaml:"name"`
	Lang   string `yaml:"lang"`
	Gender string `yaml:"gender"`
}

// ModelMapEntry is one model_map value. The reference declares two shapes for
// it: a bare path, or `{path, task}` when the request payload differs per task
// (open-sse/providers/schema.js:40-44, read by imageProviders/huggingface.js:19-20).
// A decoder that accepted only one shape would silently drop the other, so both
// normalise to this struct and the task keeps its documented default.
type ModelMapEntry struct {
	// Path is the upstream id or route the model resolves to.
	Path string `yaml:"path"`
	// Task is the operation the request performs. It defaults to
	// "text-to-image", which is the reference's own default for the bare-path
	// shape.
	Task string `yaml:"task"`
}

// UnmarshalYAML accepts both declared shapes: a scalar is the path with the
// default task, and a mapping carries both fields.
func (m *ModelMapEntry) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		m.Path = node.Value
		m.Task = ModelMapDefaultTask
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("registry: model_map value must be a string or a mapping")
	}
	type plain ModelMapEntry
	var decoded plain
	if err := node.Decode(&decoded); err != nil {
		return err
	}
	*m = ModelMapEntry(decoded)
	if m.Task == "" {
		m.Task = ModelMapDefaultTask
	}
	return nil
}

// ModelMapDefaultTask is the operation a model_map entry performs when it
// declares none, matching the reference's own default.
const ModelMapDefaultTask = "text-to-image"

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
