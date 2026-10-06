// Kimi for Coding: the weekly and rate-limit windows one credential publishes at /v1/usages.
//
// @file      internal/service/quotafetch/kimi.go
// @for       Reads Kimi's published usage window and rate limit, and the plan tier they belong to.
// @uses      internal/service/quotafetch, context, encoding/json, fmt, math, net/http, regexp, strconv, strings, time
// @reason    Kimi authenticates this surface two ways, a stored API key as a raw x-api-key, an OAuth connection as a bearer token with device identification, and its 403 means "no usage entitlement", not a dead session, so the two refusals stay distinct.
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
	"regexp"
	"strings"
)

const (
	kimiDisplay = "Kimi Coding"
	// The host the reference hard-codes, to promote into familyEndpoints: the kimi entry declares
	// no usage block, and a declared transport.usage.url still beats this built-in.
	kimiUsageURL           = "https://api.kimi.com/coding/v1/usages"
	kimiMissingCredential  = "Kimi access token or API key not available."
	kimiExpiredMessage     = "Kimi authentication expired. Please re-authorize."
	kimiNoPermission       = "Kimi connected, but this account has no permission to view usage. Subscribe to Kimi Code to access quota."
	kimiInvalidJSON        = kimiDisplay + ". Invalid JSON response from API."
	kimiNoQuotaMessage     = kimiDisplay + ". Usage tracked per request."
	kimiReasonNoPermission = "REASON_FEATURE_NO_PERMISSION"
)

var kimiPlanLevels = map[string]string{
	"LEVEL_BASIC": "Moderato", "LEVEL_INTERMEDIATE": "Allegretto",
	"LEVEL_ADVANCED": "Allegro", "LEVEL_STANDARD": "Vivace",
}
var kimiPermissionPattern = regexp.MustCompile(`(?i)permission_denied|do not have permission|subscribe`)

type kimiAnswer struct {
	Usage  kimiWindow `json:"usage"`
	Limits []struct {
		Detail kimiWindow `json:"detail"`
	} `json:"limits"`
	User struct {
		Membership struct {
			Level string `json:"level"`
		} `json:"membership"`
	} `json:"user"`
}

type kimiWindow struct {
	Limit     json.RawMessage `json:"limit"`
	Used      json.RawMessage `json:"used"`
	Remaining json.RawMessage `json:"remaining"`
	ResetTime json.RawMessage `json:"resetTime"`
	ResetAt   json.RawMessage `json:"reset_at"`
	ResetAlt  json.RawMessage `json:"resetAt"`
}

func fetchKimi(ctx context.Context, creds Credentials) Result {
	headers, credentialed := kimiAuth(creds)
	if !credentialed {
		return Result{Plan: kimiDisplay, Message: kimiMissingCredential}
	}
	endpoint := endpointFor(declaredOr(creds.Endpoints.URL, kimiUsageURL), creds.Endpoint)
	response, err := requestUsage(ctx, http.MethodGet, endpoint, headers, "")
	if err != nil {
		return Result{Plan: kimiDisplay, Message: fmt.Sprintf("%s. Unable to fetch usage: %s", kimiDisplay, err)}
	}
	if response.status < 200 || response.status >= 300 {
		return kimiFailure(response)
	}
	var answer kimiAnswer
	if !json.Valid(response.body) || json.Unmarshal(response.body, &answer) != nil {
		return Result{Plan: kimiDisplay, Message: kimiInvalidJSON}
	}
	result := Result{Plan: kimiPlan(answer.User.Membership.Level), Quotas: kimiRows(answer)}
	if len(result.Quotas) == 0 {
		result.Message = kimiNoQuotaMessage
	}
	return result
}

// kimiAuth picks the credential and its scheme: a stored API key travels as the raw x-api-key the
// platform itself sends, a keyless OAuth connection as a bearer token beside the device identity.
func kimiAuth(creds Credentials) (map[string]string, bool) {
	headers := make(map[string]string, len(creds.UsageHeaders)+6)
	for key, value := range creds.UsageHeaders {
		if strings.TrimSpace(value) != "" {
			headers[key] = value
		}
	}
	headers["Content-Type"] = "application/json"
	if apiKey := strings.TrimSpace(creds.APIKey); apiKey != "" {
		headers["x-api-key"] = apiKey
		return headers, true
	}
	token := strings.TrimSpace(creds.AccessToken)
	if token == "" {
		return nil, false
	}
	headers["Authorization"] = "Bearer " + token
	// The product pair the endpoint already sees this connection send, and the device id it was
	// authorised on: a server-side read has no machine to name, so none is stored, none is sent.
	headers["X-Msh-Platform"], headers["X-Msh-Version"] = "9router", "1.0.0"
	headers["X-Msh-Device-Id"] = kimiFirst(creds.ProviderSpecificData["deviceId"],
		creds.ProviderSpecificData["device_id"])
	return headers, true
}

// kimiRows renders the rolling window then the rate limit. Every published limit lands on the one
// "Ratelimit" row, the last that states a ceiling, as the reference's map key does.
func kimiRows(answer kimiAnswer) []Quota {
	rows := make([]Quota, 0, 2)
	if weekly, ok := kimiRow("Weekly", answer.Usage); ok {
		rows = append(rows, weekly)
	}
	rate, stated := Quota{}, false
	for _, item := range answer.Limits {
		if next, ok := kimiRow("Ratelimit", item.Detail); ok {
			rate, stated = next, true
		}
	}
	if stated {
		rows = append(rows, rate)
	}
	return rows
}

// kimiRow reads one allocation, drawing the bar from the provider's remaining count when it states
// one, because that is what the reference's percentage is made of and the two need not agree.
func kimiRow(label string, window kimiWindow) (Quota, bool) {
	stated, hasCeiling := kimiScalar(window.Limit)
	total := math.Max(0, stated)
	if !hasCeiling || total <= 0 {
		return Quota{}, false
	}
	used, _ := kimiScalar(window.Used)
	if remaining, hasRemaining := kimiScalar(window.Remaining); hasRemaining {
		used = total - math.Max(0, remaining)
	}
	return Quota{Label: label, Used: math.Min(math.Max(0, used), total), Total: total,
		ResetAt: kimiReset(window)}, true
}
