// The Antigravity model surface: the IDE identification every call carries, and the
// fetchAvailableModels document reduced to the models this screen is allowed to show.
//
// @file      internal/service/quotafetch/antigravity_models.go
// @for       Reads Antigravity's published per-model windows and the headers that call needs.
// @uses      internal/service/quotafetch, encoding/json, strings, google_account.go
// @reason    The endpoint meters far more models than the card is meant to name, marks some of
//
//	them internal, and keys them by an id the display name cannot be matched
//	against once the map is walked — so the allowlist, the id it is keyed by
//	and the row it produces belong in one place.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-02
package quotafetch

import (
	"encoding/json"
	"strings"
)

// The IDE identification the endpoint refuses a call without. The registry's antigravity entry
// declares the same user agent on transport.headers; these are the built-ins for a caller that
// handed over no entry, and the version is the same release the user agent names.
const (
	antigravityIDEVersion = "2.11.0"
	antigravityUserAgent  = "antigravity/ide/2.11.0 darwin/arm64"
	antigravityClientName = "antigravity"
	antigravityFreeTierID = "free-tier"
)

// antigravityImportantModels is the reference's allowlist: the windows the card is allowed to
// show. A model outside it is metered by the provider but not published for this screen, and
// its order is also this fetcher's row order, because a Go map cannot preserve the document's.
var antigravityImportantModels = []string{
	"gemini-3.8-flash-high",
	"gemini-3.8-flash-medium",
	"gemini-3.8-flash-low",
	"gemini-3.7-flash-high",
	"gemini-3.7-flash-medium",
	"gemini-3.7-flash-low",
	"gemini-3.6-flash-high",
	"gemini-3.6-flash-medium",
	"gemini-3.6-flash-low",
	"gemini-3.5-flash-low",
	"gemini-3.5-flash-extra-low",
	"gemini-pro-agent",
	"gemini-3.1-pro-low",
	"claude-sonnet-4-6",
	"claude-opus-4-6-thinking",
	"gpt-oss-120b-medium",
	"gemini-3.1-flash-image",
}

// antigravityModelAnswer is the fetchAvailableModels document. `models` is keyed by the
// provider's own model id, which is also the row's identity.
type antigravityModelAnswer struct {
	Models map[string]antigravityModel `json:"models"`
}

type antigravityModel struct {
	DisplayName string                 `json:"displayName"`
	IsInternal  bool                   `json:"isInternal"`
	QuotaInfo   *antigravityModelQuota `json:"quotaInfo"`
}

type antigravityModelQuota struct {
	RemainingFraction json.RawMessage `json:"remainingFraction"`
	ResetTime         json.RawMessage `json:"resetTime"`
}

// antigravityModelWindow is one published model row kept together with the provider's own
// model id. The id, not the label, is what names the model's family: the label the card sees
// is a display name the provider spells however it likes.
type antigravityModelWindow struct {
	Quota
	modelID string
}

// antigravityHeaders is the IDE's own identification on every call it makes.
func antigravityHeaders(token string, extra map[string]string) map[string]string {
	headers := googleJSONHeaders(token, antigravityUserAgent, extra)
	headers["X-Client-Name"] = antigravityClientName
	headers["X-Client-Version"] = antigravityIDEVersion
	return headers
}

// antigravityModelRows renders the published allowlist models, in allowlist order. A model the
// provider marks internal, or that states no quota at all, is not a window this card claims to
// read.
func antigravityModelRows(models map[string]antigravityModel, freeTier bool) []antigravityModelWindow {
	if freeTier || len(models) == 0 {
		return nil
	}
	rows := make([]antigravityModelWindow, 0, len(models))
	for _, id := range antigravityImportantModels {
		model, ok := models[id]
		if !ok || model.IsInternal || model.QuotaInfo == nil {
			continue
		}
		fraction, ok := googleFraction(model.QuotaInfo.RemainingFraction)
		if !ok {
			continue
		}
		rows = append(rows, antigravityModelWindow{
			Quota: googleShareRow(antigravityModelLabel(model, id), fraction,
				parseReset(model.QuotaInfo.ResetTime)),
			modelID: id,
		})
	}
	return rows
}

// antigravityModelLabel names a model row by the display name the provider publishes, falling
// back to the id it is keyed by. The id is the row's identity in the reference and stays the
// fallback here, so a model never arrives unnamed.
func antigravityModelLabel(model antigravityModel, id string) string {
	if name := strings.TrimSpace(model.DisplayName); name != "" {
		return name
	}
	return id
}

// antigravityGeminiFamily and antigravityClaudeFamily split the published models into the two
// windows families the summary meters, on the provider's own model id. The image model is not
// part of the text session it shares a prefix with, which is why the gemini test excludes it.
func antigravityGeminiFamily(modelID string) bool {
	lowered := strings.ToLower(modelID)
	return strings.HasPrefix(lowered, "gemini-") && !strings.Contains(lowered, "image")
}

func antigravityClaudeFamily(modelID string) bool {
	return strings.HasPrefix(strings.ToLower(modelID), "claude-")
}
