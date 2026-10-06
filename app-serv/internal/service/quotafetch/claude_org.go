// The Claude legacy path and the published-window document both paths share: the settings
// read that names an organization, the organization-usage read behind it, and the turning of
// five_hour / seven_day / seven_day_<model> / limits[] into rows in the card's order.
//
// @file      internal/service/quotafetch/claude_org.go
// @for       Reads Claude's organization settings and usage document into percent rows.
// @uses      internal/service/quotafetch, context, encoding/json, net/http, sort, strings, time
// @reason    Consumer OAuth tokens answer the primary endpoint and admin API tokens answer the organization one, both in the same window document; the reference falls back from the first to the second, and the card needs the rows in the order the provider listed them rather than in map order.
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
	"sort"
	"strings"
	"time"
)

const (
	// claudeSettingsURL and claudeOrgUsageURL are the legacy pair's built-ins, named here for
	// the same reason the primary is: endpoints.go stays untouched until the integrator
	// promotes them under "claude". The org URL carries the {org_id} placeholder the
	// registry declares, filled from the settings read below.
	claudeSettingsURL = "https://api.anthropic.com/v1/settings"
	claudeOrgUsageURL = "https://api.anthropic.com/v1/organizations/{org_id}/usage"

	claudeOrgIDPlaceholder = "{org_id}"
	claudeUnknownPlan      = "Unknown"

	claudeAdminNote     = "Claude connected. Usage details require admin access."
	claudePermissionMsg = "Claude connected. Usage API requires admin permissions."
)

// claudeWindow is one published utilization window. `utilization` is a pointer because a
// window is only read when it states a number: an object that carries none is an absent
// window, and a row of 0% would read as an untouched allocation.
type claudeWindow struct {
	Utilization *float64        `json:"utilization"`
	ResetsAt    json.RawMessage `json:"resets_at"`
}

// claudeLimit is a model-scoped weekly limit, which arrives in `limits[]` rather than as one
// of the seven_day_* keys.
type claudeLimit struct {
	Kind     string          `json:"kind"`
	Percent  *float64        `json:"percent"`
	ResetsAt json.RawMessage `json:"resets_at"`
	Scope    struct {
		Model struct {
			DisplayName string `json:"display_name"`
		} `json:"model"`
	} `json:"scope"`
}

// claudeSettings is what the settings read publishes: the plan word the card titles with and
// the organization the usage read is scoped to. The organization name the reference also
// carries has no field in Result and is reported nowhere.
type claudeSettings struct {
	Plan           string `json:"plan"`
	OrganizationID string `json:"organization_id"`
}

// claudeLegacy is the fallback for a primary that refused for a reason a second endpoint
// could still answer, or answered 2xx with nothing this card reads.
func claudeLegacy(ctx context.Context, creds Credentials, token string) Result {
	settingsURL := endpointFor(declaredOr(creds.Endpoints.SettingsURL, claudeSettingsURL), creds.Endpoint)
	settingsResponse, err := requestUsage(ctx, http.MethodGet, settingsURL,
		claudeVersionedHeaders(token, creds.UsageHeaders), "")
	if err != nil {
		return Result{Message: fmt.Sprintf("Claude connected. Unable to fetch usage: %s", err)}
	}
	if settingsResponse.status < 200 || settingsResponse.status >= 300 {
		return Result{Message: claudePermissionMsg}
	}

	var settings claudeSettings
	if err := json.Unmarshal(settingsResponse.body, &settings); err != nil {
		return Result{Plan: claudeUnknownPlan, Message: claudePermissionMsg}
	}
	plan := claudePlan(settings.Plan)
	orgID := strings.TrimSpace(settings.OrganizationID)
	if orgID == "" {
		return Result{Plan: plan, Message: claudeAdminNote}
	}

	usageURL := endpointFor(claudeFillOrgID(declaredOr(creds.Endpoints.OrgURL, claudeOrgUsageURL), orgID), creds.Endpoint)
	usageResponse, err := requestUsage(ctx, http.MethodGet, usageURL,
		claudeVersionedHeaders(token, creds.UsageHeaders), "")
	if err != nil {
		return Result{Plan: plan, Message: fmt.Sprintf("Claude connected. Unable to fetch usage: %s", err)}
	}
	if usageResponse.status < 200 || usageResponse.status >= 300 {
		return Result{Plan: plan, Message: claudeAdminNote}
	}
	if result, usable := claudeDocumentResult(usageResponse); usable {
		result.Plan = plan
		return result
	}
	return Result{Plan: plan, Message: claudeAdminNote}
}

// claudeFillOrgID substitutes the placeholder the registry declares. A declared org_url that
// carries no placeholder is taken as the operator wrote it: an exact endpoint needs no id
// inserted into it, and inventing a path segment would silently move the read.
func claudeFillOrgID(template string, orgID string) string {
	if !strings.Contains(template, claudeOrgIDPlaceholder) {
		return template
	}
	return strings.ReplaceAll(template, claudeOrgIDPlaceholder, orgID)
}

func claudePlan(plan string) string {
	if trimmed := strings.TrimSpace(plan); trimmed != "" {
		return trimmed
	}
	return claudeUnknownPlan
}

// claudeDocumentResult turns one published window document into rows. It reports false when
// the provider answered but published nothing this card reads, which is the reference's own
// trigger for the legacy read and, on the legacy read itself, the admin-access sentence.
func claudeDocumentResult(response usageResponse) (Result, bool) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(response.body, &fields); err != nil {
		return Result{}, false
	}
	quotas := claudeRows(fields)
	if len(quotas) == 0 {
		return Result{}, false
	}
	return Result{Plan: claudePlanLabel, Quotas: quotas}, true
}

// claudeRows reads the windows in the order the card renders them: the 5-hour session, the
// general 7-day week, then the model-scoped weeks, then the scoped limits that share those
// names.
func claudeRows(fields map[string]json.RawMessage) []Quota {
	rows := make([]Quota, 0, 3)
	if row, ok := claudeRow("session (5h)", fields["five_hour"]); ok {
		rows = append(rows, row)
	}
	if row, ok := claudeRow("weekly (7d)", fields["seven_day"]); ok {
		rows = append(rows, row)
	}
	for _, key := range claudeSortedKeys(fields) {
		if !strings.HasPrefix(key, "seven_day_") {
			continue
		}
		model := strings.TrimPrefix(key, "seven_day_")
		if row, ok := claudeRow(fmt.Sprintf("weekly %s (7d)", model), fields[key]); ok {
			rows = claudePut(rows, row)
		}
	}
	return claudeLimitRows(rows, fields["limits"])
}

// claudeRow maps one published window to a percent row, leaving the provider's utilization
// as stated: a window published over 100 is an overspent one, not a capped one.
func claudeRow(label string, raw json.RawMessage) (Quota, bool) {
	var window claudeWindow
	if err := json.Unmarshal(raw, &window); err != nil || window.Utilization == nil {
		return Quota{}, false
	}
	return claudePercentRow(label, *window.Utilization, parseReset(window.ResetsAt)), true
}

// claudePercentRow builds the one row shape this family publishes.
func claudePercentRow(label string, used float64, resetsAt time.Time) Quota {
	return Quota{
		Label:     label,
		Used:      used,
		Total:     claudePercentWindow,
		Unit:      "%",
		ResetAt:   resetsAt,
		Recurring: true,
	}
}

// claudeLimitRows adds the model-scoped weekly limits that arrive as { kind:
// "weekly_scoped", percent, scope.model.display_name }. No entry means the account has no
// such window, so no row is invented for it.
func claudeLimitRows(rows []Quota, raw json.RawMessage) []Quota {
	var limits []claudeLimit
	if err := json.Unmarshal(raw, &limits); err != nil {
		return rows
	}
	for _, limit := range limits {
		if limit.Kind != "weekly_scoped" || limit.Percent == nil {
			continue
		}
		model := strings.ToLower(strings.TrimSpace(limit.Scope.Model.DisplayName))
		if model == "" {
			continue
		}
		rows = claudePut(rows, claudePercentRow(fmt.Sprintf("weekly %s (7d)", model),
			claudeClamp(*limit.Percent), parseReset(limit.ResetsAt)))
	}
	return rows
}

// claudeClamp holds a scoped percent inside the window it is a share of, the reference's own
// Math.max(0, Math.min(100, percent)).
func claudeClamp(percent float64) float64 {
	if percent < 0 {
		return 0
	}
	if percent > claudePercentWindow {
		return claudePercentWindow
	}
	return percent
}

// claudePut writes a row by label the way a keyed object does: a label already present keeps
// its position and takes the newer value, so a scoped limit and a same-model window never
// arrive as two rows the card cannot tell apart.
func claudePut(rows []Quota, row Quota) []Quota {
	for index, existing := range rows {
		if existing.Label == row.Label {
			rows[index] = row
			return rows
		}
	}
	return append(rows, row)
}

// claudeSortedKeys orders the document's own keys. The reference walks Object.entries in
// response order; a Go map has no order, so the model-scoped windows are listed by name,
// which keeps two reads of the same document identical.
func claudeSortedKeys(fields map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
