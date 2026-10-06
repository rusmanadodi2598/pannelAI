// The Antigravity weekly buckets: v1internal:retrieveUserQuotaSummary read beside the
// per-model windows, giving the 5-hour session and the 7-day week for each model family.
//
// @file      internal/service/quotafetch/antigravity_weekly.go
// @for       Turns Antigravity's quota summary into its four session and weekly rows.
// @uses      internal/service/quotafetch, context, encoding/json, net/http, strings
// @reason    The summary is the only surface that states a window per family rather than per model, it answers under two different envelopes depending on the release, and it is best-effort: a summary that fails must never take the per-model windows down with it.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package quotafetch

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

const (
	// antigravityQuotaSummaryURL is the built-in summary host the antigravity entry declares
	// as quota_summary_api_url; declared here while endpoints.go stays untouched, for the
	// integrator to promote under "antigravity".
	antigravityQuotaSummaryURL = "https://daily-cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary"

	// The four window rows the card is named for. They are constants rather than inline
	// strings because the per-model read reconciles against the session labels by name.
	antigravitySessionGemini = "Gemini (5h)"
	antigravityWeeklyGemini  = "Gemini (Weekly)"
	antigravitySessionClaude = "Claude & GPT (5h)"
	antigravityWeeklyClaude  = "Claude & GPT (Weekly)"
)

// antigravityWindowOrder is the order the card is handed these rows in: each family's short
// session window before its week. The provider lists groups in whatever order it likes, so a
// fixed order is what keeps two reads of one account identical.
var antigravityWindowOrder = []string{
	antigravitySessionGemini,
	antigravityWeeklyGemini,
	antigravitySessionClaude,
	antigravityWeeklyClaude,
}

// antigravityWindowGroup is one model family and the two windows it is metered against. The
// reference matches the group's display name with /gemini/i and /claude|gpt/i.
type antigravityWindowGroup struct {
	matches func(string) bool
	weekly  string
	session string
}

var antigravityWindowGroups = []antigravityWindowGroup{
	{matches: antigravityGroupGemini, weekly: antigravityWeeklyGemini, session: antigravitySessionGemini},
	{matches: antigravityGroupClaudeGPT, weekly: antigravityWeeklyClaude, session: antigravitySessionClaude},
}

// antigravitySummary is the retrieveUserQuotaSummary document. The groups live at the top
// level on one release and inside a `quotaSummary` envelope on another, and both spellings
// occur in the wild.
type antigravitySummary struct {
	Groups       []antigravityGroup       `json:"groups"`
	QuotaSummary *antigravitySummaryBlock `json:"quotaSummary"`
}

type antigravitySummaryBlock struct {
	Groups []antigravityGroup `json:"groups"`
}

type antigravityGroup struct {
	DisplayName string                  `json:"displayName"`
	Buckets     []antigravityWeekBucket `json:"buckets"`
}

// antigravityWeekBucket is one published window. `disabled` is a pointer because the provider
// omitting it and setting it false are different statements only in as far as a missing flag
// must not be read as a disabled bucket.
type antigravityWeekBucket struct {
	BucketID          string          `json:"bucketId"`
	DisplayName       string          `json:"displayName"`
	Window            string          `json:"window"`
	Disabled          *bool           `json:"disabled"`
	RemainingFraction json.RawMessage `json:"remainingFraction"`
	ResetTime         json.RawMessage `json:"resetTime"`
}

// fetchAntigravityWeekly reads the summary. It is best-effort by contract: every failure,
// transport, refusal, an unreadable body, answers no rows and never a message, because the
// per-model windows already on the card are the better answer than nothing.
func fetchAntigravityWeekly(ctx context.Context, creds Credentials, token string, project string) []Quota {
	endpoint := endpointFor(declaredOr(creds.Endpoints.QuotaSummaryAPIURL, antigravityQuotaSummaryURL), creds.Endpoint)
	response, err := requestUsage(ctx, http.MethodPost, endpoint,
		antigravityHeaders(token, creds.UsageHeaders), googleProjectBody(project))
	if err != nil || response.status < 200 || response.status >= 300 {
		return nil
	}
	var summary antigravitySummary
	if err := json.Unmarshal(response.body, &summary); err != nil {
		return nil
	}
	return antigravityWeeklyRows(antigravitySummaryGroups(summary))
}

func antigravitySummaryGroups(summary antigravitySummary) []antigravityGroup {
	if len(summary.Groups) > 0 {
		return summary.Groups
	}
	if summary.QuotaSummary != nil {
		return summary.QuotaSummary.Groups
	}
	return nil
}

// antigravityWeeklyRows reads every bucket of every group and keeps the first that names each
// window, then hands the rows back in this family's fixed order.
func antigravityWeeklyRows(groups []antigravityGroup) []Quota {
	found := make(map[string]Quota, len(antigravityWindowOrder))
	for _, group := range groups {
		for _, bucket := range group.Buckets {
			label, row, ok := antigravityWeekRow(group.DisplayName, bucket)
			if !ok {
				continue
			}
			if _, taken := found[label]; !taken {
				found[label] = row
			}
		}
	}

	rows := make([]Quota, 0, len(found))
	for _, label := range antigravityWindowOrder {
		if row, ok := found[label]; ok {
			rows = append(rows, row)
		}
	}
	return rows
}

// antigravityWeekRow classifies one bucket and maps it to its window. A bucket in no known
// family, or of no window this card names, is not reported: an invented row would read as a
// window the provider never sold.
func antigravityWeekRow(groupName string, bucket antigravityWeekBucket) (string, Quota, bool) {
	weekly, session := antigravityBucketWindow(bucket)
	if !weekly && !session {
		return "", Quota{}, false
	}
	// A session window disabled upstream is still the session window, the week was hit, and
	// the card should show the 5-hour row empty rather than drop it. A disabled week is gone.
	disabled := bucket.Disabled != nil && *bucket.Disabled
	if disabled && weekly {
		return "", Quota{}, false
	}

	fraction := 0.0
	if !disabled {
		value, ok := googleFraction(bucket.RemainingFraction)
		if !ok {
			return "", Quota{}, false
		}
		fraction = value
	}

	for _, group := range antigravityWindowGroups {
		if !group.matches(strings.ToLower(strings.TrimSpace(groupName))) {
			continue
		}
		label := group.session
		if weekly {
			label = group.weekly
		}
		return label, googleShareRow(label, fraction, parseReset(bucket.ResetTime)), true
	}
	return "", Quota{}, false
}

// antigravityBucketWindow classifies a bucket by its window type, falling back to the words
// the provider puts in its id and name, older releases state no window at all.
func antigravityBucketWindow(bucket antigravityWeekBucket) (bool, bool) {
	window := strings.ToLower(strings.TrimSpace(bucket.Window))
	text := strings.ToLower(bucket.BucketID + " " + bucket.DisplayName)

	weekly := window == "weekly" || strings.Contains(text, "weekly")
	session := window == "5h" || window == "daily" ||
		strings.Contains(text, "five hour") || strings.Contains(text, "5h") || strings.Contains(text, "daily")
	return weekly, session
}

func antigravityGroupGemini(lowered string) bool {
	return strings.Contains(lowered, "gemini")
}

func antigravityGroupClaudeGPT(lowered string) bool {
	return strings.Contains(lowered, "claude") || strings.Contains(lowered, "gpt")
}
