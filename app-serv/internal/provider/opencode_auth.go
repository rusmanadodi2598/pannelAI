// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_auth.go
// @for       The credential and identity the OpenCode connector presents, per
//
//	endpoint.
//
// @uses      net/http, strings, internal/registry.
// @reason    A keyed OpenCode lane reads a different header per wire: the chat
//
//	endpoint takes `Authorization: Bearer`, and the Messages endpoint takes
//	a raw `x-api-key`. The free lane reads neither, because it pools
//	anonymous traffic and presents the literal public bearer. Both rules
//	are about the endpoint the request reached, which is why they live
//	together and apart from the URL rule in opencode.go (AGENTS.md §1.1).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-24
package provider

import (
	"net/http"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// ApplyAuth places the free tier's credential and identity on the request. The
// credential is always the literal public bearer, because the free tier pools
// anonymous traffic: a configured key would be ignored and would leak to an
// endpoint that has no use for it. The session is derived from the endpoint, so
// one account presents one stable identity. A request on the Anthropic Messages
// leaf also names the API version that wire requires, decided from the URL rather
// than the model, so a claude-target model on the go lane gets it at
// /zen/go/v1/messages too. A version the entry declared is kept.
func (c *OpenCode) ApplyAuth(req *http.Request, cred Credential) error {
	// The auth path has no model to read a provider override from, so it uses
	// the entry the connector was built for.
	entry := c.entry
	// A keyed lane places the credential the way the endpoint it reached
	// declares, because a multi-endpoint provider reads a different header per
	// wire. A keyless entry never does; the free-lane path below always applies.
	if !entry.NoAuth && entry.AuthType != registry.AuthNone {
		family, value, err := cred.Secret()
		if err != nil {
			return err
		}
		if value != "" {
			if auth, ok := c.endpointAuth(entry, req.URL.Path); ok {
				return applyOpenCodeAuth(req, auth, family, value)
			}
			return applyOpenCodeAuth(req, entry.Transport.Auth, family, value)
		}
	}
	req.Header.Set("Authorization", "Bearer public")
	req.Header.Set("User-Agent", OpenCodeUserAgent)
	req.Header.Set(openCodeClientHeader, c.clientValue(req))
	req.Header.Set(openCodeSessionHeader, OpenCodeSession(cred.EndpointID()))
	req.Header.Set(openCodeProjectHeader, openCodeProjectValue)
	if isOpenCodeMessagesURL(req.URL.Path) && req.Header.Get(openCodeAnthropicVersionHeader) == "" {
		req.Header.Set(openCodeAnthropicVersionHeader, OpenCodeAnthropicVersion)
	}
	return nil
}

// endpointAuth finds the declared credential placement for the endpoint a URL
// names, so the header follows the wire rather than the provider's default.
func (c *OpenCode) endpointAuth(entry registry.Provider, path string) (registry.AuthConfig, bool) {
	if len(entry.Transports) == 0 {
		return registry.AuthConfig{}, false
	}
	wire := openCodeWireFromPath(path)
	for _, endpoint := range entry.Transports {
		if strings.TrimSpace(endpoint.Format) == wire {
			return endpoint.Auth, true
		}
	}
	return registry.AuthConfig{}, false
}

// openCodeWireFromPath names the wire a URL path belongs to, which is what ties
// an authenticated request back to its endpoint without threading the model
// through. The three leaves are the reference's own vocabulary.
func openCodeWireFromPath(path string) string {
	switch {
	case isOpenCodeMessagesURL(path):
		return openCodeClaudeWire
	case strings.HasSuffix(strings.TrimRight(path, "/"), openCodeResponsesLeaf):
		return registry.FormatOpenAIResponses
	default:
		return registry.DefaultFormat
	}
}

// applyOpenCodeAuth places a credential the way one auth configuration declares,
// defaulting to the registry's documented bearer when the entry states nothing.
func applyOpenCodeAuth(req *http.Request, auth registry.AuthConfig, family Family, value string) error {
	scheme := auth
	if family == FamilyStaticKey && auth.APIKey != nil {
		scheme = registry.AuthConfig{Header: auth.APIKey.Header, Scheme: auth.APIKey.Scheme}
	}
	header := strings.TrimSpace(scheme.Header)
	if header == "" {
		header = DefaultAuthHeader
	}
	switch strings.TrimSpace(scheme.Scheme) {
	case "raw":
		req.Header.Set(header, value)
	case "", "bearer":
		req.Header.Set(header, "Bearer "+value)
	default:
		req.Header.Set(header, scheme.Scheme+" "+value)
	}
	if scheme.AnthropicVersion && req.Header.Get(openCodeAnthropicVersionHeader) == "" {
		req.Header.Set(openCodeAnthropicVersionHeader, OpenCodeAnthropicVersion)
	}
	return nil
}
