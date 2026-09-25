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
}

// Credentials carries what a family's endpoint authenticates with. A family uses what it
// uses (an OAuth bearer token, an API key, or either); the empty fields are simply unused.
type Credentials struct {
	AccessToken          string
	APIKey               string
	ProviderSpecificData map[string]string

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
