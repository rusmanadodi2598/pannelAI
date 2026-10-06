// MiniMax: the two remaining-quota hosts one Token/Coding Plan key is checked against.
//
// @file      internal/service/quotafetch/minimax.go
// @for       Reads MiniMax's per-model session and weekly windows from the quota hosts it publishes.
// @uses      internal/service/quotafetch, context, encoding/json, math, net/http, regexp, strconv, strings, time
// @reason    MiniMax answers from two hosts whose counters mean opposite things, the token plan counts what was spent, the coding plan what is left, and its M-series buckets ship a percentage only, so both shapes must be read apart or the card renders a flipped bar.
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
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	miniMaxDisplay        = "MiniMax"
	miniMaxCodingPlanPath = "/coding_plan/remains" // the host whose counter counts what is LEFT
	miniMaxPercentWindow  = 100                    // the synthetic ceiling of a percent-only bucket
	miniMaxInvalidKeyCode = 1004                   // MiniMax's in-body answer for a key with no plan

	miniMaxInvalidKeyMessage = "MiniMax API key invalid or inactive. Use an active Token/Coding Plan key."
	miniMaxUpstreamErrorText = "Upstream quota API error"

	// Its registry entry declares both hosts under transport.usage.urls; these are the built-in for
	// a slot it leaves empty, and the two values to promote into familyEndpoints once that map can
	// carry a list rather than one URL.
	miniMaxTokenPlanURL  = "https://www.minimax.io/v1/token_plan/remains"
	miniMaxCodingPlanURL = "https://api.minimax.io/v1/api/openplatform/coding_plan/remains"
)

var miniMaxAuthPattern = regexp.MustCompile(`(?i)token plan|coding plan|invalid api key|invalid key|unauthorized|inactive`)
var miniMaxCamelPattern = regexp.MustCompile(`_([a-z])`)
var miniMaxSeparators = strings.NewReplacer("_", " ", "-", " ")
var miniMaxAcronyms = strings.NewReplacer(" To ", " to ", " Tts ", " TTS ", " Hd ", " HD ")

var miniMaxIntervals = []miniMaxInterval{
	{" (5h)", "current_interval_total_count", "current_interval_usage_count", "current_interval_remaining_percent", "remains_time", "end_time"},
	{" (7d)", "current_weekly_total_count", "current_weekly_usage_count", "current_weekly_remaining_percent", "weekly_remains_time", "weekly_end_time"},
}

type miniMaxInterval struct{ suffix, total, count, percent, remains, end string }

type miniMaxAnswer struct {
	BaseResp        miniMaxFields   `json:"base_resp"`
	BaseRespCamel   miniMaxFields   `json:"baseResp"`
	ModelRemains    []miniMaxFields `json:"model_remains"`
	ModelRemainsAlt []miniMaxFields `json:"modelRemains"`
}

// miniMaxFields is one JSON object kept as a bag of raw fields, read by name: every scalar arrives
// as a number or a quoted number, under either spelling.
type miniMaxFields map[string]json.RawMessage

// fetchMiniMax asks every host MiniMax declares in order, keeping the first answer with quota.
func fetchMiniMax(ctx context.Context, creds Credentials) Result {
	apiKey := strings.TrimSpace(creds.APIKey)
	if apiKey == "" {
		return Result{Message: miniMaxDisplay + " API key not available."}
	}

	urls, lastError := miniMaxURLs(creds.Endpoints.URLs), ""
	for index, familyURL := range urls {
		canFallback := index < len(urls)-1
		headers := bearer(apiKey, creds.UsageHeaders)
		headers["Content-Type"] = "application/json" // bearer, not the raw x-api-key the chat host takes
		response, err := requestUsage(ctx, http.MethodGet, endpointFor(familyURL, creds.Endpoint), headers, "")
		if err != nil {
			lastError = err.Error()
			if !canFallback {
				break
			}
			continue
		}

		var answer miniMaxAnswer
		if decodeErr := json.Unmarshal(response.body, &answer); decodeErr != nil {
			answer = miniMaxAnswer{} // an HTML error page carries no quota, only prose that may name a dead plan
		}
		envelope := answer.BaseResp
		if len(envelope) == 0 {
			envelope = answer.BaseRespCamel
		}
		code, message := envelope.number("status_code"), envelope.text("status_msg")
		if response.status == http.StatusUnauthorized || response.status == http.StatusForbidden ||
			code == miniMaxInvalidKeyCode || miniMaxAuthPattern.MatchString(strings.TrimSpace(message+" "+string(response.body))) {
			return Result{Message: miniMaxInvalidKeyMessage}
		}
		if response.status < 200 || response.status >= 300 {
			lastError = fmt.Sprintf("%s usage endpoint error (%d)", miniMaxDisplay, response.status)
			// Only a host that does not serve this path at all is worth a second request against.
			if canFallback && (response.status == http.StatusNotFound ||
				response.status == http.StatusMethodNotAllowed || response.status >= 500) {
				continue
			}
			return Result{Message: miniMaxDisplay + " connected. " + lastError}
		}
		if code != 0 {
			if message == "" {
				message = miniMaxUpstreamErrorText
			}
			return Result{Message: miniMaxDisplay + " connected. " + message}
		}

		models := answer.ModelRemains
		if len(models) == 0 {
			models = answer.ModelRemainsAlt
		}
		// The host being read, not the payload, decides what the counter counts.
		rows := miniMaxRows(models, strings.Contains(familyURL, miniMaxCodingPlanPath), time.Now())
		if len(rows) == 0 {
			return Result{Message: miniMaxDisplay + " connected. No quota data was returned."}
		}
		return Result{Quotas: rows}
	}

	if lastError != "" {
		lastError = ": " + lastError
	}
	return Result{Message: miniMaxDisplay + " connected. Unable to fetch usage" + lastError}
}

// miniMaxURLs is the host list MiniMax is asked, in order: every host the registry entry declares,
// and the built-in for each slot it leaves empty, so a moved host never drops the other one.
func miniMaxURLs(declared []string) []string {
	urls := make([]string, 2)
	urls[0], urls[1] = miniMaxTokenPlanURL, miniMaxCodingPlanURL
	for index, entry := range declared {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		if index < len(urls) {
			urls[index] = trimmed
			continue
		}
		urls = append(urls, trimmed)
	}
	return urls
}

// miniMaxRows renders the two windows of every model that publishes quota, under the "<name> (5h)"
// and "<name> (7d)" labels the operator reads. A percentage-only bucket is normalised onto a
// 100-wide window whose synthetic count must match what this host's counter means, or it inverts.
func miniMaxRows(models []miniMaxFields, countsLeft bool, now time.Time) []Quota {
	rows := make([]Quota, 0, len(models)*2)
	for _, model := range models {
		name := miniMaxName(model)
		for _, interval := range miniMaxIntervals {
			total := math.Max(0, model.number(interval.total))
			percent, stated := miniMaxSplit(model.value(interval.percent))
			if total <= 0 && !stated {
				continue
			}
			unit, count := "", model.number(interval.count)
			if total <= 0 {
				// Rounded after the share is taken, the way the reference rounds it.
				total, unit = miniMaxPercentWindow, "%"
				share := math.Max(0, math.Min(100, percent)) / miniMaxPercentWindow
				count = math.Round(miniMaxPercentWindow * (1 - share))
				if countsLeft {
					count = math.Round(miniMaxPercentWindow * share)
				}
			}
			used := math.Min(math.Max(0, count), total)
			if countsLeft {
				used = math.Max(0, total-count)
			}
			rows = append(rows, Quota{Label: name + interval.suffix, Used: used, Total: total, Unit: unit,
				ResetAt: model.moment(interval.remains, interval.end, now)})
		}
	}
	return rows
}

// miniMaxName labels one model's rows: the shared M-series pool gets the friendly series name the
// reference gives it (newer responses call it "general"), anything else its own id spelled out.
func miniMaxName(model miniMaxFields) string {
	switch name := model.text("model_name"); name {
	case "":
		return miniMaxDisplay
	case "MiniMax-M*", "general":
		return "M-series"
	default:
		return miniMaxTitleCase(name)
	}
}

// miniMaxTitleCase spells a model id the reference's way: spaces, capitalised words, and three
// acronyms that keep the vendor's casing rather than printing "To", "Tts" or "Hd".
func miniMaxTitleCase(name string) string {
	words := strings.Fields(miniMaxSeparators.Replace(name))
	for index, word := range words {
		first, width := utf8.DecodeRuneInString(word)
		words[index] = strings.ToUpper(string(first)) + word[width:]
	}
	return strings.TrimSpace(miniMaxAcronyms.Replace(" " + strings.Join(words, " ") + " "))
}

func (m miniMaxFields) number(key string) float64 {
	value, _ := miniMaxSplit(m.value(key))
	return value
}

func (m miniMaxFields) text(key string) string {
	return strings.Trim(strings.TrimSpace(string(m.value(key))), `"`)
}

// moment reads the countdown the host states, in milliseconds, else the absolute end.
func (m miniMaxFields) moment(remains, end string, now time.Time) time.Time {
	if left := m.number(remains); left > 0 {
		return now.Add(time.Duration(math.Round(left)) * time.Millisecond)
	}
	return parseReset(m.value(end))
}

// value reads one field under either spelling: the declared snake_case name, else its camelCase.
func (m miniMaxFields) value(key string) json.RawMessage {
	if raw := m[key]; len(raw) > 0 {
		return raw
	}
	return m[miniMaxCamelPattern.ReplaceAllStringFunc(key, func(match string) string {
		return strings.ToUpper(match[1:2])
	})]
}

// miniMaxSplit reads a scalar sent as a number or a quoted number, reporting whether the host
// stated one at all: "0% left" and "nothing stated" are different answers.
func miniMaxSplit(raw json.RawMessage) (float64, bool) {
	value, err := strconv.ParseFloat(strings.Trim(strings.TrimSpace(string(raw)), `"`), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}
