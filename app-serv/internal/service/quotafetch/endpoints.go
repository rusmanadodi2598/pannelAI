// Usage endpoint resolution: which host a family's quota read goes to, and how the
// provider's own registry declaration overrides the built-in.
//
// @file      internal/service/quotafetch/endpoints.go
// @for       Resolving one family's usage URL from the registry block it declares.
// @uses      internal/service/quotafetch, net/url, strings
// @reason    A provider moves its billing endpoint without telling this gateway, and the registry is where an operator records the move; reading the declared host first is what makes that record take effect instead of a stale copy here.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package quotafetch

import (
	"net/url"
	"strings"
)

// familyEndpoints are the usage endpoints the reference registry carries
// (open-sse/providers/registry/<family>.js, transport.usage.url). A family lands here
// only when no provider entry declares its host; the declared block always wins.
var familyEndpoints = map[string]string{
	"codebuddy-cn":   "https://copilot.tencent.com/v2/billing/meter/get-user-resource",
	"codebuddy-intl": "https://www.codebuddy.ai/v2/billing/meter/get-user-resource",
	"qoder":          "https://openapi.qoder.sh/api/v2/quota/usage",
	"qoder-cn":       "https://openapi.qoder.com.cn/api/v2/quota/usage",
}

// usageEndpoint resolves the one URL a family's quota read goes to: the endpoint
// the provider's registry entry declares when it declares one, and the family's
// built-in otherwise. The built-in is not a legacy to delete but the fallback for
// an entry that declares no usage URL.
//
// The order matters for correctness rather than taste: the reference reads
// `transport.usage.url` off the registry, so a second copy here would silently
// keep asking the old host after an operator or an upstream moved the endpoint.
func usageEndpoint(creds Credentials, family string) string {
	return declaredOr(creds.Endpoints.URL, familyEndpoints[family])
}

// declaredOr takes the host a family's purpose names when the provider's registry entry
// declares one, and its built-in otherwise. A family with more than one surface (Claude's
// account endpoint and its organization endpoint, Grok's billing host and its user host)
// reads each typed member of the block through here, so an undeclared key degrades to the
// built-in rather than to a request against an empty URL.
func declaredOr(declared string, builtIn string) string {
	if trimmed := strings.TrimSpace(declared); trimmed != "" {
		return trimmed
	}
	return builtIn
}

// endpointFor overrides the scheme and host of a family's endpoint while keeping the
// family's path, which is what a test stub needs to intercept a call without changing
// what the family requests.
func endpointFor(familyURL string, override string) string {
	if override == "" {
		return familyURL
	}
	parsed, err := url.Parse(familyURL)
	if err != nil {
		return familyURL
	}
	return override + parsed.Path
}
