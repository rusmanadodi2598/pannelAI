// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/default.go
// @for       The connector every provider without specialized handling uses.
// @uses      internal/registry, net/http, strings.
// @reason    Most registry providers speak a standard wire format with a static
//
//	credential, so they need no per-vendor code. This connector is what
//	makes those providers routable with zero extra source: adding an
//	entry to the embedded registry is enough, which is the property the
//	plugin seam exists to give.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package provider

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// DefaultAuthHeader and DefaultAuthScheme are the registry's documented default
// for a provider that declares no auth configuration: the reference's
// PROVIDER_DEFAULTS sends the credential as "Authorization: Bearer ...". Naming
// them keeps the default in one place instead of a literal in the request path.
const (
	DefaultAuthHeader = "Authorization"
	DefaultAuthScheme = "bearer"
)

// Default is the connector for a provider that needs no custom behaviour. It
// builds the URL from the registry entry, applies the credential using the
// header and scheme the entry declares, and defers everything else to Base.
type Default struct {
	Base

	// entry is the registry entry this connector was built for. It is held
	// rather than looked up per request so the connector has no dependency on
	// a global index, which keeps it testable on its own.
	entry registry.Provider
}

// NewDefault builds the connector for one registry entry.
func NewDefault(entry registry.Provider) *Default {
	return &Default{
		Base: Base{
			ID:     entry.ID,
			Auth:   entry.AuthType,
			Format: entry.Transport.Format,
		},
		entry: entry,
	}
}

// DefaultFactory returns the fallback factory a Connectors needs. It is what a
// provider gets when nothing specialized is registered for it.
func DefaultFactory(entry registry.Provider) Plugin { return NewDefault(entry) }

// Endpoint builds the absolute URL to call.
//
// The registry stores a full chat URL rather than a base, because providers
// disagree about where the path ends ("/v1/chat/completions",
// "/v1/messages", a suffix with a query). So the join is "use the entry's URL,
// appending what it declares" and there is no path guessing.
//
// One declaration changes that reading: an entry carrying a chat_path is saying
// its base_url is a BASE, not an endpoint. A custom node stores
// "https://host/v1" and relies on the path to complete it, and the reference's
// BaseExecutor.buildUrl joins the two the same way. A Responses-format entry may
// instead declare a complete responses_url, which takes precedence. A path-less
// entry keeps the full-URL reading, which is what the embedded registry relies on.
func (d *Default) Endpoint(req Request, _ Credential) (string, error) {
	transport := req.Provider.Transport

	// A provider may declare several hosts for failover; the first is the
	// primary. Choosing among them is the transport's job, which is why this
	// returns one URL rather than a list.
	base := strings.TrimSpace(transport.BaseURL)
	if base == "" && len(transport.BaseURLs) > 0 {
		base = strings.TrimSpace(transport.BaseURLs[0])
	}
	if base == "" {
		return "", fmt.Errorf("provider %s: no base_url is configured", req.Provider.ID)
	}

	url := base
	if transport.Format == registry.FormatOpenAIResponses && strings.TrimSpace(transport.ResponsesURL) != "" {
		// Responses-format entries may keep a chat-shaped base_url for
		// compatibility while declaring the actual Responses endpoint
		// separately. The explicit endpoint wins and is already complete.
		url = strings.TrimSpace(transport.ResponsesURL)
	} else if path := strings.TrimSpace(transport.ChatPath); path != "" {
		url = joinPath(base, path)
	}
	if transport.URLSuffix != "" {
		// A suffix may be a query ("?beta=true") or a path fragment. Appending
		// covers both; the entry is responsible for the leading character, and
		// it goes last so a query lands after the path it belongs to.
		url += transport.URLSuffix
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return "", fmt.Errorf("provider %s: base_url is not absolute", req.Provider.ID)
	}
	return url, nil
}

// joinPath appends a declared path to a base URL with exactly one separator, so
// a base written with a trailing slash ("https://host/v1/") and a path written
// with or without a leading one both produce one correct URL instead of a
// doubled slash that some upstreams treat as a different path.
func joinPath(base, path string) string {
	return strings.TrimSuffix(base, "/") + "/" + strings.TrimPrefix(path, "/")
}

// ApplyAuth places the credential on the request using the entry's declared
// header and scheme.
//
// This is the one place the core would otherwise hardcode a bearer token, and
// providers disagree widely: most read Authorization, Anthropic reads
// x-api-key, and several read a different header per credential family
// (an OAuth token versus a static key). The registry records that, so the rule
// is data here rather than a branch per provider.
func (d *Default) ApplyAuth(req *http.Request, cred Credential) error {
	family, value := cred.family()
	if value == "" {
		// No credential material means the provider needs none, so nothing is
		// sent. This is deliberately the generous reading: a `no_auth` provider is
		// served by a zero-value Credential, so a strict reading would break
		// every credential-free provider while turning a caller's forgotten
		// credential into an indistinguishable case. The configuration mistake
		// that actually matters is caught by authFor below, which refuses when
		// the entry routes this family through a different header.
		return nil
	}

	auth, found := d.authFor(family)
	if !found {
		return fmt.Errorf("provider %s: this credential has no valid header for %s", d.ID, family)
	}
	header := strings.TrimSpace(auth.Header)
	if header == "" {
		header = DefaultAuthHeader
	}
	scheme := strings.TrimSpace(auth.Scheme)
	if scheme == "" && header == DefaultAuthHeader {
		// The registry's documented default for a provider that declares no auth
		// at all is Authorization + bearer, matching the reference's
		// PROVIDER_DEFAULTS. Treating an undeclared scheme as raw would send a
		// bare secret where every OpenAI-compatible API expects "Bearer ".
		scheme = DefaultAuthScheme
	}

	switch scheme {
	case "", "raw":
		req.Header.Set(header, value)
	case "bearer":
		req.Header.Set(header, "Bearer "+value)
	default:
		req.Header.Set(header, scheme+" "+value)
	}
	return nil
}

// authFor picks the configuration for the credential family actually in use.
//
// A provider that declares a per-family header gets that one; otherwise the
// shared configuration applies. The `found` result is false when the entry
// declares a family-specific header for the OTHER family only, which means this
// credential cannot be presented correctly rather than that it should be sent
// with the wrong header.
func (d *Default) authFor(family Family) (registry.AuthConfig, bool) {
	auth := d.entry.Transport.Auth
	switch family {
	case FamilyStaticKey:
		if auth.APIKey != nil {
			return registry.AuthConfig{Header: auth.APIKey.Header, Scheme: auth.APIKey.Scheme}, true
		}
		if auth.OAuth != nil {
			// The entry routes OAuth through one header and static keys through
			// another, so a static key has no correct placement here.
			return registry.AuthConfig{}, false
		}
	case FamilyOAuth:
		if auth.OAuth != nil {
			return registry.AuthConfig{Header: auth.OAuth.Header, Scheme: auth.OAuth.Scheme}, true
		}
		if auth.APIKey != nil {
			return registry.AuthConfig{}, false
		}
	}
	return auth, true
}
