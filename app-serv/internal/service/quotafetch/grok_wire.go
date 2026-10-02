// The Grok billing wire shapes: the protobuf-json leaf that arrives as a number, a string, or a
// `{ val: ... }` wrapper, and the payload the two reads return.
//
// @file      internal/service/quotafetch/grok_wire.go
// @for       Decoding Grok's billing and user payloads into typed values.
// @uses      encoding/json, math, strconv, strings.
// @reason    Grok spells one numeric field three ways depending on which of its services answered,
//
//	and a plain float decode fails on two of the three. Keeping that shape in one file leaves the
//	read itself free to talk about periods and tiers.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-02
package quotafetch

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

// grokVal is a protobuf-json numeric leaf (number, string, or { val: ... }); absent reads as absent, not zero.
type grokVal struct {
	value float64
	ok    bool
}

func (v *grokVal) UnmarshalJSON(raw []byte) error {
	text := strings.TrimSpace(string(raw))
	switch {
	case text == "" || text == "null":
		*v = grokVal{}
	case text[0] == '{':
		var wrap struct {
			Val grokVal `json:"val"`
		}
		// A body that is not the `{val:…}` wrapper is a value in another shape, not a
		// decode failure: leaving `ok` false is how the caller learns it carried nothing.
		if err := json.Unmarshal(raw, &wrap); err == nil {
			*v = wrap.Val
		}
	default:
		value, err := strconv.ParseFloat(strings.Trim(text, `"`), 64)
		*v = grokVal{value: value, ok: err == nil && !math.IsNaN(value) && !math.IsInf(value, 0)}
	}
	return nil
}

// grokBillingFields carries every billing value the rows and plan need, decoded at root or config.
type grokBillingFields struct {
	MonthlyLimit       grokVal         `json:"monthlyLimit"`
	IncludedUsed       grokVal         `json:"includedUsed"`
	TotalUsed          grokVal         `json:"totalUsed"`
	OnDemandCap        grokVal         `json:"onDemandCap"`
	OnDemandUsed       grokVal         `json:"onDemandUsed"`
	PrepaidBalance     grokVal         `json:"prepaidBalance"`
	CreditUsagePercent grokVal         `json:"creditUsagePercent"`
	CurrentPeriod      *grokPeriod     `json:"currentPeriod"`
	BillingPeriodEnd   json.RawMessage `json:"billingPeriodEnd"`
	ResetAt            json.RawMessage `json:"resetAt"`
	SubscriptionTier   string          `json:"subscriptionTier"`
}

type grokPeriod struct{ End json.RawMessage }

type grokBillingPayload struct {
	grokBillingFields
	Config *grokBillingFields `json:"config"`
}

func (p grokBillingPayload) effective() grokBillingFields {
	if p.Config != nil {
		return *p.Config
	}
	return p.grokBillingFields
}

type grokUserView struct {
	SubscriptionTier  string `json:"subscriptionTier"`
	HasGrokCodeAccess bool   `json:"hasGrokCodeAccess"`
}
