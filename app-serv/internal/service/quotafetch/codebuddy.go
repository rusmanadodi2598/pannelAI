// The CodeBuddy families (CN and Intl): one Tencent billing endpoint per region.
//
// @file      internal/service/quotafetch/codebuddy.go
// @for       Ports the reference's getCodeBuddyUsage (services/usage/codebuddy-cn.js) for both regions.
// @uses      internal/service/quotafetch, net/http, encoding/json
// @reason    CodeBuddy quota mixes recurring refill packs and one-shot bonus packs that must not be merged.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-25
package quotafetch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"
)

// refillGap is the cycle-to-validity gap that separates a refill pack (whose allowance
// rolls over long before the resource expires) from a one-shot bonus pack (whose cycle
// ends exactly at expiry). The reference's constant: two days in milliseconds.
const refillGap = 2 * 24 * time.Hour

// fetchCodeBuddy binds the shared billing reader to one region's endpoint and headers.
func fetchCodeBuddy(family codebuddyFamily) func(context.Context, Credentials) Result {
	return func(ctx context.Context, creds Credentials) Result {
		return readCodeBuddy(ctx, family, creds)
	}
}

func readCodeBuddy(ctx context.Context, family codebuddyFamily, creds Credentials) Result {
	endpoint := endpointFor(familyEndpoints["codebuddy-cn"], creds.Endpoint)
	if family.name != "CN" {
		endpoint = endpointFor(familyEndpoints["codebuddy-intl"], creds.Endpoint)
	}

	token := creds.AccessToken
	if token == "" {
		token = creds.APIKey
	}
	if token == "" {
		return Result{Message: fmt.Sprintf("CodeBuddy %s credential not available.", family.name)}
	}

	payload, err := postCodeBuddy(ctx, endpoint, family, token)
	if err != nil {
		switch detail := err.(type) {
		case refusedCredential:
			return Result{Message: fmt.Sprintf("CodeBuddy %s credential invalid or expired (%d).", family.name, detail.status)}
		case billingRejected:
			return Result{Message: fmt.Sprintf("CodeBuddy %s quota error: %s", family.name, detail.message)}
		default:
			return Result{Message: fmt.Sprintf("CodeBuddy %s error: %s", family.name, err)}
		}
	}
	return codeBuddyResult(family, payload)
}

// refusedCredential and billingRejected are the two provider answers that stay soft:
// the sentence each maps to is what the reference's card renders for them.
type refusedCredential struct {
	status int
}

func (e refusedCredential) Error() string {
	return fmt.Sprintf("credential refused (%d)", e.status)
}

type billingRejected struct {
	message string
}

func (e billingRejected) Error() string {
	return e.message
}

func postCodeBuddy(ctx context.Context, endpoint string, family codebuddyFamily, token string) (codebuddyEnvelope, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBufferString("{}"))
	if err != nil {
		return codebuddyEnvelope{}, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	for key, value := range family.headers {
		request.Header.Set(key, value)
	}

	response, err := client.Do(request)
	if err != nil {
		return codebuddyEnvelope{}, err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return codebuddyEnvelope{}, refusedCredential{status: response.StatusCode}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return codebuddyEnvelope{}, fmt.Errorf("quota API error (%d)", response.StatusCode)
	}

	var envelope struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return codebuddyEnvelope{}, err
	}
	if envelope.Code != 0 {
		message := envelope.Msg
		if message == "" {
			message = "unknown"
		}
		return codebuddyEnvelope{}, billingRejected{message: message}
	}

	var parsed codebuddyEnvelope
	if err := json.Unmarshal(envelope.Data, &parsed); err != nil {
		return codebuddyEnvelope{}, err
	}
	return parsed, nil
}

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
