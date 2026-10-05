// The Gemini CLI family: v1internal:retrieveUserQuota, the same call `gemini /stats` makes,
// answering one bucket per model.
//
// @file      internal/service/quotafetch/google.go
// @for       Reads Gemini CLI's per-model quota buckets from the Cloud Code Assist quota call.
// @uses      internal/service/quotafetch, context, encoding/json, net/http, strings, google_account.go
// @reason    The quota endpoint refuses a call that names no Cloud project, and the project is
//
//	stored on the connection rather than returned by the quota read, so this
//	fetch has a lookup step in front of it and a project-less answer has to
//	stay a sentence the operator can act on rather than a failed page.
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
	"net/http"
	"strings"
)

const (
	// The Cloud Code Assist built-ins the gemini-cli entry asks. They are declared here while
	// endpoints.go stays untouched; the integrator promotes them under "gemini-cli", and
	// antigravity's own hosts sit with that fetcher in google_quota.go.
	googleGeminiQuotaURL    = "https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota"
	googleLoadCodeAssistURL = "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist"

	googleCLIDisplay = "Gemini CLI"
	googleFreePlan   = "Free"

	googleNoTokenMessage = "Gemini CLI access token not available."
	googleNoProjectNote  = "Gemini CLI project ID not available. Reconnect Gemini CLI, or configure a " +
		"Google Cloud project with Gemini Code Assist access before checking quota."
	googleNoBucketsNote = "Gemini CLI published no quota buckets for this project."
)

// googleBucket is one published per-model window. `remainingFraction` is the share LEFT, the
// inverse of Claude's utilization, and it stays raw because the provider sends a number here
// and a quoted numeric string there.
type googleBucket struct {
	ModelID           string          `json:"modelId"`
	RemainingFraction json.RawMessage `json:"remainingFraction"`
	ResetTime         json.RawMessage `json:"resetTime"`
}

// googleQuotaAnswer is the retrieveUserQuota document.
type googleQuotaAnswer struct {
	Buckets []googleBucket `json:"buckets"`
}

// fetchGeminiCLI is the "gemini-cli" entry.
func fetchGeminiCLI(ctx context.Context, creds Credentials) Result {
	token := strings.TrimSpace(creds.AccessToken)
	if token == "" {
		return Result{Plan: googleFreePlan, Message: googleNoTokenMessage}
	}

	plan := googleFreePlan
	project := googleStoredProject(creds.ProviderSpecificData)
	if project == "" {
		account, ok := googleLoadAccount(ctx, creds, token, googleBootstrap{
			declared: creds.Endpoints.LoadCodeAssistURL, builtIn: googleLoadCodeAssistURL,
		})
		if ok {
			project = googleAccountProject(account.Project)
			if tier := strings.TrimSpace(account.CurrentTier.Name); tier != "" {
				plan = tier
			}
		}
	}
	if project == "" {
		return Result{Plan: plan, Message: googleNoProjectNote}
	}

	endpoint := endpointFor(declaredOr(creds.Endpoints.QuotaURL, googleGeminiQuotaURL), creds.Endpoint)
	response, err := requestUsage(ctx, http.MethodPost, endpoint,
		googleJSONHeaders(token, "", creds.UsageHeaders), googleProjectBody(project))
	if err != nil {
		return Result{Plan: plan, Message: fmt.Sprintf("%s error: %s", googleCLIDisplay, err)}
	}
	if response.status < 200 || response.status >= 300 {
		return Result{Plan: plan, Message: fmt.Sprintf("%s quota error (%d).", googleCLIDisplay, response.status)}
	}

	var answer googleQuotaAnswer
	if err := json.Unmarshal(response.body, &answer); err != nil {
		return Result{Plan: plan, Message: fmt.Sprintf("%s error: %s", googleCLIDisplay, err)}
	}
	buckets := googleBucketRows(answer.Buckets)
	if len(buckets) == 0 {
		return Result{Plan: plan, Message: googleNoBucketsNote}
	}
	return Result{Plan: plan, Quotas: buckets}
}

// googleBucketRows renders every published bucket that names a model and states a fraction,
// in the order the provider listed them. A bucket without either is not a window the card can
// name, and drawing one as spent would be a fabrication.
func googleBucketRows(buckets []googleBucket) []Quota {
	rows := make([]Quota, 0, len(buckets))
	for _, bucket := range buckets {
		model := strings.TrimSpace(bucket.ModelID)
		if model == "" {
			continue
		}
		fraction, ok := googleFraction(bucket.RemainingFraction)
		if !ok {
			continue
		}
		rows = append(rows, googleShareRow(model, fraction, parseReset(bucket.ResetTime)))
	}
	return rows
}
