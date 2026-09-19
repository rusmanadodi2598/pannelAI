// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/skills.go
// @for       The agent skill catalog route (SPEC-API-001 §7.16).
// @uses      net/http, internal/schema.
// @reason    §7.16 serves the catalog the panel's skills page renders. The
//
//	documents themselves are owner-authored content in the product
//	repository, so the gateway's whole job is the metadata and the two
//	URLs a user pastes into an agent; it never serves a SKILL.md body,
//	which keeps a capability doc from drifting from what the gateway
//	serves.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-20
package handler

import (
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// The repository the skill documents live in, and the path layout inside it.
// These are the reference's derivation (src/shared/constants/skills.js) pointed
// at this product: raw.githubusercontent.com for the agent, github.com for the
// person reading it.
const (
	skillsRepo     = "rusmanadodi2598/pannelAI"
	skillsRef      = "main"
	skillsPath     = "skills"
	skillsRawBase  = "https://raw.githubusercontent.com/" + skillsRepo + "/refs/heads/" + skillsRef + "/" + skillsPath
	skillsBlobBase = "https://github.com/" + skillsRepo + "/blob/" + skillsRef + "/" + skillsPath
)

// SkillsHandler serves GET /api/v1/skills.
type SkillsHandler struct{}

// NewSkillsHandler returns the catalog handler. It has no dependencies: the
// catalog is embedded static data, the registry rule of Locked Decision 3.
func NewSkillsHandler() *SkillsHandler { return &SkillsHandler{} }

// skillEntry is one catalog row. Endpoint is null for the entry skill, which
// indexes the others rather than teaching one.
type skillEntry struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Endpoint    *string `json:"endpoint"`
	Entry       bool    `json:"entry"`
	RawURL      string  `json:"raw_url"`
	BlobURL     string  `json:"blob_url"`
}

// skillCatalog lists one skill per capability endpoint the gateway actually
// serves (§7.15), the entry skill first. Descriptions describe this gateway's
// surface rather than copying the reference's provider lists, so the catalog
// never advertises a capability §7 does not define.
var skillCatalog = buildSkillCatalog([]struct {
	id, name, description string
	endpoint              *string
	entry                 bool
}{
	{
		id: "pannelai", name: "pannelAI (Entry)", entry: true,
		description: "Setup and index of all capabilities: base URL, auth, model discovery, and links to every capability skill.",
	},
	{
		id: "pannelai-chat", name: "Chat", endpoint: strptr("/chat/completions"),
		description: "Chat and code generation over the OpenAI chat, Anthropic messages, and Responses wire formats, streamed or not, with combo and vision-adapter routing.",
	},
	{
		id: "pannelai-image", name: "Image Generation", endpoint: strptr("/images/generations"),
		description: "Text-to-image through the image-capable media providers, normalized to the OpenAI images shape.",
	},
	{
		id: "pannelai-tts", name: "Text-to-Speech", endpoint: strptr("/audio/speech"),
		description: "Text-to-speech through the media providers' TTS formats, including the provider-specific adapters and the voice catalog at /audio/voices.",
	},
	{
		id: "pannelai-stt", name: "Speech-to-Text", endpoint: strptr("/audio/transcriptions"),
		description: "Audio transcription through the media providers' STT formats, OpenAI-shaped multipart passthrough plus the provider-specific adapters.",
	},
	{
		id: "pannelai-embeddings", name: "Embeddings", endpoint: strptr("/embeddings"),
		description: "Vectors for RAG and semantic search over the embeddings-capable providers, OpenAI embeddings shape.",
	},
	{
		id: "pannelai-web-search", name: "Web Search", endpoint: strptr("/search"),
		description: "Web search through the search-capable media providers, with each provider's declared query parameters building the payload.",
	},
})

// skillCatalogResponse is the §7.16 list envelope.
type skillCatalogResponse struct {
	Data []skillEntry `json:"data"`
}

// List serves GET /api/v1/skills with the catalog in §7.16's shape.
func (h *SkillsHandler) List(w http.ResponseWriter, _ *http.Request) {
	schema.WriteJSON(w, http.StatusOK, skillCatalogResponse{Data: skillCatalog})
}

// buildSkillCatalog derives the two repository URLs each entry needs, so a
// catalog row carries only what describes it.
func buildSkillCatalog(rows []struct {
	id, name, description string
	endpoint              *string
	entry                 bool
}) []skillEntry {
	catalog := make([]skillEntry, 0, len(rows))
	for _, row := range rows {
		catalog = append(catalog, skillEntry{
			ID: row.id, Name: row.name, Description: row.description,
			Endpoint: row.endpoint, Entry: row.entry,
			RawURL:  skillsRawBase + "/" + row.id + "/SKILL.md",
			BlobURL: skillsBlobBase + "/" + row.id + "/SKILL.md",
		})
	}
	return catalog
}

// strptr exists because the catalog rows above cannot take &"literal".
func strptr(value string) *string { return &value }
