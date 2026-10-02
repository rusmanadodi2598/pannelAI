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
	"time"
)

// requestTimeout bounds every outbound quota read. The reference rate-limits some of
// these endpoints (Claude 429s), so a hung read must not hold a worker slot.
const requestTimeout = 20 * time.Second

// client is the outbound client every fetch shares. The timeout lives here rather than
// per request because the package owns the whole call.
var client = &http.Client{Timeout: requestTimeout}

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

// familyFetchers is the port of USAGE_HANDLERS, keyed by the registry provider id the
// endpoint's own row names. A family the registry marks `usage: true` but that is absent
// here answers the soft "not implemented" message — which is why parity between this map
// and the registry is a test rather than a checklist someone has to remember.
var familyFetchers = map[string]func(context.Context, Credentials) Result{
	"claude":      fetchClaude,
	"commandcode": fetchCommandCode,
	"deepseek":    fetchDeepSeek,
	"gemini-cli":  fetchGeminiCLI,
	"antigravity": fetchAntigravity,
	"github":      fetchGitHub,
	"glm":         fetchGlm,
	"grok-cli":    fetchGrok,
	"groq":        fetchGroq,
	"kimi":        fetchKimi,
	"minimax":     fetchMiniMax,
	"zed":         fetchZed,
	// Two families share one handler each because the pair differs only by host or by
	// product identification, the same reason the reference keeps one function per pair.
	"codebuddy-cn":   fetchCodeBuddy(codebuddyCN),
	"codebuddy-intl": fetchCodeBuddy(codebuddyIntl),
	"opencode-zen":   fetchOpenCode(openCodeZen),
	"opencode-go":    fetchOpenCode(openCodeGo),
	// The Qoder pair shares one handler: the CN site answers the same shape from a
	// different service, and the exchange host follows the endpoint rather than a
	// table of regions.
	"qoder":    fetchQoder("qoder", "https://openapi.qoder.sh"),
	"qoder-cn": fetchQoder("qoder-cn", "https://openapi.qoder.com.cn"),
}
