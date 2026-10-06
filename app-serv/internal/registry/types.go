// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/types.go
// @for       The registry document shape: providers, display, models, features.
// @uses      standard library only.
// @reason    SPEC-API-001 §6 puts the provider registry in embedded static config rather than database rows, and §7.4 needs every field the Providers screen renders; this file is the contract that the YAML document and the loader both have to satisfy.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability stable
// @since     2026-09-17
package registry

// Document is the root of the embedded registry file.
type Document struct {
	Revision  string     `yaml:"revision"`
	Providers []Provider `yaml:"providers"`
}

// Provider is one upstream vendor definition.
type Provider struct {
	ID                      string    `yaml:"id"`
	Priority                int       `yaml:"priority"`
	Alias                   string    `yaml:"alias"`
	Aliases                 []string  `yaml:"aliases"`
	UIAlias                 string    `yaml:"ui_alias"`
	Hidden                  bool      `yaml:"hidden"`
	Category                string    `yaml:"category"`
	AuthType                string    `yaml:"auth_type"`
	AuthModes               []string  `yaml:"auth_modes"`
	AuthHint                string    `yaml:"auth_hint"`
	HasOAuth                bool      `yaml:"has_oauth"`
	NoAuth                  bool      `yaml:"no_auth"`
	HasFree                 bool      `yaml:"has_free"`
	PassthroughModels       bool      `yaml:"passthrough_models"`
	HasProviderSpecificData bool      `yaml:"has_provider_specific_data"`
	Display                 Display   `yaml:"display"`
	Transport               Transport `yaml:"transport"`
	// Transports is a multi-endpoint provider's per-format endpoint table: one
	// entry per client wire the provider answers natively, each with its own
	// URL and credential placement. The entry matching the client's format is
	// picked so the request needs no translation, guarded per model through
	// Model.SupportedFormats. Empty means a single-endpoint provider, which is
	// what Transport.BaseURL describes. It sits at the entry level because the
	// reference declares it there.
	Transports     []TransportEndpoint `yaml:"transports"`
	OAuth          *OAuth              `yaml:"oauth"`
	Models         []Model             `yaml:"models"`
	Features       Features            `yaml:"features"`
	ThinkingConfig *ThinkingConfig     `yaml:"thinking_config"`
	ServiceKinds   []string            `yaml:"service_kinds"`
	// SystemOne is the native decision-model endpoint the provider exposes. A
	// model declaring `kind: systemone` is served by the systemone route against
	// this URL, never by the chat data plane: the payload is the provider's own
	// vocabulary, so translating it as chat sends the wrong body.
	SystemOne *SystemOneConfig `yaml:"systemone"`
	// HiddenKinds are the service kinds the panel hides for this provider.
	HiddenKinds []string `yaml:"hidden_kinds"`
	// CredentialFallback names another provider whose stored credential this
	// entry borrows, which is how the reference serves a search-only entry
	// (`ollama-search`) from the account already configured for its chat entry
	// (src/sse/handlers/search.js:155). It is an id rather than a boolean
	// because the fallback is a specific provider, and guessing one would send
	// the wrong account's key.
	CredentialFallback string       `yaml:"credential_fallback"`
	MediaPriority      int          `yaml:"media_priority"`
	Media              MediaConfigs `yaml:"media"`

	// Custom marks a provider synthesized from a provider_nodes row rather than
	// loaded from the embedded document. A synthesized entry carries no model
	// list because the user supplies the model string.
	Custom bool `yaml:"-"`
}

// FormatOpenAIResponses is the wire format of an OpenAI Responses API upstream,
// which is a different endpoint and payload from chat completions. It is named
// here because both the registry document and a synthesized custom node refer
// to it.
const FormatOpenAIResponses = "openai-responses"

// Display carries the presentation fields the Providers screen renders.
type Display struct {
	Name              string            `yaml:"name"`
	Icon              string            `yaml:"icon"`
	Color             string            `yaml:"color"`
	TextIcon          string            `yaml:"text_icon"`
	Website           string            `yaml:"website"`
	Deprecated        bool              `yaml:"deprecated"`
	DeprecationNotice string            `yaml:"deprecation_notice"`
	Notice            Notice            `yaml:"notice"`
	KindNotice        map[string]string `yaml:"kind_notice"`
}

// Notice holds the outbound links a provider row offers.
type Notice struct {
	APIKeyURL string `yaml:"api_key_url"`
	SignupURL string `yaml:"signup_url"`
	Text      string `yaml:"text"`
}

// Features records which optional provider capabilities are wired. The tag
// keeps the document's camelCase spelling, because the tag is what the decode
// contract matches against and renaming the key on only one side would fail the
// strict decode.
type Features struct {
	Usage       bool `yaml:"usage"`
	UsageAPIKey bool `yaml:"usageApikey"`
}

// ThinkingConfig is the reasoning-mode control a provider exposes.
type ThinkingConfig struct {
	Options     []string `yaml:"options"`
	DefaultMode string   `yaml:"default_mode"`
}

// Model is one routable model inside a provider entry. UpstreamModelID is set
// when the exposed id differs from the id the upstream expects, Kind marks a
// non-chat model, and Capabilities lists extra operations a media model accepts
// (an image model that also edits). Thinking is tri-state: absent means the
// model follows its provider's thinking configuration, not an explicit false.
// SupportedFormats is the per-model endpoint guard a multi-endpoint provider
// needs, so a client on the claude wire never reaches /chat/completions for a
// model that only declares `openai`; empty means every transport is allowed.
type Model struct {
	ID               string   `yaml:"id"`
	Name             string   `yaml:"name"`
	UpstreamModelID  string   `yaml:"upstream_model_id"`
	Kind             string   `yaml:"kind"`
	Params           []string `yaml:"params"`
	Capabilities     []string `yaml:"capabilities"`
	QuotaFamily      string   `yaml:"quota_family"`
	Strip            []string `yaml:"strip"`
	TargetFormat     string   `yaml:"target_format"`
	SupportedFormats []string `yaml:"supported_formats"`
	Dimensions       int      `yaml:"dimensions"`
	Thinking         *bool    `yaml:"thinking"`
	// Description is the upstream's own one-line summary of the model, which
	// the reference carries for the panel's model list.
	Description string `yaml:"description"`
	// ContextLength and MaxOutputTokens are the declared limits. They are
	// carried because the reference's catalog reports them and a client that
	// reads a model's ceiling should read the declared one rather than a
	// guess; zero means the entry declares none.
	ContextLength   int `yaml:"context_length"`
	MaxOutputTokens int `yaml:"max_output_tokens"`
	// MinOutputTokens is the smallest output ceiling this model can still
	// answer within. A reasoning model that spends the ceiling thinking returns
	// an empty body under a small one, which is the model's behaviour rather
	// than anything the client did wrong, so the floor is declared per model and
	// a connector raises a ceiling below it. Zero declares none.
	MinOutputTokens int `yaml:"min_output_tokens"`
	// RateMultiplier scales a metered provider's own cost accounting when one
	// upstream unit is not one unit of the provider's quota.
	RateMultiplier float64 `yaml:"rate_multiplier"`
	// ImageGen marks a model that produces an image rather than text.
	ImageGen bool `yaml:"image_gen"`
}

// UpstreamID reports the id the upstream expects: the override when one is
// declared, otherwise the exposed id.
func (m Model) UpstreamID() string {
	if m.UpstreamModelID != "" {
		return m.UpstreamModelID
	}
	return m.ID
}

// IsChat reports whether the model serves chat completions. A model with no
// kind is chat; a media kind is not routable through the chat data plane.
func (m Model) IsChat() bool {
	switch m.Kind {
	case "", "llm", "chat":
		return true
	default:
		return false
	}
}
