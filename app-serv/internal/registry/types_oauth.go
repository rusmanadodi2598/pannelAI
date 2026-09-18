// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/types_oauth.go
// @for       The OAuth flow configuration a registry entry may declare.
// @uses      gopkg.in/yaml.v3 for the union-typed scopes field.
// @reason    SPEC-API-001 §7.4 exposes provider OAuth, and the reference's
//
//	providers differ so widely here (redirect, device code, poll, AWS
//	SSO) that the union of their fields has to be carried explicitly;
//	a free-form map would let a misspelled key disable a flow without
//	saying so.
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

// StringList accepts the scalar and sequence shapes one field uses across the
// registry, so a provider that declares `scopes: "a b"` and one that declares
// `scopes: [a, b]` decode to the same value.
type StringList []string

// UnmarshalYAML accepts a single scalar or a sequence of scalars.
func (l *StringList) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		if node.Value == "" {
			*l = nil
			return nil
		}
		*l = []string{node.Value}
		return nil
	case yaml.SequenceNode:
		var items []string
		if err := node.Decode(&items); err != nil {
			return err
		}
		*l = items
		return nil
	default:
		return fmt.Errorf("registry: expected a string or a list of strings")
	}
}

// OAuth is the provider's authorization flow configuration.
type OAuth struct {
	ClientID          string            `yaml:"client_id"`
	ClientSecret      string            `yaml:"client_secret"`
	AuthorizeURL      string            `yaml:"authorize_url"`
	AuthorizeURLPath  string            `yaml:"authorize_url_path"`
	TokenURL          string            `yaml:"token_url"`
	TokenURLPath      string            `yaml:"token_url_path"`
	TokenExchangeURL  string            `yaml:"token_exchange_url"`
	RefreshURL        string            `yaml:"refresh_url"`
	DeviceCodeURL     string            `yaml:"device_code_url"`
	DeviceAuthURL     string            `yaml:"device_auth_url"`
	DeviceTokenURL    string            `yaml:"device_token_url"`
	UserInfoURL       string            `yaml:"user_info_url"`
	UserInfoURLPath   string            `yaml:"user_info_url_path"`
	AuthURL           string            `yaml:"auth_url"`
	StateURL          string            `yaml:"state_url"`
	CopilotTokenURL   string            `yaml:"copilot_token_url"`
	ChatEndpoint      string            `yaml:"chat_endpoint"`
	ModelsEndpoint    string            `yaml:"models_endpoint"`
	API3Endpoint      string            `yaml:"api3_endpoint"`
	AgentEndpoint     string            `yaml:"agent_endpoint"`
	AgentNonPrivacy   string            `yaml:"agent_non_privacy_endpoint"`
	LoadCodeAssistUA  string            `yaml:"load_code_assist_user_agent"`
	LoadCodeAssistAPI string            `yaml:"load_code_assist_api_client"`
	BaseURL           string            `yaml:"base_url"`
	DefaultBaseURL    string            `yaml:"default_base_url"`
	APIBaseURL        string            `yaml:"api_base_url"`
	APIEndpoint       string            `yaml:"api_endpoint"`
	APIVersion        string            `yaml:"api_version"`
	AppBaseURL        string            `yaml:"app_base_url"`
	OpenAPIBaseURL    string            `yaml:"open_api_base_url"`
	CenterBaseURL     string            `yaml:"center_base_url"`
	ChatBaseURL       string            `yaml:"chat_base_url"`
	InitiateURL       string            `yaml:"initiate_url"`
	PollURLBase       string            `yaml:"poll_url_base"`
	QuotaUsageURL     string            `yaml:"quota_usage_url"`
	LoginURL          string            `yaml:"login_url"`
	RegisterClientURL string            `yaml:"register_client_url"`
	SSOOIDCEndpoint   string            `yaml:"sso_oidc_endpoint"`
	StartURL          string            `yaml:"start_url"`
	IssuerURL         string            `yaml:"issuer_url"`
	SocialAuthURL     string            `yaml:"social_auth_endpoint"`
	SocialLoginURL    string            `yaml:"social_login_url"`
	SocialTokenURL    string            `yaml:"social_token_url"`
	SocialRefreshURL  string            `yaml:"social_refresh_url"`
	LoadCodeAssistURL string            `yaml:"load_code_assist_endpoint"`
	OnboardUserURL    string            `yaml:"onboard_user_endpoint"`
	UserAgent         string            `yaml:"user_agent"`
	Platform          string            `yaml:"platform"`
	ClientName        string            `yaml:"client_name"`
	ClientType        string            `yaml:"client_type"`
	ClientVersion     string            `yaml:"client_version"`
	Scope             string            `yaml:"scope"`
	Scopes            StringList        `yaml:"scopes"`
	AuthMethods       []string          `yaml:"auth_methods"`
	GrantTypes        []string          `yaml:"grant_types"`
	ExtraParams       map[string]string `yaml:"extra_params"`
	DBKeys            map[string]string `yaml:"db_keys"`
	CallbackPath      string            `yaml:"callback_path"`
	CodeChallenge     string            `yaml:"code_challenge_method"`
	EditorVersion     string            `yaml:"editor_version"`
	EditorPlugin      string            `yaml:"editor_plugin_version"`
	Refresh           *OAuthRefresh     `yaml:"refresh"`
	FixedPort         int               `yaml:"fixed_port"`
	PollIntervalMS    int               `yaml:"poll_interval"`
	RefreshLeadMS     int64             `yaml:"refresh_lead_ms"`
	MaxRefreshAgeMS   int64             `yaml:"max_refresh_age_ms"`
	TrackRefreshAt    bool              `yaml:"track_refresh_at"`
}

// OAuthRefresh is how a provider's refresh grant encodes its request.
type OAuthRefresh struct {
	Encoding string `yaml:"encoding"`
	Scope    string `yaml:"scope"`
}

// ScopeList returns the requested scopes whichever shape the entry used.
func (o *OAuth) ScopeList() []string {
	if len(o.Scopes) > 0 {
		return o.Scopes
	}
	if o.Scope != "" {
		return []string{o.Scope}
	}
	return nil
}

// SupportsFlow reports whether the entry carries enough to start an
// authorization: either a browser redirect or a device-code endpoint. The
// panel uses this to decide whether to offer "Connect" for a provider, so the
// rule lives with the data rather than in the UI.
func (o *OAuth) SupportsFlow() bool {
	if o == nil {
		return false
	}
	return o.AuthorizeURL != "" || o.DeviceCodeURL != "" || o.DeviceAuthURL != "" || o.StateURL != ""
}

// RequiresCustomExchange reports whether this provider's token exchange cannot
// be performed by a generic OAuth client. A poll-based or pre-authenticated
// flow does not answer a standard authorization-code request, so it needs a
// connector rather than the shared fallback.
func (o *OAuth) RequiresCustomExchange() bool {
	if o == nil {
		return false
	}
	return o.StateURL != "" || o.InitiateURL != "" || o.PollURLBase != ""
}
