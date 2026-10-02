// The Grok CLI / Grok Build family: two REST reads (billing + user) on one origin, whose
// windows are reported as percentages, with a gRPC-web weekly pool as the fallback read.
//
// @file      internal/service/quotafetch/grok.go
// @for       Reads Grok CLI's credit allotment from its billing and user endpoints.
// @uses      internal/service/quotafetch, encoding/json, math, net/http, net/url, strconv, time
// @reason    Grok meters a subscription as percentages the panel must render, never an absolute remaining.
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
	"net/url"
	"strings"
	"time"
)

const (
	grokBillingURL                                                         = "https://cli-chat-proxy.grok.com/v1/billing?format=credits"
	grokUserURL                                                            = "https://cli-chat-proxy.grok.com/v1/user?include=subscription"
	grokDisplay                                                            = "Grok CLI"
	grokLabelMonthly, grokLabelOnDemand, grokLabelPrepaid, grokLabelWeekly = "Monthly included", "On-demand", "Prepaid", "Weekly SuperGrok"
)

// fetchGrok is the "grok-cli" entry. The credential gate runs before any outbound call.
func fetchGrok(ctx context.Context, creds Credentials) Result {
	token := strings.TrimSpace(creds.AccessToken)
	if token == "" {
		return Result{Message: "Grok CLI access token not available."}
	}
	billing, user := grokEndpoints(creds)
	headers := grokHeaders(token, creds.ProviderSpecificData, creds.UsageHeaders)
	billingResp, err := requestUsage(ctx, http.MethodGet, billing, headers, "")
	if err != nil {
		return Result{Message: fmt.Sprintf("Grok CLI error: %s", err)}
	}
	if failure, refused := billingResp.softFailure(grokDisplay); refused {
		return failure
	}
	var userView grokUserView
	if response, uerr := requestUsage(ctx, http.MethodGet, user, headers, ""); uerr == nil && response.status >= 200 && response.status < 300 {
		_ = json.Unmarshal(response.body, &userView)
	}
	var payload grokBillingPayload
	if err := json.Unmarshal(billingResp.body, &payload); err != nil {
		return Result{Message: "Grok CLI billing response was not JSON."}
	}
	config := payload.effective()
	plan := grokPlan(userView, config)
	if rows := grokBillingRows(config, userView); len(rows) > 0 {
		return Result{Plan: plan, Quotas: rows}
	}
	// No REST allotment: a paid SuperGrok often reports 0 but exposes the shared weekly pool on
	// GetGrokCreditsConfig, so read the binary frame before giving up.
	row, fallback, ok := grokCreditsQuota(ctx, creds)
	if ok {
		return Result{Plan: plan, Quotas: []Quota{row}}
	}
	return Result{Plan: plan, Message: fallback.Message}
}

// grokEndpoints resolves both REST surfaces; with no declared user_url it rebuilds one from the billing origin, keeping both reads on one host.
func grokEndpoints(creds Credentials) (string, string) {
	billing := endpointFor(declaredOr(creds.Endpoints.URL, grokBillingURL), creds.Endpoint)
	if declared := strings.TrimSpace(creds.Endpoints.UserURL); declared != "" {
		return billing, endpointFor(declared, creds.Endpoint)
	}
	parsed, err := url.Parse(grokUserURL)
	origin := originOf(billing)
	if err != nil || origin == "" || parsed.Path == "" {
		return billing, endpointFor(grokUserURL, creds.Endpoint)
	}
	userURL := origin + parsed.Path
	if parsed.RawQuery != "" {
		userURL += "?" + parsed.RawQuery
	}
	return billing, endpointFor(userURL, creds.Endpoint)
}

func grokHeaders(token string, data map[string]string, extra map[string]string) map[string]string {
	headers := map[string]string{
		"Authorization":            "Bearer " + token,
		"User-Agent":               "grok-shell/0.2.99 (linux; x86_64)",
		"x-xai-token-auth":         "xai-grok-cli",
		"x-grok-client-identifier": "grok-shell",
		"x-grok-client-version":    "0.2.99",
		"x-grok-client-mode":       "headless",
	}
	for key, value := range extra {
		if key != "Authorization" && strings.TrimSpace(value) != "" {
			headers[key] = value
		}
	}
	if email := strings.TrimSpace(data["email"]); email != "" {
		headers["x-email"] = email
	}
	if id := grokFirstNonEmpty(data["userId"], data["principalId"]); id != "" {
		headers["x-userid"] = id
	}
	return headers
}

func grokFirstNonEmpty(a, b string) string {
	if trimmed := strings.TrimSpace(a); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(b)
}

// grokBillingRows maps billing to the four windows the panel names verbatim, never an absolute remaining.
func grokBillingRows(config grokBillingFields, user grokUserView) []Quota {
	rows := make([]Quota, 0, 4)
	periodEnd := grokPeriodEnd(config)
	if config.MonthlyLimit.ok && config.MonthlyLimit.value > 0 {
		used := config.IncludedUsed
		if !used.ok {
			used = config.TotalUsed
		}
		value := 0.0
		if used.ok && used.value > 0 {
			value = used.value
		}
		rows = append(rows, Quota{Label: grokLabelMonthly, Used: value, Total: config.MonthlyLimit.value, Recurring: true, ResetAt: periodEnd})
	}
	cap, onUsed := config.OnDemandCap, config.OnDemandUsed
	switch {
	case cap.ok && cap.value > 0:
		value := 0.0
		if onUsed.ok && onUsed.value > 0 {
			value = onUsed.value
		}
		rows = append(rows, Quota{Label: grokLabelOnDemand, Used: value, Total: cap.value, Recurring: true, ResetAt: periodEnd})
	case !grokSubscriptionAccess(user, config) && cap.ok && cap.value == 0 && onUsed.ok:
		rows = append(rows, Quota{Label: grokLabelOnDemand, Used: 1, Total: 1, Unit: "%", Recurring: true, ResetAt: periodEnd})
	}
	if prepaid := config.PrepaidBalance; prepaid.ok && prepaid.value > 0 {
		rows = append(rows, Quota{Label: grokLabelPrepaid, Used: 0, Total: prepaid.value})
	}
	if pct := config.CreditUsagePercent; pct.ok && pct.value >= 0 {
		rows = append(rows, Quota{Label: grokLabelWeekly, Used: grokClampPercent(pct.value), Total: 100, Unit: "%", Recurring: true, ResetAt: periodEnd})
	}
	return rows
}

func grokClampPercent(value float64) float64 { return min(100, max(0, value)) }

func grokPeriodEnd(config grokBillingFields) time.Time {
	candidates := [][]byte{config.BillingPeriodEnd, config.ResetAt}
	if config.CurrentPeriod != nil {
		candidates = append(candidates, config.CurrentPeriod.End)
	}
	for _, raw := range candidates {
		if reset := parseReset(raw); !reset.IsZero() {
			return reset
		}
	}
	return time.Time{}
}

func grokPlan(user grokUserView, config grokBillingFields) string {
	if tier := grokTier(user, config); tier != "" {
		return tier
	}
	if user.HasGrokCodeAccess {
		return "Grok Code"
	}
	return "Grok Build"
}

func grokTier(user grokUserView, config grokBillingFields) string {
	return grokFirstNonEmpty(user.SubscriptionTier, config.SubscriptionTier)
}

func grokSubscriptionAccess(user grokUserView, config grokBillingFields) bool {
	switch strings.ToLower(grokTier(user, config)) {
	case "", "free", "none", "null":
		return false
	default:
		return true
	}
}
