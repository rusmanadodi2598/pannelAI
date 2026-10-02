// The Groq family: Groq publishes no usage endpoint, so its quota is read off the
// x-ratelimit-* response headers of GET /v1/models (the same call the connector already
// uses to validate a key), for requests and tokens.
//
// @file      internal/service/quotafetch/groq.go
// @for       Reads Groq's published quota off the rate-limit headers of GET /v1/models.
// @uses      internal/service/quotafetch, net/http, strconv, time
// @reason    Groq meters each key with rolling rate limits reported only as response
//
//	headers, and the reset value is a Go-style duration string rather than a
//	timestamp, so the read lives off the headers and needs its own duration-to-
//	instant conversion.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-02
package quotafetch

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// groqModelsURL is the built-in models endpoint, declared here because this family
	// owns it while endpoints.go stays untouched. The integrator promotes it into
	// familyEndpoints under the "groq" key.
	groqModelsURL = "https://api.groq.com/openai/v1/models"
	groqDisplay   = "Groq"
)

// fetchGroq is the "groq" entry: a bearer-keyed GET whose quota is entirely in headers.
func fetchGroq(ctx context.Context, creds Credentials) Result {
	endpoint := endpointFor(declaredOr(creds.Endpoints.URL, groqModelsURL), creds.Endpoint)

	token := strings.TrimSpace(creds.APIKey)
	if token == "" {
		token = strings.TrimSpace(creds.AccessToken)
	}
	if token == "" {
		return Result{Message: "Groq API key not available. Add a key to view usage."}
	}

	response, err := requestUsage(ctx, http.MethodGet, endpoint, bearer(token, creds.UsageHeaders), "")
	if err != nil {
		return Result{Message: fmt.Sprintf("Groq error: %s", err)}
	}
	if failure, refused := response.softFailure(groqDisplay); refused {
		return failure
	}
	return groqResult(response.header)
}

// groqResult turns the x-ratelimit-* trios into the two windows Groq publishes. A key can
// report one bucket without the other, so each is read independently; a read that reported
// neither is a valid connection that simply has no meter yet, which is not an error and the
// card must not render it as one.
func groqResult(header http.Header) Result {
	quotas := make([]Quota, 0, 2)
	if row, ok := groqRateRow(header, "Requests", "requests",
		"x-ratelimit-limit-requests", "x-ratelimit-remaining-requests", "x-ratelimit-reset-requests"); ok {
		quotas = append(quotas, row)
	}
	if row, ok := groqRateRow(header, "Tokens", "tokens",
		"x-ratelimit-limit-tokens", "x-ratelimit-remaining-tokens", "x-ratelimit-reset-tokens"); ok {
		quotas = append(quotas, row)
	}
	if len(quotas) == 0 {
		return Result{Plan: groqDisplay, Message: "Groq connected. No rate-limit data reported for this key yet."}
	}
	return Result{Plan: groqDisplay, Quotas: quotas}
}

// groqRateRow builds one window from a limit/remaining/reset header trio. A header the
// provider did not send reads as empty, which is absence rather than a real "0 remaining"
// quota, so both counts must be present before a row is reported.
func groqRateRow(header http.Header, label, unit, limitKey, remainingKey, resetKey string) (Quota, bool) {
	limitRaw := strings.TrimSpace(header.Get(limitKey))
	remainingRaw := strings.TrimSpace(header.Get(remainingKey))
	if limitRaw == "" || remainingRaw == "" {
		return Quota{}, false
	}
	limit, err := strconv.ParseFloat(limitRaw, 64)
	if err != nil {
		return Quota{}, false
	}
	remaining, err := strconv.ParseFloat(remainingRaw, 64)
	if err != nil {
		return Quota{}, false
	}
	used := limit - remaining
	if used < 0 {
		used = 0
	}
	return Quota{
		Label:     label,
		Used:      used,
		Total:     limit,
		Unit:      unit,
		ResetAt:   groqResetFrom(header.Get(resetKey)),
		Recurring: true,
	}, true
}

// groqResetFrom converts Groq's Go-style duration string ("2m59.56s") into the instant the
// window refills. The header is a duration, not a timestamp, so time.ParseDuration is the
// correct reader; a value it cannot parse leaves no reset rather than failing the whole read,
// which matches the reference and keeps one malformed header from hiding a working quota.
func groqResetFrom(raw string) time.Time {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}
	}
	window, err := time.ParseDuration(trimmed)
	if err != nil || window < 0 {
		return time.Time{}
	}
	return time.Now().Add(window)
}
