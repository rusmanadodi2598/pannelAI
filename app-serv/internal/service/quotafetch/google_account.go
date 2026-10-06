// The Cloud Code Assist client both Google products share: the client identification their
// bootstrap calls demand, the project lookup that sits in front of every quota read, and the
// remaining-fraction arithmetic one published share becomes a row with.
//
// @file      internal/service/quotafetch/google_account.go
// @for       Resolves a Google project and tier and turns published fractions into windows.
// @uses      internal/service/quotafetch, context, encoding/json, math, net/http, runtime, strconv, strings, time
// @reason    Both quota endpoints are scoped to a Cloud project they never name themselves, both publish shares rather than counters, and both refuse a call that carries the wrong client identification, so the two fetchers need one place that knows how the product introduces itself.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package quotafetch

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// googleNormalisedWindow is the ceiling a published remaining fraction is drawn against. The
// provider states a share and no counter, so a row is a share of a stated base, the
// reference's own convention, shared by both products.
const googleNormalisedWindow = 1000

// googleProjectDataKey is the name the reference's providerSpecificData carries the
// connection's Cloud project under. The service layer must fill it; it does not yet.
const googleProjectDataKey = "projectId"

// The Cloud Code Assist client identification, from the reference's CLIENT_METADATA: a fixed
// ideType and pluginType for this product pair, and a platform derived from the host process.
const (
	googleIDETypeAntigravity = 9
	googlePluginTypeGemini   = 2

	googlePlatformUnspecified  = 0
	googlePlatformDarwinAMD64  = 1
	googlePlatformDarwinARM64  = 2
	googlePlatformLinuxAMD64   = 3
	googlePlatformLinuxARM64   = 4
	googlePlatformWindowsAMD64 = 5
)

// googleMetadata is the CLIENT_METADATA body member every bootstrap call demands.
type googleMetadata struct {
	IdeType    int `json:"ideType"`
	Platform   int `json:"platform"`
	PluginType int `json:"pluginType"`
}

// googleAccount is the subscription information loadCodeAssist publishes. Both members the
// fetchers read are polymorphic across the two products, so they stay raw and are read by the
// helpers below.
type googleAccount struct {
	Project     json.RawMessage `json:"cloudaicompanionProject"`
	CurrentTier struct {
		Name string `json:"name"`
	} `json:"currentTier"`
	PaidTier struct {
		ID string `json:"id"`
	} `json:"paidTier"`
}

// googleBootstrap names one loadCodeAssist call: the declared key its host may sit under, the
// built-in when nothing declares it, whether this product sends the mode flag, and the user
// agent it identifies itself with.
type googleBootstrap struct {
	declared  string
	builtIn   string
	mode      bool
	userAgent string
}

// googleStoredProject reads the project the connection stores. The reference normalizes both a
// bare id and an `{ id }` object, and the value reaches this package as a string, so a
// stringified object is unwrapped the same way rather than sent as JSON text.
func googleStoredProject(data map[string]string) string {
	value := strings.TrimSpace(data[googleProjectDataKey])
	if !strings.HasPrefix(value, "{") {
		return value
	}
	return googleAccountProject(json.RawMessage(value))
}

// googleAccountProject reads the project loadCodeAssist publishes, which arrives as a bare id
// on one account shape and as an object carrying it on another.
func googleAccountProject(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var reference struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &reference); err == nil {
		return strings.TrimSpace(reference.ID)
	}
	return ""
}

// googleLoadAccount asks loadCodeAssist for the subscription information. A call that fails or
// is refused reports false rather than an error: the reference treats this lookup as
// best-effort and lets the caller answer with its own project-not-available sentence.
func googleLoadAccount(ctx context.Context, creds Credentials, token string, bootstrap googleBootstrap) (googleAccount, bool) {
	endpoint := endpointFor(declaredOr(bootstrap.declared, bootstrap.builtIn), creds.Endpoint)
	response, err := requestUsage(ctx, http.MethodPost, endpoint,
		googleJSONHeaders(token, bootstrap.userAgent, creds.UsageHeaders), googleAssistBody(bootstrap.mode))
	if err != nil || response.status < 200 || response.status >= 300 {
		return googleAccount{}, false
	}
	var account googleAccount
	if err := json.Unmarshal(response.body, &account); err != nil {
		return googleAccount{}, false
	}
	return account, true
}

// googleJSONHeaders is the header set every Cloud Code Assist read sends: the bearer, JSON, and
// the product user agent when that call identifies itself as an IDE.
func googleJSONHeaders(token string, userAgent string, extra map[string]string) map[string]string {
	headers := bearer(token, extra)
	headers["Content-Type"] = "application/json"
	if userAgent != "" {
		headers["User-Agent"] = userAgent
	}
	return headers
}

// googleProjectBody names the project a read is scoped to. An empty project sends `{}`, the
// reference's own body for a lookup that has nothing to scope to yet.
func googleProjectBody(project string) string {
	body, err := json.Marshal(googleProjectRequest{Project: strings.TrimSpace(project)})
	if err != nil {
		return "{}"
	}
	return string(body)
}

type googleProjectRequest struct {
	Project string `json:"project,omitempty"`
}

// googleAssistBody is the loadCodeAssist request. Antigravity sends the mode flag and the CLI
// does not, which is the only difference between the two products' lookups.
func googleAssistBody(mode bool) string {
	modeValue := 1
	request := struct {
		Metadata googleMetadata `json:"metadata"`
		Mode     *int           `json:"mode,omitempty"`
	}{Metadata: googleClientMetadata()}
	if mode {
		request.Mode = &modeValue
	}
	body, err := json.Marshal(request)
	if err != nil {
		return "{}"
	}
	return string(body)
}

func googleClientMetadata() googleMetadata {
	return googleMetadata{
		IdeType:    googleIDETypeAntigravity,
		Platform:   googlePlatformEnum(runtime.GOOS, runtime.GOARCH),
		PluginType: googlePluginTypeGemini,
	}
}

// googlePlatformEnum maps the host process onto the platform enum the endpoint expects. The two
// spellings of Windows are accepted because Node reports win32 and Go reports windows for the
// same system, and an unmapped host answers unspecified rather than a wrong real value.
func googlePlatformEnum(goos string, goarch string) int {
	switch goos {
	case "darwin":
		if goarch == "arm64" {
			return googlePlatformDarwinARM64
		}
		return googlePlatformDarwinAMD64
	case "linux":
		if goarch == "arm64" {
			return googlePlatformLinuxARM64
		}
		return googlePlatformLinuxAMD64
	case "windows", "win32":
		return googlePlatformWindowsAMD64
	default:
		return googlePlatformUnspecified
	}
}

// googleShareRow draws a published remaining fraction as a used-over-total window. An
// over-allocated fraction (the provider has been observed to answer above 1 mid-reset) clamps
// to a fully untouched window rather than a negative used count.
func googleShareRow(label string, remainingFraction float64, resetsAt time.Time) Quota {
	remaining := math.Round(float64(googleNormalisedWindow) * remainingFraction)
	if remaining < 0 {
		remaining = 0
	}
	if remaining > googleNormalisedWindow {
		remaining = googleNormalisedWindow
	}
	return Quota{
		Label:     label,
		Used:      float64(googleNormalisedWindow) - remaining,
		Total:     googleNormalisedWindow,
		ResetAt:   resetsAt,
		Recurring: true,
	}
}

// googleFraction reads a published remaining fraction, sent as a JSON number or a numeric
// string. An absent, null or unreadable value reports false rather than reading as zero: a zero
// the provider never published would render as a spent window.
func googleFraction(raw json.RawMessage) (float64, bool) {
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
