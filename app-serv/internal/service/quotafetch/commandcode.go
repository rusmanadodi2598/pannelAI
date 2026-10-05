// Command Code: billing credits beside the 5-hour and weekly windows the same host publishes.
//
// @file      internal/service/quotafetch/commandcode.go
// @for       Reads Command Code's plan cap, remaining credits and two rolling rate windows.
// @uses      internal/service/quotafetch, context, encoding/json, fmt, math, net/http, net/url, strconv, strings
// @reason    The provider splits this over three endpoints and states no ceiling on the credits
//
//	surface, so the cap has to come from the subscription and the wallet from the
//	billing call, or the card would draw a balance as a share of nothing.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package quotafetch

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	commandCodeDisplay = "Command Code"

	// The billing host the reference defaults to. The commandcode registry entry declares no
	// usage block, so this is the built-in to promote into familyEndpoints; a declared
	// transport.usage.url still wins over it.
	commandCodeAPIBase = "https://api.commandcode.ai"

	commandCodeWhoamiRoute       = "/alpha/whoami"
	commandCodeCreditsRoute      = "/alpha/billing/credits"
	commandCodeSubscriptionsPath = "/alpha/billing/subscriptions"

	commandCodeMissingKeyMessage = "Command Code API key not available. Add a key to view usage."
	commandCodeAuthMessage       = "Command Code authentication failed. Check the API key."
)

// The plan's public name and the credit ceiling it carries. The cap is what turns a remaining
// balance into a window with a share to draw; a plan absent from it is unbounded.
var commandCodePlanNames = map[string]string{
	"individual-go": "Go", "individual-goat": "GOAT", "individual-pro": "Pro",
	"individual-pro-v1": "Pro", "individual-provider": "Provider", "individual-max": "Max",
	"individual-ultra": "Ultra", "teams-pro": "Teams Pro",
}

var commandCodePlanCaps = map[string]float64{
	"individual-go": 10, "individual-goat": 70, "individual-pro": 30,
	"individual-pro-v1": 80, "individual-provider": 15, "individual-max": 150,
	"individual-ultra": 300, "teams-pro": 40,
}

type commandCodeWhoami struct {
	Org struct {
		ID json.RawMessage `json:"id"`
	} `json:"org"`
}

// commandCodeCredits is the billing answer: the three wallets that add up to what is left, plus
// the rolling windows. Counts stay raw because the host sends numbers on one plan and quoted
// numbers on another.
type commandCodeCredits struct {
	Credits struct {
		MonthlyCredits   json.RawMessage `json:"monthlyCredits"`
		PurchasedCredits json.RawMessage `json:"purchasedCredits"`
		FreeCredits      json.RawMessage `json:"freeCredits"`
	} `json:"credits"`
	WindowLimits struct {
		FiveHour *commandCodeWindow `json:"fiveHour"`
		Weekly   *commandCodeWindow `json:"weekly"`
	} `json:"windowLimits"`
}

type commandCodeWindow struct {
	Used    json.RawMessage `json:"used"`
	Cap     json.RawMessage `json:"cap"`
	ResetAt json.RawMessage `json:"resetAt"`
}

type commandCodeSubscription struct {
	Data struct {
		PlanID           string          `json:"planId"`
		CurrentPeriodEnd json.RawMessage `json:"currentPeriodEnd"`
	} `json:"data"`
}

// fetchCommandCode walks the three surfaces the reference walks, in that order. The last two are
// read in sequence rather than at once, because the package bounds the whole read with a single
// deadline and a serial walk keeps that bound honest.
func fetchCommandCode(ctx context.Context, creds Credentials) Result {
	apiKey := strings.TrimSpace(creds.APIKey)
	if apiKey == "" {
		return Result{Message: commandCodeMissingKeyMessage}
	}

	base, headers := commandCodeBase(creds), bearer(apiKey, creds.UsageHeaders)
	whoami, refusal, answered := commandCodeAsk(ctx, base+commandCodeWhoamiRoute+"?limits=1", headers, "usage")
	if !answered {
		return refusal
	}

	var who commandCodeWhoami
	_ = json.Unmarshal(whoami.body, &who) // an unreadable whoami simply carries no organization to scope by
	org := commandCodeOrgQuery(who.Org.ID)

	credits, refusal, answered := commandCodeAsk(ctx, base+commandCodeCreditsRoute+org, headers, "credits")
	if !answered {
		return refusal
	}
	subscriptions, refusal, answered := commandCodeAsk(ctx, base+commandCodeSubscriptionsPath+org, headers, "subscriptions")
	if !answered {
		return refusal
	}

	var wallet commandCodeCredits
	var plan commandCodeSubscription
	_ = json.Unmarshal(credits.body, &wallet)
	_ = json.Unmarshal(subscriptions.body, &plan)
	return commandCodeResult(wallet, plan)
}

// commandCodeBase resolves the billing host: the declared usage URL when the entry carries one,
// the built-in otherwise, and the test seam's host over both while keeping each route's own path.
func commandCodeBase(creds Credentials) string {
	if override := strings.TrimSpace(creds.Endpoint); override != "" {
		return override
	}
	return strings.TrimRight(declaredOr(creds.Endpoints.URL, commandCodeAPIBase), "/")
}

// commandCodeAsk sends one billing read and turns every refusal into the sentence the reference
// puts on the card: a dead key is named as one, and a surface that errored is named by route,
// because the operator's fix differs between the two.
func commandCodeAsk(ctx context.Context, endpoint string, headers map[string]string,
	surface string) (usageResponse, Result, bool) {
	response, err := requestUsage(ctx, http.MethodGet, endpoint, headers, "")
	if err != nil {
		return response, Result{Plan: commandCodeDisplay,
			Message: fmt.Sprintf("%s error: %s", commandCodeDisplay, err)}, false
	}
	if response.status == http.StatusUnauthorized || response.status == http.StatusForbidden {
		return response, Result{Plan: commandCodeDisplay, Message: commandCodeAuthMessage}, false
	}
	if response.status < 200 || response.status >= 300 {
		return response, Result{Plan: commandCodeDisplay,
			Message: fmt.Sprintf("%s %s API error (%d)", commandCodeDisplay, surface, response.status)}, false
	}
	return response, Result{}, true
}

// commandCodeResult renders the credits window first, then the rolling windows the host states.
func commandCodeResult(wallet commandCodeCredits, plan commandCodeSubscription) Result {
	planID := strings.TrimSpace(plan.Data.PlanID)
	ceiling := commandCodePlanCaps[planID]
	remaining := commandCodeNumber(wallet.Credits.MonthlyCredits) +
		commandCodeNumber(wallet.Credits.PurchasedCredits) + commandCodeNumber(wallet.Credits.FreeCredits)

	credits := Quota{Label: "Credits", Total: commandCodeClamp(remaining), Unlimited: ceiling <= 0,
		ResetAt: parseReset(plan.Data.CurrentPeriodEnd)}
	if ceiling > 0 {
		credits.Total = ceiling
		credits.Used = math.Max(0, ceiling-commandCodeClamp(remaining))
	}

	rows := make([]Quota, 0, 3)
	rows = append(rows, credits)
	if window, ok := commandCodeBalance("Session (5h)", wallet.WindowLimits.FiveHour); ok {
		rows = append(rows, window)
	}
	if window, ok := commandCodeBalance("Weekly", wallet.WindowLimits.Weekly); ok {
		rows = append(rows, window)
	}
	return Result{Plan: commandCodePlanName(planID), Quotas: rows}
}

// commandCodeBalance reads one rolling window. A window that states neither a ceiling nor a
// counter was not published, so it is dropped rather than drawn as a row of zeroes.
func commandCodeBalance(label string, window *commandCodeWindow) (Quota, bool) {
	if window == nil {
		return Quota{}, false
	}
	used := commandCodeClamp(commandCodeNumber(window.Used))
	total := commandCodeClamp(commandCodeNumber(window.Cap))
	if total <= 0 && used <= 0 {
		return Quota{}, false
	}
	return Quota{Label: label, Used: math.Min(used, total), Total: total,
		ResetAt: parseReset(window.ResetAt)}, true
}

// commandCodePlanName names the card after the published plan: the provider's own friendly name
// for a tier it recognises, the raw plan id for one this table has not seen.
func commandCodePlanName(planID string) string {
	if name, ok := commandCodePlanNames[planID]; ok {
		return name
	}
	if planID != "" {
		return planID
	}
	return commandCodeDisplay
}

// commandCodeOrgQuery scopes the billing reads to the organization whoami named, and asks without
// the parameter when the account stated none, the reference drops a null orgId rather than
// sending the word.
func commandCodeOrgQuery(orgID json.RawMessage) string {
	trimmed := strings.Trim(strings.TrimSpace(string(orgID)), `"`)
	if trimmed == "" || trimmed == "null" {
		return ""
	}
	return "?orgId=" + url.QueryEscape(trimmed)
}

func commandCodeNumber(raw json.RawMessage) float64 {
	value, err := strconv.ParseFloat(strings.Trim(strings.TrimSpace(string(raw)), `"`), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}

func commandCodeClamp(value float64) float64 {
	return math.Max(0, value)
}
