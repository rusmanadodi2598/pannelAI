// The CodeBuddy billing payload's shape, and how its credit packages read as windows.
//
// @file      internal/service/quotafetch/codebuddy_packs.go
// @for       Decodes one CodeBuddy billing answer and turns its credit packages into quota windows.
// @uses      encoding/json, sort, time
// @reason    A refill pack and a bonus pack look alike on the wire but must not be merged, so the
//
//	rule that separates them belongs with the fields it reads rather than with the request
//	that fetches them.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-29
package quotafetch

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// refillGap is the cycle-to-validity gap that separates a refill pack (whose allowance
// rolls over long before the resource expires) from a one-shot bonus pack (whose cycle
// ends exactly at expiry). The reference's constant: two days in milliseconds.
const refillGap = 2 * 24 * time.Hour

// codebuddyEnvelope unwraps the billing payload's doubled envelope
// (data.Response.Data.Accounts[]) and the per-package credit fields it carries.
type codebuddyEnvelope struct {
	Response struct {
		Data struct {
			Accounts []codebuddyAccount `json:"Accounts"`
		} `json:"Data"`
	} `json:"Response"`
}

type codebuddyAccount struct {
	PackageName              string          `json:"PackageName"`
	SubProductName           string          `json:"SubProductName"`
	CycleStartTime           json.RawMessage `json:"CycleStartTime"`
	CycleEndTime             json.RawMessage `json:"CycleEndTime"`
	DeductionEndTime         int64           `json:"DeductionEndTime"`
	CycleCapacityUsed        json.Number     `json:"CycleCapacityUsed"`
	CycleCapacityUsedPrecise string          `json:"CycleCapacityUsedPrecise"`
	CycleCapacitySize        json.Number     `json:"CycleCapacitySize"`
	CycleCapacitySizePrecise string          `json:"CycleCapacitySizePrecise"`
	CapacityUsed             json.Number     `json:"CapacityUsed"`
	CapacityUsedPrecise      string          `json:"CapacityUsedPrecise"`
	CapacitySize             json.Number     `json:"CapacitySize"`
	CapacitySizePrecise      string          `json:"CapacitySizePrecise"`
}

func codeBuddyResult(family codebuddyFamily, envelope codebuddyEnvelope) Result {
	accounts := envelope.Response.Data.Accounts
	if len(accounts) == 0 {
		return Result{Message: fmt.Sprintf("CodeBuddy %s connected. No credit package found.", family.name)}
	}

	var refills, bonuses []codebuddyAccount
	for _, account := range accounts {
		if isRefill(account) {
			refills = append(refills, account)
		} else {
			bonuses = append(bonuses, account)
		}
	}
	byExpiry := func(list []codebuddyAccount) {
		sort.Slice(list, func(i, j int) bool {
			return cycleEnd(list[i]).Before(cycleEnd(list[j]))
		})
	}
	byExpiry(refills)
	byExpiry(bonuses)

	result := Result{Plan: "CodeBuddy"}
	seenRefill := map[string]int{}
	for _, account := range refills {
		base := refillCadence(account)
		seenRefill[base]++
		label := base
		if seenRefill[base] > 1 {
			label = fmt.Sprintf("%s %d", base, seenRefill[base])
		}
		result.Quotas = append(result.Quotas, Quota{
			Label:     label,
			Used:      num(account.CycleCapacityUsedPrecise, account.CycleCapacityUsed),
			Total:     num(account.CycleCapacitySizePrecise, account.CycleCapacitySize),
			ResetAt:   parseReset(account.CycleEndTime),
			Recurring: true,
		})
	}
	for i, account := range bonuses {
		result.Quotas = append(result.Quotas, Quota{
			Label:     fmt.Sprintf("Bonus Pack %d", i+1),
			Used:      num(account.CapacityUsedPrecise, account.CapacityUsed),
			Total:     num(account.CapacitySizePrecise, account.CapacitySize),
			ResetAt:   parseReset(account.CycleEndTime),
			Recurring: false,
		})
	}
	if len(refills) > 0 {
		if refills[0].PackageName != "" {
			result.Plan = refills[0].PackageName
		} else if refills[0].SubProductName != "" {
			result.Plan = refills[0].SubProductName
		}
	}
	return result
}

func cycleEnd(account codebuddyAccount) time.Time {
	if reset := parseReset(account.CycleEndTime); !reset.IsZero() {
		return reset
	}
	return time.Time{}
}

func isRefill(account codebuddyAccount) bool {
	end := cycleEnd(account)
	if end.IsZero() {
		return false
	}
	deduction := time.UnixMilli(account.DeductionEndTime)
	return deduction.Sub(end) > refillGap
}

func refillCadence(account codebuddyAccount) string {
	start := parseReset(account.CycleStartTime)
	end := parseReset(account.CycleEndTime)
	if !start.IsZero() && !end.IsZero() {
		days := end.Sub(start).Hours() / 24
		if days <= 1.5 {
			return "Daily"
		}
		if days <= 10 {
			return "Weekly"
		}
	}
	return "Monthly"
}
