// The Antigravity family: v1internal:fetchAvailableModels for the per-model windows, behind a
// loadCodeAssist lookup that supplies both the project and the plan, with the weekly buckets
// of antigravity_weekly.go overlaid on top.
//
// @file      internal/service/quotafetch/google_quota.go
// @for       Runs Antigravity's reads and assembles the windows the card is handed.
// @uses      internal/service/quotafetch, context, fmt, net/http, strings, time, antigravity_models.go
// @reason    The IDE meters a paid tier per model and a free tier not at all, and answers the project, the tier and the windows from three different surfaces, so the read that shows the card the truth has to keep a tier-less account from being shown model rows and mark a session window spent when the whole family behind it is.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
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
	// The antigravity entry's built-in hosts, declared here while endpoints.go stays
	// untouched. The integrator promotes them under "antigravity"; the quota-summary host
	// sits with its own reader in antigravity_weekly.go.
	antigravityFetchModelsURL = "https://daily-cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels"
	antigravityLoadProjectURL = "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist"

	antigravityDisplay    = "Antigravity"
	antigravityNoTierPlan = "Unknown"

	antigravityForbiddenMsg = "Antigravity quota API access forbidden. Chat may still work."
	antigravityExpiredMsg   = "Antigravity quota API authentication expired. Chat may still work."
	antigravityNoTokenMsg   = "Antigravity access token not available. Re-authorize the connection to view usage."
	antigravityNoRowsMsg    = "Antigravity published no quota windows for this account."
)

// fetchAntigravity is the "antigravity" entry.
func fetchAntigravity(ctx context.Context, creds Credentials) Result {
	token := strings.TrimSpace(creds.AccessToken)
	if token == "" {
		return Result{Plan: antigravityNoTierPlan, Message: antigravityNoTokenMsg}
	}

	// One lookup supplies all three facts the rest of the read needs, and the reference
	// tolerates it failing: an account whose tier is unknown still has windows to show.
	account, _ := googleLoadAccount(ctx, creds, token, googleBootstrap{
		declared:  creds.Endpoints.LoadProjectAPIURL,
		builtIn:   antigravityLoadProjectURL,
		mode:      true,
		userAgent: antigravityUserAgent,
	})
	plan := antigravityPlan(account)
	project := googleAccountProject(account.Project)

	endpoint := endpointFor(declaredOr(creds.Endpoints.QuotaAPIURL, antigravityFetchModelsURL), creds.Endpoint)
	response, err := requestUsage(ctx, http.MethodPost, endpoint,
		antigravityHeaders(token, creds.UsageHeaders), googleProjectBody(project))
	if err != nil {
		return Result{Plan: plan, Message: fmt.Sprintf("%s error: %s", antigravityDisplay, err)}
	}
	// Both refusals say something the shared sentence cannot: chat with this token still
	// works, so the operator must not be told the account is dead.
	if response.status == http.StatusForbidden {
		return Result{Plan: plan, Message: antigravityForbiddenMsg}
	}
	if response.status == http.StatusUnauthorized {
		return Result{Plan: plan, Message: antigravityExpiredMsg}
	}
	if failure, refused := response.hardFailure(antigravityDisplay); refused {
		failure.Plan = plan
		return failure
	}

	var answer antigravityModelAnswer
	if err := json.Unmarshal(response.body, &answer); err != nil {
		return Result{Plan: plan, Message: fmt.Sprintf("%s error: %s", antigravityDisplay, err)}
	}

	models := antigravityModelRows(answer.Models, antigravityFreeTier(account))
	weekly := antigravityReconcile(models, fetchAntigravityWeekly(ctx, creds, token, project))
	rows := antigravityRows(models, weekly)
	if len(rows) == 0 {
		return Result{Plan: plan, Message: antigravityNoRowsMsg}
	}
	return Result{Plan: plan, Quotas: rows}
}

// antigravityFreeTier reads the tier the lookup published. A free account has no separate
// 5-hour window at all, and the per-model fractions it answers are the weekly limit read out
// of place, so the reference skips them and the card shows the weekly rows only.
func antigravityFreeTier(account googleAccount) bool {
	id := strings.TrimSpace(account.PaidTier.ID)
	return id == "" || id == antigravityFreeTierID
}

// antigravityPlan names the plan the lookup published. An account the lookup could not read
// still has a card to title, and "Unknown" is the reference's own word for it.
func antigravityPlan(account googleAccount) string {
	if tier := strings.TrimSpace(account.CurrentTier.Name); tier != "" {
		return tier
	}
	return antigravityNoTierPlan
}

// antigravityReconcile marks a session row spent when every model of its family is exhausted
// while the session window still reports room. The provider leaves the 5-hour row untouched in
// that state, and a card showing room there while nothing can be run would be the wrong fact.
func antigravityReconcile(models []antigravityModelWindow, weekly []Quota) []Quota {
	if len(models) == 0 || len(weekly) == 0 {
		return weekly
	}
	antigravityExhaustSession(weekly, models, antigravityGeminiFamily, antigravitySessionGemini)
	antigravityExhaustSession(weekly, models, antigravityClaudeFamily, antigravitySessionClaude)
	return weekly
}

func antigravityExhaustSession(weekly []Quota, models []antigravityModelWindow, family func(string) bool, label string) {
	exhausted, latest := antigravityFamilyState(models, family)
	if !exhausted {
		return
	}
	antigravityMarkSpent(weekly, label, latest)
}

// antigravityFamilyState reports whether every model of one family is spent, and the latest
// reset any of them states. A family the answer named no model of is not exhausted: it is
// absent, and absent models say nothing about the window they would have shared.
func antigravityFamilyState(models []antigravityModelWindow, family func(string) bool) (bool, time.Time) {
	seen := false
	var latest time.Time
	for _, model := range models {
		if !family(model.modelID) {
			continue
		}
		if model.Total <= 0 || model.Used < model.Total {
			return false, time.Time{}
		}
		seen = true
		if model.ResetAt.After(latest) {
			latest = model.ResetAt
		}
	}
	return seen, latest
}

// antigravityMarkSpent overwrites one named window as fully used. A row that already states no
// room left is left alone, and a row that is absent is not invented.
func antigravityMarkSpent(rows []Quota, label string, resetsAt time.Time) {
	for index := range rows {
		if rows[index].Label != label {
			continue
		}
		if rows[index].Total > 0 && rows[index].Used >= rows[index].Total {
			return
		}
		rows[index].Used = rows[index].Total
		if !resetsAt.IsZero() {
			rows[index].ResetAt = resetsAt
		}
		return
	}
}

// antigravityRows assembles the card: the per-model windows the provider listed first, then the
// session and weekly buckets overlaid on them, which is the order the reference's own merge
// produced.
func antigravityRows(models []antigravityModelWindow, weekly []Quota) []Quota {
	rows := make([]Quota, 0, len(models)+len(weekly))
	for _, model := range models {
		rows = append(rows, model.Quota)
	}
	return append(rows, weekly...)
}
