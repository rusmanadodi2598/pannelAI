// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/types_transport.go
// @for       Transport, authentication, retry, and OAuth shapes of a provider.
// @uses      gopkg.in/yaml.v3 for the two union-typed fields.
// @reason    SPEC-API-001 §5 defines the registry entry's transport as the
//
//	runtime HTTP contract; the reference declares two of its fields
//	with more than one YAML shape, so those two get an explicit
//	decoder here instead of a loosened type that hides the union.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package registry

import (
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Transport is the runtime HTTP configuration for one provider.
//
// AuthType and NoAuth are declared here as well as on Provider because the
// reference writes them at one level or the other depending on the entry; the
// loader reads whichever is set, so carrying both keeps the document verbatim
// instead of forcing the generator to normalise a field whose position is not
// semantically meaningful.
type Transport struct {
	BaseURL         string            `yaml:"base_url"`
	BaseURLs        []string          `yaml:"base_urls"`
	Format          string            `yaml:"format"`
	URLSuffix       string            `yaml:"url_suffix"`
	ForceStream     bool              `yaml:"force_stream"`
	TimeoutMS       int               `yaml:"timeout_ms"`
	StallTimeoutMS  int               `yaml:"stall_timeout_ms"`
	ValidateURL     string            `yaml:"validate_url"`
	ResponsesURL    string            `yaml:"responses_url"`
	ChatPath        string            `yaml:"chat_path"`
	AuthType        string            `yaml:"auth_type"`
	NoAuth          bool              `yaml:"no_auth"`
	Headers         map[string]string `yaml:"headers"`
	Auth            AuthConfig        `yaml:"auth"`
	Quirks          Quirks            `yaml:"quirks"`
	Retry           Retry             `yaml:"retry"`
	Usage           UsageConfig       `yaml:"usage"`
	ReasoningInject *ReasoningInject  `yaml:"reasoning_inject"`
	Regions         map[string]string `yaml:"regions"`
	DefaultRegion   string            `yaml:"default_region"`
	ThinkingFormat  string            `yaml:"thinking_format"`
	CLIVersion      string            `yaml:"cli_version"`
	ClientVersion   string            `yaml:"client_version"`
	APIClient       string            `yaml:"api_client"`
	AuthURL         string            `yaml:"auth_url"`
	Copilot         *CopilotConfig    `yaml:"copilot"`
}

// AuthConfig describes how a credential is placed on an outbound request.
// APIKey and OAuth override the top-level header when a provider reads a
// different header per credential family; Combined means one header carries
// the credential for both families.
type AuthConfig struct {
	Header string `yaml:"header"`
	Scheme string `yaml:"scheme"`
	// AuthQuery places the credential as a query parameter with this name
	// instead of a header. Some providers read it only from the URL — the gemini
	// family's models endpoint takes `?key=` — and the reference applies the same
	// rule in its models route (`authQuery`, models/route.js:189, :634-637).
	//
	// It is mutually exclusive with Header: a request that sent both would put
	// the credential in a URL and a header at once, and the reference's route
	// takes the query branch before it considers a header.
	AuthQuery string      `yaml:"auth_query"`
	Source    []string    `yaml:"source"`
	Combined  bool        `yaml:"combined"`
	Hooks     []string    `yaml:"hooks"`
	OAuth     *AuthScheme `yaml:"oauth"`
	APIKey    *AuthScheme `yaml:"api_key"`
	// AnthropicVersion marks an endpoint that requires the Anthropic wire's
	// `anthropic-version` header. The reference declares it on the transport's
	// own auth object (registry/opencode-go.js:31-35), beside the header and
	// scheme the endpoint reads, which is why it lives here rather than being
	// derived from the URL by a connector.
	AnthropicVersion bool `yaml:"anthropic_version"`
}

// AuthScheme is one credential family's header and prefix.
type AuthScheme struct {
	Header string `yaml:"header"`
	Scheme string `yaml:"scheme"`
}

// TransportEndpoint is one wire-specific endpoint of a multi-endpoint provider.
// It is the unit `transports[]` lists: a format, the URL that answers it, and
// the credential placement that URL reads.
type TransportEndpoint struct {
	Format    string            `yaml:"format"`
	BaseURL   string            `yaml:"base_url"`
	Headers   map[string]string `yaml:"headers"`
	URLSuffix string            `yaml:"url_suffix"`
	Auth      AuthConfig        `yaml:"auth"`
}

// Quirks are per-provider request adjustments the translator must apply.
type Quirks struct {
	CloakToolsOnOAuth  bool `yaml:"cloak_tools_on_oauth"`
	DropClientMetadata bool `yaml:"drop_client_metadata"`
	DropOutputConfig   bool `yaml:"drop_output_config"`
	// ForceAutoToolChoiceModels names the models whose `tool_choice` the
	// provider's Responses wire refuses unless it is `auto`. The reference
	// declares it per provider (registry/opencode.js:22-24) and the connector
	// reads it rather than hardcoding an id, so a model that gains the quirk
	// upstream is one registry line away.
	ForceAutoToolChoiceModels []string `yaml:"force_auto_tool_choice_models"`
	// PreserveCacheControl keeps the client's `cache_control` markers instead
	// of stripping them.
	PreserveCacheControl bool `yaml:"preserve_cache_control"`
}

// SystemOneConfig is the native decision-model endpoint a provider exposes
// (the reference's `kind: "systemone"`). Its body is the provider's own
// vocabulary rather than a chat payload, which is why it is a separate block
// and a separate route instead of a chat model (SPEC-API-001 §7.15).
type SystemOneConfig struct {
	BaseURL string            `yaml:"base_url"`
	Headers map[string]string `yaml:"headers"`
}

// ReasoningInject is the scope over which a provider accepts reasoning fields.
type ReasoningInject struct {
	Scope string `yaml:"scope"`
}

// CopilotConfig carries the editor-identity values GitHub Copilot expects.
type CopilotConfig struct {
	VSCodeVersion string `yaml:"vscode_version"`
	ChatVersion   string `yaml:"chat_version"`
	UserAgent     string `yaml:"user_agent"`
	APIVersion    string `yaml:"api_version"`
}

// UsageConfig is the provider's quota-reporting surface (SPEC-API-001 §7.12).
type UsageConfig struct {
	URL                    string   `yaml:"url"`
	URLs                   []string `yaml:"urls"`
	TokenURL               string   `yaml:"token_url"`
	QuotaURL               string   `yaml:"quota_url"`
	QuotaAPIURL            string   `yaml:"quota_api_url"`
	LoadCodeAssistURL      string   `yaml:"load_code_assist_url"`
	LoadProjectAPIURL      string   `yaml:"load_project_api_url"`
	OrgURL                 string   `yaml:"org_url"`
	SettingsURL            string   `yaml:"settings_url"`
	LimitsPath             string   `yaml:"limits_path"`
	ResetCreditsConsumeURL string   `yaml:"reset_credits_consume_url"`
	ResetCreditsURL        string   `yaml:"reset_credits_url"`
	QuotaSummaryAPIURL     string   `yaml:"quota_summary_api_url"`
	UserURL                string   `yaml:"user_url"`
	OAuthURL               string   `yaml:"oauth_url"`
	CWHost                 string   `yaml:"cw_host"`
	QHost                  string   `yaml:"q_host"`
}

// Retry is a provider's attempt override. The reference declares three shapes
// for one field: a count, a per-status count, and a per-status object. Both
// are normalised here, because a decoder that accepted only one would silently
// drop the others.
type Retry struct {
	DefaultAttempts int

	// ByStatus overrides DefaultAttempts for one upstream status code.
	ByStatus map[int]int
}

// Attempts reports how many times to retry a given upstream status.
func (r Retry) Attempts(status int) int {
	if n, ok := r.ByStatus[status]; ok {
		return n
	}
	return r.DefaultAttempts
}

// UnmarshalYAML accepts the scalar and mapping shapes of the retry field.
func (r *Retry) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		n, err := strconv.Atoi(node.Value)
		if err != nil {
			return fmt.Errorf("registry: retry must be a number or a map: %q", node.Value)
		}
		r.DefaultAttempts = n
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("registry: retry must be a number or a map")
	}

	var raw map[string]yaml.Node
	if err := node.Decode(&raw); err != nil {
		return err
	}
	r.ByStatus = make(map[int]int, len(raw))
	for key, value := range raw {
		status, err := strconv.Atoi(key)
		if err != nil {
			return fmt.Errorf("registry: retry key %q is not a status code", key)
		}
		attempts, err := retryAttempts(value)
		if err != nil {
			return err
		}
		r.ByStatus[status] = attempts
	}
	return nil
}

// retryAttempts reads either the count form or the {attempts: n} form.
func retryAttempts(node yaml.Node) (int, error) {
	if node.Kind == yaml.ScalarNode {
		return strconv.Atoi(node.Value)
	}
	var shaped struct {
		Attempts int `yaml:"attempts"`
	}
	if err := node.Decode(&shaped); err != nil {
		return 0, err
	}
	return shaped.Attempts, nil
}
