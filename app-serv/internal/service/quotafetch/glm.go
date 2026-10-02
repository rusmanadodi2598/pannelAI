// GLM Coding Plan: the interval limits the provider publishes for one API key.
//
// @file      internal/service/quotafetch/glm.go
// @for       Reads GLM's published session and weekly limits and the plan tier they belong to.
// @uses      internal/service/quotafetch, context, encoding/json, net/http
// @reason    GLM meters a coding plan in percentages against intervals it names itself — an
//
//	N-hour session as well as a week — so the rows have to be read from the
//	provider's interval codes rather than guessed at from a fixed window here.
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
	"math"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

// glmDisplayName is the word every GLM message and soft answer is titled with; a published
// plan tier replaces it only when the provider states one.
const glmDisplayName = "GLM"

// glmQuotaURL is the international coding plan's quota host. The glm registry entry declares
// the same value under transport.usage.url, which wins; this is the fallback for a caller
// that handed over no entry. The China region is reached by declaring its own
// monitor/usage/quota/limit host in the registry — the app has no glm-cn entry to build a
// second built-in for.
const glmQuotaURL = "https://api.z.ai/api/monitor/usage/quota/limit"

// glmPercentWindow is what a published limit is a share of: GLM states usage as a
// percentage of the allocation, never as an absolute token count.
const glmPercentWindow = 100

// The provider's own interval codes and limit types, named rather than left inline because
// a row's label is chosen from all four.
const (
	glmUnitSession = "3"
	glmUnitWeekly  = "6"

	glmTypeTokens = "TOKENS_LIMIT"
	glmTypeCredit = "CREDIT_LIMIT"
)

// glmAnswer unwraps the published block. A response carrying no `data` object decodes to a
// zero value, which is the same answer the reference gives it: no limits.
type glmAnswer struct {
	Data glmData `json:"data"`
}

type glmData struct {
	Limits []glmLimit `json:"limits"`
	Level  string     `json:"level"`
}

// glmLimit is one published interval. The scalars that name it stay raw because GLM sends
// the same field as a JSON number on one plan and a quoted string on another.
type glmLimit struct {
	Type          string          `json:"type"`
	Unit          json.RawMessage `json:"unit"`
	Number        json.RawMessage `json:"number"`
	Percentage    json.RawMessage `json:"percentage"`
	NextResetTime json.RawMessage `json:"nextResetTime"`
}

// fetchGlm reads the coding plan's published limits with the key the account stores.
func fetchGlm(ctx context.Context, creds Credentials) Result {
	apiKey := strings.TrimSpace(creds.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(creds.AccessToken)
	}
	if apiKey == "" {
		return Result{Plan: glmDisplayName, Message: glmDisplayName + " API key not available."}
	}

	endpoint := endpointFor(declaredOr(creds.Endpoints.URL, glmQuotaURL), creds.Endpoint)
	response, err := requestUsage(ctx, http.MethodGet, endpoint, bearer(apiKey, creds.UsageHeaders), "")
	if err != nil {
		return Result{Plan: glmDisplayName, Message: fmt.Sprintf("%s error: %s", glmDisplayName, err)}
	}
	if failure, soft := response.softFailure(glmDisplayName); soft {
		failure.Plan = glmDisplayName
		return failure
	}

	var answer glmAnswer
	if err := json.Unmarshal(response.body, &answer); err != nil {
		return Result{Plan: glmDisplayName, Message: fmt.Sprintf("%s error: %s", glmDisplayName, err)}
	}
	return glmResult(answer.Data)
}

// glmResult renders every published limit, in the order the provider listed them. A plan
// that publishes nothing says so: the card would otherwise be blank where the honest fact
// is that the provider answered and named no allocation.
func glmResult(data glmData) Result {
	result := Result{Plan: glmPlan(data.Level)}
	for _, limit := range data.Limits {
		quota, ok := glmWindow(limit)
		if !ok {
			continue
		}
		result.Quotas = append(result.Quotas, quota)
	}
	if len(result.Quotas) == 0 {
		result.Message = "GLM published no quota for this account."
	}
	return result
}

// glmWindow reads one published limit as a percent window. A limit that carries no readable
// percentage is still a limit the provider published, and its reset date is the fact worth
// showing, so it arrives as 0% used rather than being dropped.
func glmWindow(limit glmLimit) (Quota, bool) {
	if !glmPublished(limit.Type) {
		return Quota{}, false
	}
	return Quota{
		Label:     glmLabel(limit),
		Used:      glmPercent(limit.Percentage),
		Total:     glmPercentWindow,
		Unit:      "%",
		ResetAt:   parseReset(limit.NextResetTime),
		Recurring: true,
	}, true
}

// glmPublished keeps the two limit types the provider meters coding plans with. Any other
// type is a meter this card does not claim to read, and rendering it as a coding quota
// would invent a window the provider never published for this plan.
func glmPublished(kind string) bool {
	return kind == glmTypeTokens || kind == glmTypeCredit
}

// glmLabel names the interval from the provider's own codes: an N-hour session keeps the
// number it states, the weekly window is the week it always is, and a limit outside both
// falls back to its type so two rows never arrive under one meaningless name.
func glmLabel(limit glmLimit) string {
	unit := glmText(limit.Unit)
	number := glmText(limit.Number)
	switch {
	case unit == glmUnitSession:
		return fmt.Sprintf("Session (%sh)", number)
	case unit == glmUnitWeekly:
		return "Weekly (7d)"
	case limit.Type == glmTypeTokens:
		return "Tokens"
	default:
		return fmt.Sprintf("Limit (%s)", number)
	}
}

// glmPercent reads the published share, sent as a number or a numeric string. An unreadable
// value counts as no use yet, the reference's reading, because the limit exists regardless.
func glmPercent(raw json.RawMessage) float64 {
	value, err := strconv.ParseFloat(glmText(raw), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}

// glmText reads a small scalar either way the provider spells it, unquoting a string so a
// quoted interval code names its row the same way a bare number does.
func glmText(raw json.RawMessage) string {
	return strings.Trim(strings.TrimSpace(string(raw)), `"`)
}

// glmPlan titles the card with the tier the provider names, capitalising its first letter
// the way the reference does, and answers "Unknown" when the published block states no tier.
func glmPlan(level string) string {
	trimmed := strings.TrimSpace(level)
	if trimmed == "" {
		return "Unknown"
	}
	first, width := utf8.DecodeRuneInString(trimmed)
	return strings.ToUpper(string(first)) + strings.ToLower(trimmed[width:])
}
