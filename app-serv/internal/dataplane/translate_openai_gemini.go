// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_openai_gemini.go
// @for       OpenAI chat request to Gemini generateContent payload translation,
//
//	and Gemini usage to the OpenAI accounting block.
//
// @uses      internal/schema, encoding/json, strings.
// @reason    SPEC-API-001 §7.15 lists Gemini among the translated formats and the
//
//	reference implements this direction as a pure function
//	(open-sse/translator/request/openai-to-gemini.js). No P1 provider
//	can reach it — every entry declaring format: gemini is reported as
//	`routability: connector` because its protocol wraps the payload in a
//	vendor envelope — so this is translation without a route, kept
//	because the registry data it is written against is already on disk
//	and a connector later needs exactly this payload.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"encoding/json"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// Gemini part discriminators. They are not wire fields — Gemini infers the part
// kind from which member is present — so they name the members a reader greps
// for rather than values on the wire.
const (
	GeminiPartFunctionCall     = "functionCall"
	GeminiPartFunctionResponse = "functionResponse"
	GeminiPartInlineData       = "inlineData"
	GeminiPartFileData         = "fileData"
)

// geminiSafetySettings disables every safety category, which is what the
// reference sends by default (formats/gemini.js DEFAULT_SAFETY_SETTINGS): a
// gateway in front of a coding agent must not have a second, invisible filter
// change an answer the client already agreed to receive.
var geminiSafetySettings = []geminiSafety{
	{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: "OFF"},
	{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: "OFF"},
	{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: "OFF"},
	{Category: "HARM_CATEGORY_HARASSMENT", Threshold: "OFF"},
	{Category: "HARM_CATEGORY_CIVIC_INTEGRITY", Threshold: "OFF"},
}

// geminiSafety is one safety setting.
type geminiSafety struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

// GeminiRequest is the generateContent payload the translator produces.
type GeminiRequest struct {
	Model             string           `json:"-"`
	Contents          []GeminiContent  `json:"contents"`
	SystemInstruction *GeminiContent   `json:"systemInstruction,omitempty"`
	GenerationConfig  *GeminiConfig    `json:"generationConfig,omitempty"`
	SafetySettings    []geminiSafety   `json:"safetySettings,omitempty"`
	Tools             []GeminiToolSets `json:"tools,omitempty"`
}

// GeminiContent is one Gemini turn. Gemini names the assistant role "model".
type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

// GeminiPart is one Gemini content part. The Type discriminator is not a wire
// field: Gemini infers the part kind from which member is present.
type GeminiPart struct {
	Text             string              `json:"text,omitempty"`
	InlineData       *GeminiInlineData   `json:"inlineData,omitempty"`
	FileData         *GeminiFileData     `json:"fileData,omitempty"`
	FunctionCall     *GeminiFunctionCall `json:"functionCall,omitempty"`
	FunctionResponse *GeminiFunctionResp `json:"functionResponse,omitempty"`
}

// GeminiInlineData is an inlined base64 payload.
type GeminiInlineData struct {
	MIMEType string `json:"mime_type"`
	Data     string `json:"data"`
}

// GeminiFileData is a remote file reference.
type GeminiFileData struct {
	FileURI  string `json:"fileUri"`
	MIMEType string `json:"mimeType"`
}

// GeminiFunctionCall is one Gemini function invocation. Its Args is an object,
// not a JSON string, which is the shape difference that matters most here.
type GeminiFunctionCall struct {
	ID   string         `json:"id,omitempty"`
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

// GeminiFunctionResp is one Gemini function result.
type GeminiFunctionResp struct {
	ID       string         `json:"id,omitempty"`
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

// GeminiConfig is the generationConfig block.
type GeminiConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	TopK            *int     `json:"topK,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
}

// GeminiToolSets is Gemini's tool wrapper, which is always an array holding one
// object with a functionDeclarations array.
type GeminiToolSets struct {
	FunctionDeclarations []GeminiFunctionDecl `json:"functionDeclarations"`
}

// GeminiFunctionDecl is one declared tool.
type GeminiFunctionDecl struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// OpenAIToGemini translates an OpenAI chat request into a Gemini
// generateContent payload.
//
// Three shape differences are load-bearing. A system prompt has no place in
// Gemini's contents, so it moves to systemInstruction (a lone system message is
// treated as the user's first turn, exactly as the reference does). A tool
// result is a functionResponse part inside a user turn, paired by call id, rather
// than its own message. And tool arguments are an object rather than a JSON
// string, so an unparseable argument string becomes an empty object instead of
// invalid JSON.
func OpenAIToGemini(req schema.ChatRequest, upstreamModel string) GeminiRequest {
	out := GeminiRequest{
		Model:          upstreamModel,
		Contents:       make([]GeminiContent, 0, len(req.Messages)),
		SafetySettings: geminiSafetySettings,
		GenerationConfig: &GeminiConfig{
			Temperature: req.Temperature,
			TopP:        req.TopP,
			TopK:        nil,
		},
	}
	if max := req.MaxOutputTokens(); max > 0 {
		out.GenerationConfig.MaxOutputTokens = &max
	}

	calls := geminiCallNames(req.Messages)
	system := make([]string, 0, 2)

	for _, message := range req.Messages {
		switch message.Role {
		case schema.RoleSystem, schema.RoleDeveloper:
			if len(req.Messages) == 1 {
				// A conversation of one system message is the user's turn in the
				// reference's reading; hoisting it would leave contents empty,
				// which Gemini rejects.
				out.Contents = append(out.Contents, GeminiContent{
					Role: RoleUser, Parts: geminiParts(message.Content),
				})
				continue
			}
			if text := message.Content.TextContent(); text != "" {
				system = append(system, text)
			}
		default:
			if content, ok := geminiTurn(message, calls); ok {
				out.Contents = append(out.Contents, content)
			}
		}
	}

	if len(system) > 0 {
		out.SystemInstruction = &GeminiContent{
			Role:  RoleUser,
			Parts: []GeminiPart{{Text: strings.Join(system, "\n")}},
		}
	}
	if declarations := geminiDeclarations(req.Tools); len(declarations) > 0 {
		out.Tools = []GeminiToolSets{{FunctionDeclarations: declarations}}
	}
	return out
}

// GeminiUsageToOpenAI folds a Gemini usageMetadata block into the OpenAI
// accounting block.
//
// candidatesTokenCount excludes the thinking tokens while OpenAI counts reasoning
// inside completion_tokens, so the two are added. The two fallbacks are the
// reference's derivations (concerns/usage.js, the `gemini` extractor) for an
// upstream that reports only some of the numbers: candidates from
// total - prompt - thoughts, and total from the sum.
func GeminiUsageToOpenAI(raw json.RawMessage) schema.Usage {
	usage, ok := decodeObject(raw)
	if !ok {
		return schema.Usage{}
	}
	prompt := intField(usage, "promptTokenCount")
	thoughts := intField(usage, "thoughtsTokenCount")
	total := intField(usage, "totalTokenCount")
	candidates := intField(usage, "candidatesTokenCount")
	if candidates == 0 && total > 0 {
		candidates = total - prompt - thoughts
		if candidates < 0 {
			candidates = 0
		}
	}
	if total == 0 {
		total = prompt + candidates + thoughts
	}
	out := schema.Usage{
		PromptTokens:     prompt,
		CompletionTokens: candidates + thoughts,
		TotalTokens:      total,
	}
	if cached := intField(usage, "cachedContentTokenCount"); cached > 0 {
		out.PromptTokensDetails = &schema.PromptTokensDetails{CachedTokens: cached}
	}
	if thoughts > 0 {
		out.CompletionTokensDetail = &schema.CompletionDetail{ReasoningTokens: thoughts}
	}
	return out
}
