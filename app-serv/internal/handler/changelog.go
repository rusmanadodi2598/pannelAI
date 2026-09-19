// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/changelog.go
// @for       The served release notes (SPEC-API-001 §7.18).
// @uses      net/http, internal/schema.
// @reason    The reference fetches its changelog from a GitHub URL in the
//
//	browser; a self-hosted panel behind the §9 egress guard has no
//	business fetching anything external, so §7.18 serves the history
//	from the binary that earned it. Entries are embedded static data:
//	the changelog describes the binary, and the binary carries it.
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

// changelogEntry is one release row. Versions follow the delivery phases of
// §10: the v0.0.1 tag is the P0 skeleton, and each later phase bumps the minor.
type changelogEntry struct {
	Version string `json:"version"`
	Date    string `json:"date"`
	Title   string `json:"title"`
	Notes   string `json:"notes"`
}

// changelogResponse is the §7.18 list envelope.
type changelogResponse struct {
	Data []changelogEntry `json:"data"`
}

// changelogEntries is newest first; the handler serves it as declared and the
// test pins that order, so a prepended entry is the only way to add one.
var changelogEntries = []changelogEntry{
	{
		Version: "v0.4.0", Date: "2026-09-20", Title: "Skills catalog, served contract, release notes",
		Notes: "GET /api/v1/skills serves the agent skill catalog (§7.16); GET /api/v1/openapi.json serves the machine-readable contract (§7.17); GET /api/v1/changelog serves these release notes (§7.18). The panel playground calls the data plane directly and adds no route (§7.15).",
	},
	{
		Version: "v0.3.0", Date: "2026-09-19", Title: "Responses API and the native token-saver engine",
		Notes: "POST /api/v1/responses serves the Responses wire as client route and upstream target; POST /api/v1/messages/count_tokens estimates tokens; the RTK, Headroom, and Ponytail savers run in the relay under 002-TOKEN-SAVER. Every saver ships off, so the request path is byte-neutral until an operator enables one.",
	},
	{
		Version: "v0.2.0", Date: "2026-09-19", Title: "OAuth, fusion, media, proxies",
		Notes: "OAuth start/callback/status/refresh with bulk credential import; the fusion combo strategy with its judge and degradations; the media data plane and per-provider overrides, including the provider-specific TTS and STT adapters; proxy pools with guarded candidate tests; the token-saver configuration surface; budget caps on /quotas.",
	},
	{
		Version: "v0.1.0", Date: "2026-09-18", Title: "Registry, endpoints and keys, chat data plane",
		Notes: "The embedded provider registry port; upstream endpoints carrying N keys each with the bulk onboarding routes; combos in fallback and round_robin; the vision adapter; chat over the OpenAI and Anthropic wires; usage, quota windows, request logs, and the settings surface.",
	},
	{
		Version: "v0.0.1", Date: "2026-09-16", Title: "Skeleton",
		Notes: "Go service skeleton, typed config, migrations, health and version, dashboard auth with Redis-backed sessions, gateway keys, and the P0 commit gates.",
	},
}

// ChangelogHandler serves GET /api/v1/changelog.
type ChangelogHandler struct{}

// NewChangelogHandler returns the release-notes handler. It has no
// dependencies: the entries are embedded static data.
func NewChangelogHandler() *ChangelogHandler { return &ChangelogHandler{} }

// List serves GET /api/v1/changelog, newest first.
func (h *ChangelogHandler) List(w http.ResponseWriter, _ *http.Request) {
	schema.WriteJSON(w, http.StatusOK, changelogResponse{Data: changelogEntries})
}
