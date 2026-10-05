// DeepSeek: the wallet balance the billing endpoint publishes for one API key.
//
// @file      internal/service/quotafetch/deepseek.go
// @for       Reads DeepSeek's per-currency balance and the availability flag that names the account.
// @uses      internal/service/quotafetch, context, encoding/json, net/http
// @reason    DeepSeek bills a prepaid wallet rather than a capped window, so its answer has
//
//	to reach the card as an amount of money: drawn as a window it would render a
//	balance as a share of something the provider never stated a ceiling for.
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

// deepSeekDisplayName is the plan word and the message subject for this family; the answer
// names no plan of its own beyond whether the wallet can be spent.
const deepSeekDisplayName = "DeepSeek"

// deepSeekBalanceURL is the billing host the reference hardcodes, because the deepseek
// registry entry declares no usage URL of its own, it is the built-in to promote into
// familyEndpoints. A declared transport.usage.url still wins over it.
const deepSeekBalanceURL = "https://api.deepseek.com/user/balance"

// deepSeekAnswer is the published wallet block. The availability flag arrives under both
// spellings across the endpoint's shapes, and an answer that states neither is read as not
// available, which is the reference's reading of an absent flag.
type deepSeekAnswer struct {
	IsAvailable      *bool                 `json:"is_available"`
	IsAvailableCamel *bool                 `json:"isAvailable"`
	BalanceInfos     []deepSeekBalanceInfo `json:"balance_infos"`
}

type deepSeekBalanceInfo struct {
	Currency        string          `json:"currency"`
	TotalBalance    json.RawMessage `json:"total_balance"`
	TotalBalanceAlt json.RawMessage `json:"totalBalance"`
}

func (a deepSeekAnswer) available() bool {
	return deepSeekFlagSet(a.IsAvailable) || deepSeekFlagSet(a.IsAvailableCamel)
}

func deepSeekFlagSet(flag *bool) bool {
	return flag != nil && *flag
}

// fetchDeepSeek reads the wallet the billing endpoint publishes for the stored key.
func fetchDeepSeek(ctx context.Context, creds Credentials) Result {
	apiKey := strings.TrimSpace(creds.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(creds.AccessToken)
	}
	if apiKey == "" {
		return Result{Plan: deepSeekDisplayName,
			Message: "DeepSeek API key not available. Add a key to view usage."}
	}

	endpoint := endpointFor(declaredOr(creds.Endpoints.URL, deepSeekBalanceURL), creds.Endpoint)
	response, err := requestUsage(ctx, http.MethodGet, endpoint, bearer(apiKey, creds.UsageHeaders), "")
	if err != nil {
		return Result{Plan: deepSeekDisplayName, Message: fmt.Sprintf("DeepSeek error: %s", err)}
	}
	if failure, soft := response.softFailure(deepSeekDisplayName); soft {
		failure.Plan = deepSeekDisplayName
		return failure
	}

	var answer deepSeekAnswer
	if err := json.Unmarshal(response.body, &answer); err != nil {
		return Result{Plan: deepSeekDisplayName, Message: "DeepSeek balance response was not JSON."}
	}
	// A response that carried no currency at all is a connected account with nothing to
	// show, which the reference names as such rather than as an empty card.
	wallets := deepSeekWallets(answer.BalanceInfos)
	if len(wallets) == 0 {
		return Result{Plan: deepSeekDisplayName, Message: "DeepSeek connected. No balance data returned."}
	}
	return Result{Plan: deepSeekPlan(answer), Quotas: wallets}
}

// deepSeekWallets renders one credit-balance row per published currency. The reference also
// reads granted and topped-up amounts and then discards them, so only the spendable total is
// ported: the operator's question is what is left to spend.
func deepSeekWallets(infos []deepSeekBalanceInfo) []Quota {
	wallets := make([]Quota, 0, len(infos))
	for _, info := range infos {
		currency := strings.ToUpper(strings.TrimSpace(info.Currency))
		if currency == "" {
			continue
		}
		wallets = append(wallets, Quota{
			Label:           fmt.Sprintf("Balance (%s)", currency),
			Total:           deepSeekBalance(info),
			Unit:            currency,
			IsCreditBalance: true,
		})
	}
	return wallets
}

// deepSeekBalance reads the spendable amount, which arrives as a number on some answers and
// a numeric string on others, under either spelling. An amount that cannot be read is an
// empty wallet, a stated zero and a missing value are not worth telling apart here, since
// neither can be spent.
func deepSeekBalance(info deepSeekBalanceInfo) float64 {
	amount := deepSeekAmount(info.TotalBalance)
	if amount == 0 {
		amount = deepSeekAmount(info.TotalBalanceAlt)
	}
	if amount < 0 {
		return 0
	}
	return amount
}

func deepSeekAmount(raw json.RawMessage) float64 {
	text := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	if text == "" || text == "null" {
		return 0
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}

// deepSeekPlan names the account the way the endpoint's availability flag does: a wallet the
// provider will not serve is labelled for what the operator would otherwise discover at the
// first request.
func deepSeekPlan(answer deepSeekAnswer) string {
	if answer.available() {
		return deepSeekDisplayName
	}
	return deepSeekDisplayName + " (Insufficient Balance)"
}
