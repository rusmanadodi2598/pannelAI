// The OpenCode products (Zen and Go): one published percent document, two hosts.
//
// @file      internal/service/quotafetch/opencode.go
// @for       Reads the rolling, weekly and monthly windows OpenCode publishes for one of its two products.
// @uses      internal/service/quotafetch, context, encoding/json, net/http
// @reason    Zen (pay-as-you-go) and Go (subscription) answer the same quota document from
//
//	different hosts with different words on the card, so a read that asked one
//	host for both reports a working connection against the wrong allocation.
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
	"strconv"
	"strings"
)

// The two products' own usage hosts. Each registry entry declares its one under
// transport.usage.url; these are the fallback for a caller that handed over no entry, and
// they are the values to promote into familyEndpoints once that file is free to edit.
const (
	openCodeZenURL = "https://opencode.ai/zen/v1/usage"
	openCodeGoURL  = "https://opencode.ai/zen/go/v1/usage"
)

const (
	// openCodePercentWindow is the ceiling OpenCode states its windows against: a published
	// percent of one full allocation, so every row is a share rather than an absolute counter.
	openCodePercentWindow = 100

	// openCodeEntitlementError is the error type the endpoint answers with when the key is
	// valid but carries no paid product, which is a different fix than a refused key.
	openCodeEntitlementError = "EntitlementError"
)

// openCodeFamily is one OpenCode product: the same document, its own host and its own
// wording. `entitlement` holds the noun the product uses for what the key is missing,
// because the two products name it differently on their own cards.
type openCodeFamily struct {
	display     string
	builtIn     string
	entitlement string
}

var (
	openCodeZen = openCodeFamily{display: "OpenCode Zen", builtIn: openCodeZenURL, entitlement: "billing"}
	openCodeGo  = openCodeFamily{display: "OpenCode Go", builtIn: openCodeGoURL, entitlement: "subscription"}
)

// openCodeAnswer is the published document. `usage` is a pointer because the reference
// separates a response carrying no quota object from one whose periods hold no usable
// percent, and the card says something different about each.
type openCodeAnswer struct {
	Usage *openCodeUsage `json:"usage"`
}

type openCodeUsage struct {
	Rolling json.RawMessage `json:"rolling"`
	Weekly  json.RawMessage `json:"weekly"`
	Monthly json.RawMessage `json:"monthly"`
}

type openCodePeriod struct {
	Percent  json.RawMessage `json:"percent"`
	ResetsAt json.RawMessage `json:"resetsAt"`
}

// fetchOpenCode binds the shared reader to one OpenCode product.
func fetchOpenCode(family openCodeFamily) func(context.Context, Credentials) Result {
	return func(ctx context.Context, creds Credentials) Result {
		apiKey := strings.TrimSpace(creds.APIKey)
		if apiKey == "" {
			apiKey = strings.TrimSpace(creds.AccessToken)
		}
		if apiKey == "" {
			return Result{Plan: family.display,
				Message: fmt.Sprintf("%s API key not available. Add a key to view usage.", family.display)}
		}

		endpoint := endpointFor(declaredOr(creds.Endpoints.URL, family.builtIn), creds.Endpoint)
		response, err := requestUsage(ctx, http.MethodGet, endpoint, bearer(apiKey, creds.UsageHeaders), "")
		if err != nil {
			return Result{Plan: family.display, Message: fmt.Sprintf("%s error: %s", family.display, err)}
		}

		// A 403 states one fact the shared sentence cannot: OpenCode distinguishes a key with
		// no product behind it from a key it simply refuses, and the operator's fix differs.
		if response.status == http.StatusForbidden {
			return Result{Plan: family.display, Message: openCodeForbidden(response, family)}
		}
		if failure, soft := response.softFailure(family.display); soft {
			failure.Plan = family.display
			return failure
		}

		var answer openCodeAnswer
		if err := json.Unmarshal(response.body, &answer); err != nil {
			return Result{Plan: family.display,
				Message: fmt.Sprintf("%s usage response did not contain quota data.", family.display)}
		}
		// Two absences the reference keeps apart, because the operator's next move differs:
		// a document that carries no quota object at all, and one whose periods publish no
		// usable percent for this plan.
		if answer.Usage == nil {
			return Result{Plan: family.display,
				Message: fmt.Sprintf("%s usage response did not contain quota data.", family.display)}
		}
		quotas := openCodeQuotas(answer.Usage)
		if len(quotas) == 0 {
			return Result{Plan: family.display,
				Message: fmt.Sprintf("%s usage response did not contain valid quota data.", family.display)}
		}
		return Result{Plan: family.display, Quotas: quotas}
	}
}

// openCodeForbidden words the two refusals the endpoint can mean, reading the error type out
// of the body when the body states one and falling back to the plain refusal otherwise.
func openCodeForbidden(response usageResponse, family openCodeFamily) string {
	var body struct {
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.body, &body); err == nil && body.Error.Type == openCodeEntitlementError {
		return fmt.Sprintf("%s %s required for this API key.", family.display, family.entitlement)
	}
	return fmt.Sprintf("%s access forbidden for this API key.", family.display)
}

// openCodeQuotas reads the windows in the order the reference names them, skipping a period
// the provider did not publish or published without a usable percent: a row of 0% for an
// absent window would read as an untouched allocation rather than as no allocation.
func openCodeQuotas(usage *openCodeUsage) []Quota {
	if usage == nil {
		return nil
	}
	windows := []struct {
		label string
		raw   json.RawMessage
	}{
		{"Rolling", usage.Rolling},
		{"Weekly", usage.Weekly},
		{"Monthly", usage.Monthly},
	}
	quotas := make([]Quota, 0, len(windows))
	for _, window := range windows {
		if quota, ok := openCodeWindow(window.label, window.raw); ok {
			quotas = append(quotas, quota)
		}
	}
	return quotas
}

func openCodeWindow(label string, raw json.RawMessage) (Quota, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return Quota{}, false
	}
	var period openCodePeriod
	if err := json.Unmarshal(raw, &period); err != nil {
		return Quota{}, false
	}
	percent, ok := openCodePercent(period.Percent)
	if !ok {
		return Quota{}, false
	}
	return Quota{
		Label:     label,
		Used:      openCodeClamp(percent),
		Total:     openCodePercentWindow,
		Unit:      "%",
		ResetAt:   parseReset(period.ResetsAt),
		Recurring: true,
	}, true
}

// openCodePercent reads a published share, which OpenCode sends as a JSON number on some
// answers and as a numeric string on others. Anything else, absent, empty, unparseable,
// not finite, is no reading at all, which is why it reports ok instead of answering zero.
func openCodePercent(raw json.RawMessage) (float64, bool) {
	text := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	if text == "" || text == "null" {
		return 0, false
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}

// openCodeClamp holds a published share inside the window it states. A provider that answers
// 140% of a 100% ceiling has overspent rather than published a wider window, and a bar that
// renders 140 of 100 is not a fact the operator can act on.
func openCodeClamp(percent float64) float64 {
	if percent < 0 {
		return 0
	}
	if percent > openCodePercentWindow {
		return openCodePercentWindow
	}
	return percent
}
