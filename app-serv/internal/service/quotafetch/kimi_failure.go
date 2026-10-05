// The Kimi family's refusal wording and value coercion.
//
// @file      internal/service/quotafetch/kimi_failure.go
// @for       Turning a Kimi error body into the sentence the card renders, and coercing the values its windows carry.
// @uses      internal/service/quotafetch, encoding/json, math, net/http, strconv, strings, time
// @reason    This surface refuses in the Connect-RPC shape, nesting the reason and the localized
//
//	sentence inside the first detail's own debug block, so reading a refusal
//	here is a different job from reading a quota window, and the reference
//	quotes the provider's own sentence, because that sentence is the only
//	part of a rejection an operator can act on.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package quotafetch

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// kimiFailure words the three refusals this surface gives: an expired session, an account
// with no usage entitlement, and anything else the API said, quoting the provider's own sentence
// rather than a bare status, because that sentence is what the operator can act on.
func kimiFailure(response usageResponse) Result {
	if response.status == http.StatusUnauthorized {
		return Result{Plan: kimiDisplay, Message: kimiExpiredMessage}
	}
	var body kimiErrorBody
	if json.Unmarshal(response.body, &body) != nil {
		body = kimiErrorBody{}
	}
	reason, localized := body.explain()
	// Two different uses of the same bytes: `raw` is searched for the permission markers,
	// which a provider may state inside an HTML error page, while `message` is what the
	// operator is shown, and markup never belongs there.
	raw := strings.TrimSpace(string(response.body))
	message := kimiFirst(localized, providerDetail(response.body))
	if response.status == http.StatusForbidden && (reason == kimiReasonNoPermission ||
		kimiPermissionPattern.MatchString(body.Code+" "+raw)) {
		return Result{Plan: kimiDisplay, Message: kimiFirst(localized, kimiNoPermission)}
	}
	if message == "" {
		return Result{Plan: kimiDisplay, Message: fmt.Sprintf("%s. API Error %d", kimiDisplay, response.status)}
	}
	if runes := []rune(message); len(runes) > 100 {
		message = string(runes[:100])
	}
	return Result{Plan: kimiDisplay, Message: fmt.Sprintf("%s. API Error %d: %s", kimiDisplay, response.status, message)}
}

// kimiErrorBody is a refusal in the Connect-RPC shape the endpoint answers with, nesting the same
// reason and sentence inside the first detail and its own debug block.
type kimiErrorBody struct {
	Code    string           `json:"code"`
	Message string           `json:"message"`
	Reason  string           `json:"reason"`
	Debug   kimiErrorBlock   `json:"debug"`
	Details []kimiErrorBlock `json:"details"`
}

type kimiErrorBlock struct {
	Reason    string          `json:"reason"`
	Debug     *kimiErrorBlock `json:"debug"`
	Localized kimiLocalized   `json:"localizedMessage"`
}

type kimiLocalized struct {
	Text string `json:"message"`
}

// explain reads them in the reference's order: the first detail's own debug block when one was
// sent, the top-level block otherwise, then the fields beside them.
func (b kimiErrorBody) explain() (string, string) {
	block, detail := b.Debug, kimiErrorBlock{}
	if len(b.Details) > 0 {
		detail = b.Details[0]
		if detail.Debug != nil {
			block = *detail.Debug
		}
	}
	return kimiFirst(block.Reason, detail.Reason, b.Reason),
		kimiFirst(block.Localized.Text, detail.Localized.Text, b.Message)
}

// kimiPlan titles the card with the tier the provider names, lowercasing a code this table has not seen
// and keeping the family's own word when the code carries no name at all.
func kimiPlan(level string) string {
	if known, ok := kimiPlanLevels[level]; ok {
		return known
	}
	if trimmed := strings.TrimSpace(level); trimmed != "" {
		if named := strings.ToLower(strings.TrimPrefix(trimmed, "LEVEL_")); named != "" {
			return named
		}
	}
	return kimiDisplay
}

func kimiScalar(raw json.RawMessage) (float64, bool) {
	value, err := strconv.ParseFloat(strings.Trim(strings.TrimSpace(string(raw)), `"`), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}

func kimiReset(window kimiWindow) time.Time {
	for _, raw := range []json.RawMessage{window.ResetTime, window.ResetAt, window.ResetAlt} {
		if instant := parseReset(raw); !instant.IsZero() {
			return instant
		}
	}
	return time.Time{}
}

func kimiFirst(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
