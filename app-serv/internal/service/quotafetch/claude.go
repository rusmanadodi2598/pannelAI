// The Claude family: the OAuth usage endpoint Claude Code publishes for its own token,
// answering utilization percentages against a 100-point window, with the legacy
// organization-usage read behind it (claude_org.go) when the primary says nothing usable.
//
// @file      internal/service/quotafetch/claude.go
// @for       Asks Claude's OAuth usage endpoint and words its refusals.
// @uses      internal/service/quotafetch, context, net/http, strconv, strings, time
// @reason    The OAuth usage endpoint is rate-limited independently of chat and answers in
//
//	percentages rather than counters, so the read needs its own header set,
//	its own cooldown outcome, and a fallback that only runs where a
//	second endpoint could still answer something usable.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package quotafetch

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// claudeOAuthUsageURL is the built-in primary endpoint, declared here while endpoints.go
	// stays untouched. The integrator promotes it into familyEndpoints under "claude";
	// the settings and organization hosts sit with the legacy read in claude_org.go.
	claudeOAuthUsageURL = "https://api.anthropic.com/api/oauth/usage"

	claudeDisplay       = "Claude"
	claudePlanLabel     = "Claude Code"
	claudeAPIVersion    = "2023-06-01"
	claudeOAuthBetaFlag = "oauth-2025-04-20"

	// claudePercentWindow is what every published Claude window is a share of: the provider
	// states utilization as the percent USED of a 100-point allocation, never as a counter.
	claudePercentWindow = 100

	// claudeCooldownSeconds is how long the endpoint asks to be left alone after a 429. The
	// countdown itself belongs to the scheduler that owns this connection's retries; the
	// fetcher only names the number so that layer can act on it.
	claudeCooldownSeconds = 180

	claudeRateLimitedPrefix = "Claude quota endpoint rate limited (429). Retry after "
	claudeNoTokenMessage    = "Claude access token not available. Re-authorize the connection to view usage."
)

// fetchClaude is the "claude" entry.
func fetchClaude(ctx context.Context, creds Credentials) Result {
	token := strings.TrimSpace(creds.AccessToken)
	if token == "" {
		return Result{Plan: claudePlanLabel, Message: claudeNoTokenMessage}
	}

	endpoint := endpointFor(declaredOr(creds.Endpoints.OAuthURL, claudeOAuthUsageURL), creds.Endpoint)
	response, err := requestUsage(ctx, http.MethodGet, endpoint, claudeOAuthHeaders(token, creds.UsageHeaders), "")
	if err != nil {
		return Result{Plan: claudePlanLabel, Message: fmt.Sprintf("Claude connected. Unable to fetch usage: %s", err)}
	}

	// A 429 is the endpoint throttling its own meter, not the credential: chat with the same
	// token still works. Nothing is retried here and no fallback is attempted, because the
	// only useful answer is the cooldown, see claudeRateLimited for who owns the countdown.
	if response.status == http.StatusTooManyRequests {
		return Result{Plan: claudePlanLabel, Message: claudeRateLimited(claudeRetryAfter(response.header))}
	}
	// The legacy fallback presents this same bearer, so a refused token is reported once
	// rather than spent on a second endpoint that must refuse it too.
	if claudeFatalStatus(response.status) {
		failure, _ := response.softFailure(claudeDisplay)
		failure.Plan = claudePlanLabel
		return failure
	}
	if response.status >= 200 && response.status < 300 {
		if result, usable := claudeDocumentResult(response); usable {
			return result
		}
	}
	return claudeLegacy(ctx, creds, token)
}

// claudeFatalStatus keeps the two refusals apart from the provider's other failures: a dead
// credential is named as one, while a 5xx or a 404 still gets the legacy read the reference
// falls back to for any non-2xx.
func claudeFatalStatus(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden
}

// claudeRateLimited words the cooldown outcome. The sentence is machine-readable at its
// prefix and names the seconds to wait, so the worker that schedules this connection can
// hold its last good read for that long instead of showing this message.
func claudeRateLimited(retry time.Duration) string {
	seconds := int64(retry / time.Second)
	if seconds <= 0 {
		seconds = claudeCooldownSeconds
	}
	return fmt.Sprintf("%s%ds. Keep showing the last published quota until then.", claudeRateLimitedPrefix, seconds)
}

// claudeRetryAfter reads the endpoint's own wait, honouring a positive integer Retry-After
// and falling back to the reference's 180s cooldown. An HTTP-date Retry-After is not parsed:
// this endpoint answers seconds, and a wrong guess at a date would cool down too long.
func claudeRetryAfter(header http.Header) time.Duration {
	value := claudeCooldownSeconds * time.Second
	raw := strings.TrimSpace(header.Get("Retry-After"))
	if seconds, err := strconv.ParseInt(raw, 10, 64); err == nil && seconds > 0 {
		value = time.Duration(seconds) * time.Second
	}
	return value
}

// claudeOAuthHeaders presents the bearer plus the two version identification headers this
// endpoint reads. The beta flag is this family's own: the registry's chat header carries a
// whole chain of feature flags, and the usage endpoint wants exactly one.
func claudeOAuthHeaders(token string, extra map[string]string) map[string]string {
	headers := claudeVersionedHeaders(token, extra)
	headers["anthropic-beta"] = claudeOAuthBetaFlag
	return headers
}

// claudeVersionedHeaders is the shared base for every Claude read, primary and legacy alike.
// Declared headers are merged in but never allowed to replace the credential or the version
// flags, which are this family's own.
func claudeVersionedHeaders(token string, extra map[string]string) map[string]string {
	headers := make(map[string]string, len(extra)+2)
	for key, value := range extra {
		if claudeOwnedHeader(key) || strings.TrimSpace(value) == "" {
			continue
		}
		headers[key] = value
	}
	headers["Authorization"] = "Bearer " + token
	headers["anthropic-version"] = claudeAPIVersion
	return headers
}

func claudeOwnedHeader(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "authorization", "anthropic-beta", "anthropic-version":
		return true
	default:
		return false
	}
}
