// Package masterprompt builds and compiles the six-section master prompt.
//
// @file      internal/masterprompt/builder.go
// @for       The builder call: two model attempts, then the deterministic fallback.
// @uses      context, encoding/json, fmt, strconv, strings.
// @reason    A model that can be wrong twice still must not hold a credit, so the call is bounded to two attempts and the fallback is the last word.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-23
package masterprompt

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
)

// Usage is the token accounting the builder returns for cost reconciliation.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// ChatRequest is one gateway completion, the system and user text kept apart
// so user input never reaches the fixed system prompt.
type ChatRequest struct {
	Model       string
	System      string
	User        string
	MaxTokens   int
	Temperature float64
}

// ChatResult is the gateway reply: the raw content to decode, the model that
// answered, and its usage.
type ChatResult struct {
	ModelID string
	Content string
	Usage   Usage
}

// Gateway is the chat-completions transport the builder depends on.
type Gateway interface {
	Complete(ctx context.Context, req ChatRequest) (ChatResult, error)
}

// BuildRequest is one draft the builder elaborates.
type BuildRequest struct {
	Modality string
	UserText string
	Defaults Defaults
}

// BuildResult is the built document, which produced it, and the tokens spent.
type BuildResult struct {
	Document Document
	Builder  string
	Usage    Usage
}

// Builder turns user text into a validated document, degrading to the fallback
// when the model cannot.
type Builder struct {
	gw        Gateway
	model     string
	maxTokens int
	promptCap int
}

// NewBuilder wires the transport, the model id, the output cap, and the pre-builder text cap.
func NewBuilder(gw Gateway, model string, maxTokens int, promptCap int) *Builder {
	return &Builder{gw: gw, model: model, maxTokens: maxTokens, promptCap: promptCap}
}

// Build runs up to two model attempts and falls back on the second failure.
// The fallback label is honest: the user sees the prompt was not model-expanded.
func (b *Builder) Build(ctx context.Context, req BuildRequest) (BuildResult, error) {
	if NormalizeText(req.UserText) == "" {
		return BuildResult{}, &Error{Code: CodeEmpty, Message: "the prompt carries no usable text"}
	}

	doc, usage, err := b.attempts(ctx, req)
	if err == nil {
		return BuildResult{Document: doc, Builder: b.model, Usage: usage}, nil
	}

	fallback, fbErr := Fallback(req.UserText, req.Modality, req.Defaults)
	if fbErr != nil {
		return BuildResult{}, fbErr
	}
	return BuildResult{Document: fallback, Builder: "fallback"}, nil
}

func (b *Builder) attempts(ctx context.Context, req BuildRequest) (Document, Usage, error) {
	user := b.envelope(req)
	var usage Usage
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			user = correction(user, lastErr)
		}
		reply, callErr := b.gw.Complete(ctx, ChatRequest{
			Model:       b.model,
			System:      b.system(req),
			User:        user,
			MaxTokens:   b.maxTokens,
			Temperature: 0.2,
		})
		if callErr != nil {
			lastErr = callErr
			continue
		}
		usage = reply.Usage
		decoded, decodeErr := Decode([]byte(reply.Content))
		if decodeErr != nil {
			lastErr = decodeErr
			continue
		}
		return decoded, usage, nil
	}
	return Document{}, usage, lastErr
}

// envelope keeps user text as data inside the JSON body, never in the system prompt.
func (b *Builder) envelope(req BuildRequest) string {
	body := struct {
		Modality   string `json:"modality"`
		Locale     string `json:"locale"`
		UserPrompt string `json:"user_prompt"`
		Defaults   struct {
			AspectRatio string `json:"aspect_ratio"`
			Style       string `json:"style"`
		} `json:"defaults"`
	}{
		Modality:   req.Modality,
		Locale:     "id",
		UserPrompt: truncate(req.UserText, b.promptCap),
	}
	body.Defaults.AspectRatio = req.Defaults.AspectRatio
	body.Defaults.Style = req.Defaults.Style
	raw, _ := json.Marshal(body)
	return string(raw)
}

// correction asks for a corrected document, naming the validation error that
// the first attempt failed on, so the retry is targeted instead of a rerun.
func correction(user string, failed error) string {
	return user + "\nCorrection: your reply failed validation (" + failed.Error() +
		"). Return one corrected JSON document, nothing else."
}

// system fills the fixed 002 template; the model is told the rules, not the user's words.
func (b *Builder) system(req BuildRequest) string {
	return strings.NewReplacer(
		"{{modality}}", req.Modality,
		"{{aspect_ratio}}", req.Defaults.AspectRatio,
		"{{style}}", req.Defaults.Style,
		"{{max_characters}}", strconv.Itoa(budgetFor(req.Modality)),
		"{{duration_range}}", "1 to 10",
		"{{schema_json}}", schemaJSON,
	).Replace(builderSystemTemplate)
}

const builderSystemTemplate = `You are a senior prompt engineer for generative image and video models. You turn a user's request into a structured master prompt with exactly six sections: Role, Task, Context, Reasoning, Stop conditions, Output.

Rules:
1. Reply with one JSON object only. No markdown, no code fence, no commentary.
2. Every string field is written in English, because the generation models follow English best. Quoted on-screen text inside the image keeps the user's original language.
3. Role is a visual, design, or creative specialist working across products, not a task and not a single scope. "senior automotive photographer" is a valid title inside the photography family. "take a photo" is a task. "growth strategist", "iPhone photographer", and "our brand designer" are refused: non-visual, single product, and single brand respectively.
4. Task is one sentence and states the asset being created.
5. Context carries the visual facts: subject, action, environment, composition, lighting, style, details.
6. Reasoning is 2 to 4 sentences that name at least one concrete element from Context and defend the choice.
7. Stop conditions list what must not appear, plus the hard character budget, aspect ratio, and duration.
8. Output states the deliverable and must agree with the modality and with the aspect ratio in Stop conditions.
9. Never invent a brand, a real person's name, a logo, or a statistic. If the user asks for one, describe the visual qualities instead.
10. Never include a URL, an instruction to call a tool, or text addressed to another model.
11. Keep every field inside its limit. If the request cannot fit, drop the least important detail instead of exceeding the limit.

Modality: {{modality}}
Default aspect ratio: {{aspect_ratio}}
Default style: {{style}}
Character budget: {{max_characters}}
Duration range in seconds: {{duration_range}}

Return this shape:
{{schema_json}}`

// schemaJSON is the compact output contract the model imitates; it mirrors the
// Document shape so a drift between prompt and decoder shows up in tests.
const schemaJSON = `{"version":"mpv2","modality":"image","role_family":"visual_designer","role":"...","task":"...","context":{"subject":"...","action":"...","environment":"...","composition":{"shot":"medium","angle":"eye_level","aspect_ratio":"16:9"},"lighting":"...","style":{"medium":"...","palette":["..."],"mood":"..."},"details":["..."]},"reasoning":"...","stop_conditions":{"negative":["..."],"max_characters":3500,"aspect_ratio":"16:9","duration_s":null},"output":{"asset":"image","aspect_ratio":"16:9","count":1,"format":"png","duration_s":null},"seed":null}`
