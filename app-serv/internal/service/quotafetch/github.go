// The GitHub Copilot family: the copilot_internal/user endpoint, keyed by a GitHub OAuth
// token presented with the `token` scheme (never `bearer`), reporting two response shapes —
// a paid plan's quota snapshots and a free plan's monthly quotas.
//
// @file      internal/service/quotafetch/github.go
// @for       Reads GitHub Copilot's published quota off the copilot_internal/user endpoint.
// @uses      internal/service/quotafetch, encoding/json, net/http, time
// @reason    GitHub meters Copilot against two different payload shapes and authenticates
//
//	with a `token` scheme plus the API-version and editor identification the endpoint
//	demands, and unlike the other families a failure here is an error the card shows
//	rather than a benign note — so its messages keep an error flavour.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-02
package quotafetch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	// githubUsageURL is the built-in usage endpoint, declared here while endpoints.go
	// stays untouched. The integrator promotes it into familyEndpoints under "github".
	githubUsageURL = "https://api.github.com/copilot_internal/user"
	githubDisplay  = "GitHub"

	githubAPIVersion          = "2022-11-28"
	githubEditorVersion       = "vscode/1.100.0"
	githubEditorPluginVersion = "copilot-chat/0.26.7"
	githubUserAgent           = "GitHubCopilotChat/0.26.7"
)

// fetchGitHub is the "github" entry.
func fetchGitHub(ctx context.Context, creds Credentials) Result {
	endpoint := endpointFor(declaredOr(creds.Endpoints.URL, githubUsageURL), creds.Endpoint)

	token := strings.TrimSpace(creds.AccessToken)
	if token == "" {
		token = strings.TrimSpace(creds.APIKey)
	}
	if token == "" {
		return Result{Message: "GitHub quota error: no access token available. Re-authorize the connection."}
	}

	response, err := requestUsage(ctx, http.MethodGet, endpoint, githubHeaders(token, creds.UsageHeaders), "")
	if err != nil {
		return Result{Message: fmt.Sprintf("GitHub quota error: %s", err)}
	}
	if failure, refused := response.hardFailure(githubDisplay); refused {
		return failure
	}

	var data githubPayload
	if err := json.Unmarshal(response.body, &data); err != nil {
		return Result{Message: fmt.Sprintf("GitHub quota error: %s", err)}
	}
	return githubResult(data)
}

// githubHeaders presents the OAuth token with the `token` scheme the endpoint requires and
// adds the identification it rejects a call without. A declared header may override the
// version or editor strings, but never the Authorization scheme, which is this family's own.
func githubHeaders(token string, extra map[string]string) map[string]string {
	headers := map[string]string{
		"Authorization":         "token " + token,
		"X-GitHub-Api-Version":  githubAPIVersion,
		"Editor-Version":        githubEditorVersion,
		"Editor-Plugin-Version": githubEditorPluginVersion,
		"User-Agent":            githubUserAgent,
	}
	for key, value := range extra {
		if key == "Authorization" || strings.TrimSpace(value) == "" {
			continue
		}
		headers[key] = value
	}
	return headers
}

// githubPayload carries both the paid (`quota_snapshots`) and free (`monthly_quotas` /
// `limited_user_quotas`) shapes. The snapshot and quota blocks are pointers so an absent
// block stays absent rather than collapsing into a zero row.
type githubPayload struct {
	CopilotPlan          string           `json:"copilot_plan"`
	AccessTypeSKU        string           `json:"access_type_sku"`
	QuotaResetDate       json.RawMessage  `json:"quota_reset_date"`
	LimitedUserResetDate json.RawMessage  `json:"limited_user_reset_date"`
	QuotaSnapshots       *githubSnapshots `json:"quota_snapshots"`
	MonthlyQuotas        *githubLimited   `json:"monthly_quotas"`
	LimitedUserQuotas    *githubLimited   `json:"limited_user_quotas"`
}

type githubSnapshots struct {
	Chat                *githubSnapshot `json:"chat"`
	Completions         *githubSnapshot `json:"completions"`
	PremiumInteractions *githubSnapshot `json:"premium_interactions"`
}

type githubSnapshot struct {
	Entitlement float64 `json:"entitlement"`
	Remaining   float64 `json:"remaining"`
	Unlimited   bool    `json:"unlimited"`
}

type githubLimited struct {
	Chat        json.Number `json:"chat"`
	Completions json.Number `json:"completions"`
}

// githubResult renders whichever shape GitHub answered. Snapshots come first, matching the
// reference's branch order; a payload with neither block is a connection we could not parse,
// which stays a soft note rather than an error.
func githubResult(data githubPayload) Result {
	if data.QuotaSnapshots != nil {
		resetAt := parseReset(data.QuotaResetDate)
		return Result{
			Plan: data.CopilotPlan,
			Quotas: []Quota{
				githubRow("chat", data.QuotaSnapshots.Chat, resetAt),
				githubRow("completions", data.QuotaSnapshots.Completions, resetAt),
				githubRow("premium_interactions", data.QuotaSnapshots.PremiumInteractions, resetAt),
			},
		}
	}

	if data.MonthlyQuotas != nil || data.LimitedUserQuotas != nil {
		resetAt := parseReset(data.LimitedUserResetDate)
		plan := data.CopilotPlan
		if plan == "" {
			plan = data.AccessTypeSKU
		}
		return Result{
			Plan: plan,
			Quotas: []Quota{
				githubFreeRow("chat", data.LimitedUserQuotas, data.MonthlyQuotas, resetAt),
				githubFreeRow("completions", data.LimitedUserQuotas, data.MonthlyQuotas, resetAt),
			},
		}
	}

	return Result{Message: "GitHub Copilot connected. Unable to parse quota data."}
}

// githubRow maps one paid snapshot: used is entitlement minus remaining, the reference's
// rule, deliberately unclamped to stay faithful to the source. A missing snapshot is an
// unlimited window, not a zero one.
func githubRow(label string, snapshot *githubSnapshot, resetAt time.Time) Quota {
	if snapshot == nil {
		return Quota{Label: label, Unlimited: true, Recurring: true, ResetAt: resetAt}
	}
	return Quota{
		Label:     label,
		Used:      snapshot.Entitlement - snapshot.Remaining,
		Total:     snapshot.Entitlement,
		Unlimited: snapshot.Unlimited,
		Recurring: true,
		ResetAt:   resetAt,
	}
}

// githubFreeRow maps one free-plan window: the used counter comes straight from
// limited_user_quotas and the ceiling from monthly_quotas, both defaulting to zero.
func githubFreeRow(label string, usedQuotas, monthlyQuotas *githubLimited, resetAt time.Time) Quota {
	return Quota{
		Label:     label,
		Used:      githubField(usedQuotas, label),
		Total:     githubField(monthlyQuotas, label),
		Recurring: true,
		ResetAt:   resetAt,
	}
}

func githubField(block *githubLimited, label string) float64 {
	if block == nil {
		return 0
	}
	switch label {
	case "chat":
		return num("", block.Chat)
	case "completions":
		return num("", block.Completions)
	default:
		return 0
	}
}
