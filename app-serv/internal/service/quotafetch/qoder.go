// Package quotafetch reads the quota a provider publishes for one of its connections.
//
// @file      internal/service/quotafetch/qoder.go
// @for       The Qoder quota read: the credit buckets the vendor publishes, and the
//
//	token exchange a Personal Access Token needs before the endpoint
//	accepts it.
//
// @uses      context, encoding/json, fmt, io, net/http, net/url, strconv, strings,
//
//	time, internal/provider.
//
// @reason    Qoder publishes credits rather than a percentage, in two buckets —
//
//	personal and organization — and its quota endpoint refuses a raw
//	Personal Access Token, so a read that presented the stored credential
//	would report a working account as broken. The exchange is the same one
//	the chat path uses, against the same host.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-27
package quotafetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
)

// qoderPatPrefix is the credential shape the quota endpoint will not read. It is
// spelled here rather than imported from the connector because this package owns its
// own credential rules, and a quota read must not depend on a connector being wired.
const (
	qoderPatPrefix = "pt-"

	// qoderResetHorizon caps how far ahead a published reset is believed. The vendor
	// answers a non-rolling plan with a year-9999 sentinel, which is a way of saying
	// "no reset date" rather than a date.
	qoderResetHorizon = 5 * 365 * 24 * time.Hour
)

// qoderUsage is one published bucket. `remaining` is read and deliberately dropped:
// the panel renders a window as used over a limit, and an absolute credit count
// smuggled in as that ratio reads 348 credits as 348% (the reference's own warning).
type qoderUsage struct {
	Total     qoderCount `json:"total"`
	Used      qoderCount `json:"used"`
	Remaining qoderCount `json:"remaining"`
	Unit      string     `json:"unit"`
}

// qoderCount accepts a count the vendor sends as either a number or a string, which
// is the shape the billing APIs of this family answer with.
type qoderCount float64

func (c *qoderCount) UnmarshalJSON(raw []byte) error {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		*c = 0
		return nil
	}
	unquoted := strings.Trim(trimmed, `"`)
	value, err := strconv.ParseFloat(unquoted, 64)
	if err != nil {
		return fmt.Errorf("quotafetch: the qoder quota count %q is not a number", unquoted)
	}
	*c = qoderCount(value)
	return nil
}

// fetchQoder reads one Qoder family's published quota. The family names both the
// endpoint and the openapi host the token exchange runs against, so intl and CN reach
// their own service and never each other's.
func fetchQoder(family string, openAPIBase string) func(context.Context, Credentials) Result {
	return func(ctx context.Context, creds Credentials) Result {
		endpoint := endpointFor(usageEndpoint(creds, family), creds.Endpoint)
		token, message := qoderQuotaToken(ctx, endpoint, openAPIBase, creds)
		if message != "" {
			return Result{Message: message}
		}

		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return Result{Message: fmt.Sprintf("%s error: %s", family, err)}
		}
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Accept", "application/json")

		response, err := client.Do(request)
		if err != nil {
			return Result{Message: fmt.Sprintf("%s error: %s", family, err)}
		}
		defer func() {
			// reason: the body is decoded below; a close error cannot be acted on.
			_ = response.Body.Close()
		}()

		if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
			return Result{Message: fmt.Sprintf("%s credential invalid or expired.", family)}
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(response.Body, 200))
			return Result{Message: fmt.Sprintf("%s quota API error (%d)%s", family, response.StatusCode, quotaDetail(body))}
		}

		var payload struct {
			UserType           string          `json:"userType"`
			UsageType          string          `json:"usageType"`
			IsQuotaExceeded    bool            `json:"isQuotaExceeded"`
			UserQuota          qoderUsage      `json:"userQuota"`
			OrganizationBucket qoderUsage      `json:"orgResourcePackage"`
			ExpiresAt          json.RawMessage `json:"expiresAt"`
		}
		if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
			return Result{Message: fmt.Sprintf("%s error: %s", family, err)}
		}
		return qoderResult(payload.UserType, payload.IsQuotaExceeded,
			payload.UserQuota, payload.OrganizationBucket, qoderReset(payload.ExpiresAt))
	}
}

// qoderReset reads the expiry the vendor publishes, dropping its "never resets"
// sentinel. The measured answer carries 253402214400000 — the year 9999 — for a
// plan whose allocation does not roll over, and a card that printed that as a reset
// date would show a number nobody can act on.
func qoderReset(raw json.RawMessage) time.Time {
	resets := parseReset(raw)
	if resets.IsZero() || resets.After(time.Now().Add(qoderResetHorizon)) {
		return time.Time{}
	}
	return resets
}

// qoderQuotaToken is the bearer the quota endpoint reads: a device or job token as
// stored, or an exchanged one when the account holds a Personal Access Token, which
// the endpoint refuses outright.
func qoderQuotaToken(ctx context.Context, endpoint, openAPIBase string, creds Credentials) (string, string) {
	token := strings.TrimSpace(creds.AccessToken)
	if token == "" {
		token = strings.TrimSpace(creds.APIKey)
	}
	if token == "" {
		return "", "Qoder credential not available."
	}
	if !strings.HasPrefix(token, qoderPatPrefix) {
		return token, ""
	}

	base := strings.TrimSuffix(openAPIBase, "/")
	if base == "" {
		// The registry's usage endpoint and its openapi host share a service, so the
		// answer's own origin is the fallback — and it is also what a test stub
		// redirects both calls with.
		base = originOf(endpoint)
	}
	exchanger, err := provider.NewQoderJobTokenClient(base, client)
	if err != nil {
		return "", fmt.Sprintf("Qoder error: %s", err)
	}
	jobToken, err := exchanger.JobToken(ctx, token)
	if err != nil {
		return "", fmt.Sprintf("Qoder error: %s", err)
	}
	return jobToken, ""
}

// qoderResult renders the published buckets as windows. A bucket that states no total
// and no use is not reported, because a row of zeros reads as a spent allocation
// rather than an absent one. When nothing publishes but the vendor says the quota is
// exceeded, that is the fact worth reporting — the measured answer for a credits plan
// with no credits does exactly this.
func qoderResult(userType string, exceeded bool, personal, organization qoderUsage, resetsAt time.Time) Result {
	quotas := make([]Quota, 0, 2)
	for _, bucket := range []struct {
		label string
		usage qoderUsage
	}{
		{"Personal", personal},
		{"Organization", organization},
	} {
		if bucket.usage.Total <= 0 && bucket.usage.Used <= 0 {
			continue
		}
		quotas = append(quotas, Quota{
			Label:   bucket.label,
			Used:    float64(bucket.usage.Used),
			Total:   float64(bucket.usage.Total),
			ResetAt: resetsAt,
		})
	}
	result := Result{Plan: strings.TrimSpace(userType), Quotas: quotas}
	switch {
	case len(quotas) > 0:
		return result
	case exceeded:
		result.Message = "Qoder reports this account's quota as exceeded."
	default:
		result.Message = "Qoder published no quota for this account."
	}
	return result
}

// quotaDetail renders a provider error body for a message, with the leading separator
// the existing families use, or nothing when the body was empty.
func quotaDetail(body []byte) string {
	detail := strings.TrimSpace(string(body))
	if detail == "" {
		return ""
	}
	return ": " + detail
}

// originOf reduces a URL to scheme and host, the part a sibling endpoint of the same
// service shares.
func originOf(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}
