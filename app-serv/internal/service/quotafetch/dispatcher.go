// Dispatcher: the family map that mirrors the reference's USAGE_HANDLERS.
//
// @file      internal/service/quotafetch/dispatcher.go
// @for       Routes one quota read to its family's fetcher, with the reference's soft answer for unknown families.
// @uses      internal/service/quotafetch, net/http
// @reason    One entry point for callers so per-family wiring stays in one map, like the reference.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-25
package quotafetch

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// requestTimeout bounds every outbound quota read. The reference rate-limits some of
// these endpoints (Claude 429s), so a hung read must not hold a worker slot.
const requestTimeout = 20 * time.Second

// client is the outbound client every fetch shares. The timeout lives here rather than
// per request because the package owns the whole call.
var client = &http.Client{Timeout: requestTimeout}

// familyEndpoints are the usage endpoints the reference registry carries
// (open-sse/providers/registry/<family>.js, transport.usage.url).
var familyEndpoints = map[string]string{
	"vercel-ai-gateway": "https://ai-gateway.vercel.sh/v1/credits",
	"codebuddy-cn":      "https://copilot.tencent.com/v2/billing/meter/get-user-resource",
	"codebuddy-intl":    "https://www.codebuddy.ai/v2/billing/meter/get-user-resource",
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

// Fetch reads one provider's published quota. An unknown family answers the reference's
// soft message rather than an error, because the caller renders that sentence on the
// provider's card: "not implemented" is a fact about the family, not a failed request.
func Fetch(ctx context.Context, family string, creds Credentials) Result {
	fetch, ok := familyFetchers[family]
	if !ok {
		return Result{Message: fmt.Sprintf("Usage API not implemented for %s", family)}
	}

	bounded, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	return fetch(bounded, creds)
}

// familyFetchers is the port of USAGE_HANDLERS. Families land here as they are ported;
// the reference's remaining families (claude, codex, gemini-cli, antigravity, and the
// rest) are recorded in docs/PORT/004-PORT-QUOTA-TRACKER.md §7.4 as follow-up work.
var familyFetchers = map[string]func(context.Context, Credentials) Result{
	"vercel-ai-gateway": fetchVercel,
	"codebuddy-cn":      fetchCodeBuddy(codebuddyCN),
	"codebuddy-intl":    fetchCodeBuddy(codebuddyIntl),
}

type codebuddyFamily struct {
	name    string            // human-facing region word for messages
	headers map[string]string // the registry transport headers the billing endpoint expects
}

var codebuddyCN = codebuddyFamily{
	name: "CN",
	headers: map[string]string{
		"User-Agent":          "CLI/2.108.1 CodeBuddy/2.108.1",
		"X-Product":           "SaaS",
		"X-IDE-Type":          "CLI",
		"X-IDE-Name":          "CLI",
		"x-requested-with":    "XMLHttpRequest",
		"x-codebuddy-request": "1",
	},
}

var codebuddyIntl = codebuddyFamily{
	name: "Intl",
	headers: map[string]string{
		"User-Agent":          "IDE/2.108.1 CodeBuddy/2.108.1",
		"X-Product":           "SaaS",
		"X-IDE-Type":          "IDE",
		"X-IDE-Name":          "IDE",
		"x-requested-with":    "XMLHttpRequest",
		"x-codebuddy-request": "1",
	},
}
