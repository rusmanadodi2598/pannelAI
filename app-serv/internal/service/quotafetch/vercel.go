// The Vercel AI Gateway family: a GET on the credits endpoint with the API key.
//
// @file      internal/service/quotafetch/vercel.go
// @for       Ports the reference's getVercelAiGatewayUsage (services/usage/misc.js:161).
// @uses      internal/service/quotafetch, net/http, encoding/json
// @reason    The gateway's quota is the credit balance its credits endpoint publishes.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-25
package quotafetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// monthlyCredit is Vercel's free monthly allocation in USD. The credits API answers a
// balance without the allocation it comes from, so the reference supplies the known
// constant as the denominator for the remaining share.
const monthlyCredit = 5.0

func fetchVercel(ctx context.Context, creds Credentials) Result {
	endpoint := endpointFor(familyEndpoints["vercel-ai-gateway"], creds.Endpoint)

	if creds.APIKey == "" {
		return Result{Message: "Vercel AI Gateway API key not available."}
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Result{Message: fmt.Sprintf("Vercel AI Gateway error: %s", err)}
	}
	request.Header.Set("Authorization", "Bearer "+creds.APIKey)
	request.Header.Set("Accept", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return Result{Message: fmt.Sprintf("Vercel AI Gateway error: %s", err)}
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return Result{Message: "Vercel AI Gateway API key invalid or expired."}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 200))
		detail := strings.TrimSpace(string(body))
		if detail != "" {
			detail = ": " + detail
		}
		return Result{Message: fmt.Sprintf("Vercel AI Gateway credits API error (%d)%s", response.StatusCode, detail)}
	}

	var payload struct {
		Balance   json.Number `json:"balance"`
		TotalUsed json.Number `json:"total_used"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return Result{Message: fmt.Sprintf("Vercel AI Gateway error: %s", err)}
	}
	balance := toFloat(payload.Balance)
	totalUsed := toFloat(payload.TotalUsed)

	if balance <= 0 && totalUsed <= 0 {
		return Result{
			Plan:    "Pay-as-you-go",
			Message: "Vercel AI Gateway connected. No credit allocation found (BYOK or unfunded account).",
		}
	}

	// The gateway spends a prepaid balance rather than a capped window, so the used row
	// has no ceiling (the reference marks it unlimited) and the remaining row is the
	// balance against the monthly allocation.
	return Result{
		Plan: "Pay-as-you-go",
		Quotas: []Quota{
			{Label: "Used (USD)", Used: totalUsed, Unlimited: true, Recurring: true},
			{Label: "Remaining (USD)", Used: balance, Total: monthlyCredit, Recurring: true},
		},
	}
}

func toFloat(value json.Number) float64 {
	parsed, err := value.Float64()
	if err != nil {
		return 0
	}
	return parsed
}
