// Package quotafetch reads quota windows a provider itself publishes, for the provider
// families that expose a usage API (the reference's USAGE_HANDLERS set).
//
// @file      internal/service/quotafetch/quotafetch.go
// @for       Shared types and parsing for per-provider quota fetchers.
// @uses      net/http, encoding/json, time
// @reason    Provider-reported windows (QuotaWindow.Report) need one shape every family's fetcher answers.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-25
package quotafetch

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// Quota is one window a provider publishes. Used and Total are in the provider's own
// counter, which is not always requests (Vercel's is USD); the panel labels the unit,
// not this package.
type Quota struct {
	Label     string
	Used      float64
	Total     float64
	Unlimited bool
	// Unit names what Used and Total count, in the provider's own dimension ("requests",
	// "tokens", "USD", "%"). Empty means the provider stated a bare counter, which is the
	// honest answer rather than a missing one, and the panel prints it without a suffix.
	Unit string
	// IsCreditBalance marks a money balance rather than a capped window: a prepaid wallet
	// has no share to bar, and drawing one would render "12.34 of something" where the
	// provider only ever claimed 12.34 USD left. The panel renders it as an amount.
	IsCreditBalance bool
	// ResetAt is when the window refills. Zero means the provider did not report one.
	// For a non-recurring quota (a one-shot pack) it is the expiry, not a refill.
	ResetAt   time.Time
	Recurring bool
}

// Result is what one fetch answers. A non-empty Message is a soft outcome — the family
// is not implemented, the credential is missing or refused, the provider answered with an
// error — and Quotas is then empty. A fetcher never turns a provider error into a Go error:
// the caller cannot tell a dead credential from a rate-limited one otherwise, and the
// reference's own cards render that sentence instead of failing.
type Result struct {
	Plan    string
	Quotas  []Quota
	Message string
	// Failed marks a sentence that the reference would have raised instead of returned.
	// Two of its usage handlers — github's and the cloudcode one antigravity reads
	// through — throw on a provider error, while the rest answer with a message; the
	// split is the reference's deliberate policy and survives the port as this flag
	// rather than as a Go error, because a fetcher has no caller to throw at. The
	// worker counts a failed poll and backs off; a plain message is an answer.
	Failed bool
}

// UsageEndpoints is every usage surface one provider's registry entry declares. The
// reference reads `transport.usage` off its registry, and a family asks one key of it:
// `url` for most, `quota_url` for the gemini CLI, `quota_api_url` for antigravity,
// `urls[]` for MiniMax (which answers from two hosts), `oauth_url` and `org_url` for
// Claude, `user_url` for Grok. Carrying the whole block rather than one URL is what
// keeps a family that asks a second key from silently calling an empty one.
//
// It is a mirror declared here rather than the registry's own struct, so this package
// stays reachable from a test without a registry and the layer rule that keeps
// `service` from importing config upward still holds: the mapping is one function in
// internal/service, and nothing here names the registry.
type UsageEndpoints struct {
	URL                 string
	URLs                []string
	TokenURL            string
	QuotaURL            string
	QuotaAPIURL         string
	LoadCodeAssistURL   string
	LoadProjectAPIURL   string
	OrgURL              string
	SettingsURL         string
	LimitsPath          string
	ResetCreditsConsume string
	ResetCredits        string
	QuotaSummaryAPIURL  string
	UserURL             string
	OAuthURL            string
	CWHost              string
	QHost               string
}

// Credentials carries what a family's endpoint authenticates with. A family uses what it
// uses (an OAuth bearer token, an API key, or either); the empty fields are simply unused.
type Credentials struct {
	AccessToken          string
	APIKey               string
	ProviderSpecificData map[string]string

	// Endpoints is the usage block the provider's own registry entry declares, which is
	// where the reference reads its URLs from too. An empty member means the entry
	// declares none for that purpose, and the family's built-in applies.
	Endpoints UsageEndpoints
	// UsageHeaders are the entry's transport headers: this billing endpoint is
	// reached with the same product identification the chat gateway demands, and
	// reading it from the entry keeps one declaration rather than two.
	UsageHeaders map[string]string

	// Endpoint overrides the family's real endpoint URL. It is the test seam: httptest
	// servers point a family at a stub this way, and production leaves it empty.
	Endpoint string
}

// parseReset reads the reset instants provider APIs answer with. The reference accepts a
// Unix timestamp in seconds or milliseconds (number or numeric string) and any string
// time.Parse reads; this port keeps both, because the ported families depend on it.
func parseReset(raw json.RawMessage) time.Time {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return time.Time{}
	}
	trimmed = strings.Trim(trimmed, `"`)
	if trimmed == "" {
		return time.Time{}
	}

	if unix, err := parseInt64(trimmed); err == nil {
		return fromUnix(unix)
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return parsed
	}
	return time.Time{}
}

// fromUnix treats a value below 1e12 as seconds and above as milliseconds, the
// reference's rule, because the two unit families both occur in provider answers.
func fromUnix(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	if value < 1e12 {
		return time.Unix(value, 0)
	}
	return time.UnixMilli(value)
}

func parseInt64(text string) (int64, error) {
	return strconv.ParseInt(text, 10, 64)
}

// num reads a count preferring the "*Precise" string field the billing APIs answer with,
// falling back to the numeric one, the reference's num() helper. A value that is neither
// parseable counts as zero rather than failing the whole read.
func num(precise string, plain json.Number) float64 {
	if precise != "" {
		var value float64
		if err := json.Unmarshal([]byte(precise), &value); err == nil {
			return value
		}
	}
	value, err := plain.Float64()
	if err != nil {
		return 0
	}
	return value
}
