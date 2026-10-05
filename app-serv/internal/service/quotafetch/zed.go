// The Zed family: GET /client/users/me, authenticated with `<user_id> <access_token>`,
// deriving quota from the plan's usage buckets and its subscription period.
//
// @file      internal/service/quotafetch/zed.go
// @for       Derives Zed's published quota from the authenticated-user payload.
// @uses      internal/service/quotafetch, encoding/json, net/http, strconv, time
// @reason    Zed sends usage limits as polymorphic values ("unlimited", a number, or an object) and reports a zero model-request limit as token billing rather than a spent quota.
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
	"strconv"
	"strings"
	"time"
)

const (
	zedDisplay = "Zed"
	zedMeURL   = "https://cloud.zed.dev/client/users/me"

	zedTokenBillingNote = "Hosted AI models are billed per token (not request count). Edit Predictions are tracked below. Token spend is on dashboard.zed.dev."
	zedOverdueNote      = "This Zed account has overdue invoices. Usage may be blocked until billing is resolved."
)

// fetchZed is the "zed" entry; the credential gate runs before any outbound call.
func fetchZed(ctx context.Context, creds Credentials) Result {
	endpoint := endpointFor(declaredOr(creds.Endpoints.URL, zedMeURL), creds.Endpoint)
	token := strings.TrimSpace(creds.AccessToken)
	if token == "" {
		return Result{Message: "Zed access token not available. Re-connect Zed to view quota."}
	}
	userID := zedUserID(creds.ProviderSpecificData)
	if userID == "" {
		return Result{Message: "Zed credential is missing user id. Re-connect Zed to view quota."}
	}
	response, err := requestUsage(ctx, http.MethodGet, endpoint, zedHeaders(userID, token, creds.UsageHeaders), "")
	if err != nil {
		return Result{Message: fmt.Sprintf("Zed error: %s", err)}
	}
	if failure, refused := response.softFailure(zedDisplay); refused {
		return failure
	}
	var info zedResponse
	if err := json.Unmarshal(response.body, &info); err != nil {
		return Result{Message: fmt.Sprintf("Zed error: %s", err)}
	}
	return zedResult(info)
}

// zedUserID reads the account id paired with the token (`userId`, else `user_id`).
func zedUserID(data map[string]string) string {
	if id := strings.TrimSpace(data["userId"]); id != "" {
		return id
	}
	return strings.TrimSpace(data["user_id"])
}

// zedHeaders builds the `<user_id> <access_token>` Authorization value; declared extras are
// merged but never replace Authorization, which is this family's own scheme.
func zedHeaders(userID, token string, extra map[string]string) map[string]string {
	headers := make(map[string]string, len(extra)+1)
	for key, value := range extra {
		if key == "Authorization" || strings.TrimSpace(value) == "" {
			continue
		}
		headers[key] = value
	}
	headers["Authorization"] = userID + " " + token
	return headers
}

type zedResponse struct {
	Plan   zedPlan `json:"plan"`
	PlanV3 string  `json:"plan_v3"`
}

type zedPlan struct {
	PlanV3             string          `json:"plan_v3"`
	PlanV2             string          `json:"plan_v2"`
	Plan               string          `json:"plan"`
	Usage              zedUsage        `json:"usage"`
	SubscriptionPeriod zedPeriod       `json:"subscription_period"`
	TrialStartedAt     json.RawMessage `json:"trial_started_at"`
	HasOverdueInvoices bool            `json:"has_overdue_invoices"`
}

type zedUsage struct {
	EditPredictions *zedBucket `json:"edit_predictions"`
	ModelRequests   *zedBucket `json:"model_requests"`
}

// zedBucket keeps its limit as raw json: Zed sends "unlimited", a number, or an object.
type zedBucket struct {
	Used  json.Number     `json:"used"`
	Limit json.RawMessage `json:"limit"`
}

type zedPeriod struct {
	EndedAt json.RawMessage `json:"ended_at"`
}

// zedResult renders the plan label and the two usage windows: an absent bucket is not
// reported, a zero model-request limit is token billing rather than a row of zeros, and an
// overdue invoice overrides the softer note.
func zedResult(info zedResponse) Result {
	plan := info.Plan
	planID := zedFirstNonEmpty(plan.PlanV3, plan.PlanV2, plan.Plan, info.PlanV3)
	resetAt := parseReset(plan.SubscriptionPeriod.EndedAt)
	quotas := make([]Quota, 0, 2)
	tokenBilling := false
	if bucket := plan.Usage.EditPredictions; bucket != nil {
		quotas = append(quotas, zedRow("Edit Predictions", num("", bucket.Used), bucket.Limit, resetAt))
	}
	if bucket := plan.Usage.ModelRequests; bucket != nil {
		unlimited, total := zedParseLimit(bucket.Limit)
		if unlimited || total > 0 {
			quotas = append(quotas, zedRow("Hosted Model Requests", num("", bucket.Used), bucket.Limit, resetAt))
		} else {
			tokenBilling = true
		}
	}
	label := zedPlanLabel(planID)
	if zedPresent(plan.TrialStartedAt) && !strings.Contains(strings.ToLower(label), "trial") {
		label += " (Trial active)"
	}
	message := ""
	if tokenBilling {
		message = zedTokenBillingNote
	}
	if plan.HasOverdueInvoices {
		message = zedOverdueNote
	}
	return Result{Plan: label, Quotas: quotas, Message: message}
}

// zedRow clamps a used count to its ceiling and marks an unlimited window with no total.
func zedRow(label string, used float64, limitRaw json.RawMessage, resetAt time.Time) Quota {
	if used < 0 {
		used = 0
	}
	unlimited, total := zedParseLimit(limitRaw)
	if unlimited {
		return Quota{Label: label, Used: used, Total: 0, Unlimited: true, Recurring: true, ResetAt: resetAt}
	}
	if total <= 0 {
		return Quota{Label: label, Used: used, Total: 0, Recurring: true, ResetAt: resetAt}
	}
	if used > total {
		used = total
	}
	return Quota{Label: label, Used: used, Total: total, Recurring: true, ResetAt: resetAt}
}

// zedParseLimit coerces a Zed limit into (unlimited, total): a number or numeric string is a
// ceiling, the "unlimited" sentinel (bare or an object flag) has none, an absent limit is 0.
func zedParseLimit(raw json.RawMessage) (bool, float64) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return false, 0
	}
	if value, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return false, zedClamp(value)
	}
	if strings.HasPrefix(trimmed, `"`) {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return false, 0
		}
		text = strings.TrimSpace(text)
		if strings.EqualFold(text, "unlimited") {
			return true, 0
		}
		if value, err := strconv.ParseFloat(text, 64); err == nil {
			return false, zedClamp(value)
		}
		return false, 0
	}
	var object struct {
		Unlimited bool     `json:"unlimited"`
		Limited   *float64 `json:"limited"`
		LimitedV  *float64 `json:"Limited"`
	}
	if err := json.Unmarshal(raw, &object); err == nil {
		if object.Unlimited {
			return true, 0
		}
		if object.Limited != nil {
			return false, zedClamp(*object.Limited)
		}
		if object.LimitedV != nil {
			return false, zedClamp(*object.LimitedV)
		}
	}
	return false, 0
}

func zedClamp(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}

// zedPlanLabel maps a plan id to its dashboard label, title-casing any id it does not know.
func zedPlanLabel(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return zedDisplay
	}
	switch strings.ToLower(trimmed) {
	case "zed_free":
		return "Zed Free"
	case "zed_pro":
		return "Zed Pro"
	case "zed_pro_trial":
		return "Zed Pro Trial"
	case "zed_student":
		return "Zed Student"
	case "zed_business":
		return "Zed Business"
	}
	words := strings.Fields(strings.ReplaceAll(trimmed, "_", " "))
	for index, word := range words {
		lowered := strings.ToLower(word)
		words[index] = strings.ToUpper(lowered[:1]) + lowered[1:]
	}
	return strings.Join(words, " ")
}

func zedPresent(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null"
}

func zedFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
