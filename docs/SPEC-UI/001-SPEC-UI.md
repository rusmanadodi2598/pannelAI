# SPEC-UI-001: pannelAI Panel (`app-ui`) v1

|                                |                                                                                                                                                                                                                                                                                                                                                        |
| ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Spec ID**                    | 001-SPEC-UI                                                                                                                                                                                                                                                                                                                                            |
| **Status**                     | Draft to Review                                                                                                                                                                                                                                                                                                                                        |
| **Date**                       | 2026-09-16                                                                                                                                                                                                                                                                                                                                             |
| **Organization**               | KENTANG TECH                                                                                                                                                                                                                                                                                                                                           |
| **Author**                     | Dodi Rusmana <rusmanadodi@kentangtech.com>                                                                                                                                                                                                                                                                                                             |
| **GitHub**                     | https://github.com/rusmanadodi2598                                                                                                                                                                                                                                                                                                                     |
| **Source Rewrite (Reference)** | `/home/rusmanadodi/ai-gateway` (9Router, Next.js monolith)                                                                                                                                                                                                                                                                                             |
| **Target**                     | `pannelAI/app-ui/` (Svelte panel, Bun toolchain and runtime)                                                                                                                                                                                                                                                                                           |
| **Runtime**                    | Bun for dev, build, test, and the production server process (owner requirement, §10.1)                                                                                                                                                                                                                                                                 |
| **Contract of record**         | [`docs/SPEC-API/001-SPEC-API.md`](../SPEC-API/001-SPEC-API.md)                                                                                                                                                                                                                                                                                         |
| **Governance (mandatory)**     | [`docs/RULLES/TDD.md`](../RULLES/TDD.md) for the strict test-first protocol (§10.2). [`AGENTS.md`](../../AGENTS.md) is present at the repository root and scopes itself to Go services under `app-*/**`, so it does not govern this panel. Its language and typing rules are mirrored here in §7.1.5 and §8.10 so the two apps cannot quietly diverge. |
| **Companion spec**             | The machine-readable contract is served rather than filed: `GET /api/v1/openapi.json`, generated from `docs/CONTRACT/001-CONTRACT-API-V1.yaml`. See §14 Q3, closed 2026-09-21.                                                                                                                                                                         |
| **Design direction**           | `DESIGN.md` is **not** in the repository. Owner-stated direction: legacy panel parity with a fresher component layer. See §9 and §14 Q1.                                                                                                                                                                                                               |

> **SPEC FIRST.** This document is the contract of record for the panel. A screen, control, or field
> that is not written here is not built. Any deviation requires editing this spec in the same pull
> request. Wire shapes and semantics come from `docs/SPEC-API/001-SPEC-API.md` (cited as **SPEC-API §N**
> below); this document never restates a wire shape, it cites one.

---

## 1. Purpose

Give the gateway an operator panel that is a **thin, strict client** of the management API:

- `app-ui` consumes `/api/v1/*` only. It never opens a PostgreSQL or Redis connection (SPEC-API §11.5).
- Every screen exists to operate one feature the owner kept. Nothing is ported for completeness.
- The panel validates and sanitizes every input and every response at its own boundary with **Zod**
  (§7), so malformed data fails at the edge with a field-level message instead of corrupting a view.
- All copy is English (SPEC-API §2.1 `English`), including validation messages and error states.
- Work obeys [`docs/RULLES/TDD.md`](../RULLES/TDD.md), the mandatory strict test-first protocol (§10.2). [`AGENTS.md`](../../AGENTS.md) is read for cross-app consistency, and it governs the Go services rather than this panel.

## 2. Scope

### 2.1 KEEP to screen mapping

Each row is an owner decision. The reference column names the legacy surface that informed the screen,
not code to copy.

| #   | KEEP feature (owner list)                         | Panel screen                                                | API surface                                                                                        | Phase  |
| --- | ------------------------------------------------- | ----------------------------------------------------------- | -------------------------------------------------------------------------------------------------- | ------ |
| 1   | Endpoint & Key                                    | `/endpoint-keys` (Gateway keys tab, Upstream endpoints tab) | SPEC-API §7.3, §7.5                                                                                | U0, U1 |
| 2   | Providers                                         | `/providers`, `/providers/[provider_id]`                    | SPEC-API §7.4, §7.6                                                                                | U1, U2 |
| 3   | Combo & Vision Adapter                            | `/combos` (Combos tab, Vision Adapter tab)                  | SPEC-API §7.7, §7.8                                                                                | U1, U2 |
| 4   | Usage                                             | `/usage` (Overview tab, Records tab)                        | SPEC-API §7.12                                                                                     | U1     |
| 5   | Quota Tracker                                     | `/quota`                                                    | SPEC-API §7.12                                                                                     | U1, U2 |
| 6   | Token Saver                                       | `/token-saver`                                              | SPEC-API §7.9                                                                                      | U2     |
| 7   | Media Provider                                    | `/media-providers/[kind]`                                   | SPEC-API §7.10                                                                                     | U2     |
| 8   | Proxy Pools                                       | `/proxy-pools`                                              | SPEC-API §7.11                                                                                     | U2     |
| 9   | Skills (agent skill catalog)                      | `/skills`                                                   | SPEC-API §7.16                                                                                     | U3     |
| 10  | API Docs                                          | `/api-docs`                                                 | SPEC-API §7 (read-only rendering)                                                                  | U1     |
| 11  | Logs                                              | `/logs` (Requests tab)                                      | SPEC-API §7.13                                                                                     | U1     |
| 12  | Settings                                          | `/settings` (Security, Routing, Network, Logging tabs)      | SPEC-API §7.14                                                                                     | U1, U2 |
| 13  | English                                           | Cross-cutting copy contract, no screen                      | SPEC-API §4                                                                                        | U0     |
| 14  | Authentication (session)                          | `/login`                                                    | SPEC-API §7.2                                                                                      | U0     |
| 15  | Playground Chat (owner addition, 2026-09-17)      | `/playground`                                               | Data plane, SPEC-API §7.15. See §6.15 and §14 Q7.                                                  | U3     |
| 16  | Changelog (owner addition, 2026-09-17)            | `/changelog`                                                | `GET /api/v1/changelog` (§7.18) for the notes, `GET /api/v1/version` (§7.1) for the running build. | U0     |
| 17  | Console Log (split out of Logs, owner 2026-09-17) | `/console-log`                                              | SPEC-API §7.13                                                                                     | U1     |

Additions 15 to 17 are owner decisions recorded on 2026-09-17. Three consequences follow, and each is
stated here rather than left implicit:

- **Console Log is now its own route.** §6.11 keeps only the Requests tab, and §6.14 is the console
  screen. The backing endpoints are unchanged (`GET` and `DELETE /api/v1/logs/console`).
- **Playground Chat is built, and its phase is U3.** SPEC-API §10 P4 assigns the page over the data plane
  with no new gateway route. Its auth model is decided and implemented: the panel's own routes inject the
  gateway key and the browser never holds a credential (owner, 2026-09-17; built 2026-09-21). The sidebar
  row carries an `href`, and §6.15 records the model, the failure vocabulary, and the rules the screen
  holds itself to.
- **Changelog reads the release notes the gateway serves.** SPEC-API §7.18 landed the route at P4, so the
  screen reads `GET /api/v1/changelog` for the releases and `GET /api/v1/version` for the running build,
  and marks each release against it (§6.16). §14 Q11 is closed by that route rather than by a panel-side
  copy: the notes travel inside the binary, and the panel bundles none of its own.

Ledger:

- **Multi Provider.** The Providers screen renders whatever `GET /api/v1/providers` returns and holds
  no provider allowlist in panel code. A provider that exists in the embedded registry is selectable the
  same day it ships, with no panel release.
- **1 upstream endpoint, N API keys.** `/endpoint-keys` treats keys as a child collection of an endpoint
  (SPEC-API §7.5). The old 1 connection to 1 key model is not represented anywhere in the panel.

### 2.2 NOT PORTED (left out on purpose, do not spec, do not build)

Carried from SPEC-API §2.2, plus what the UI audit of the reference shows. Each exclusion has a reason.

| Excluded surface                                                                          | Reason                                                                                                                                                    |
| ----------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Basic chat page                                                                           | Not in the owner KEEP list. The data plane is consumed by CLI tools.                                                                                      |
| CLI tools config writers (18 route files under `src/app/api/cli-tools/` in the reference) | SPEC-API §2.2.                                                                                                                                            |
| Translator playground                                                                     | SPEC-API §2.2. Also a developer sandbox, not an operator screen.                                                                                          |
| MITM router, pxpipe, tunnels (Cloudflare, Tailscale)                                      | SPEC-API §2.2.                                                                                                                                            |
| Cloud sync, updater and shutdown controls                                                 | SPEC-API §2.2.                                                                                                                                            |
| MCP marketplace and MCP tool pages                                                        | SPEC-API §2.2.                                                                                                                                            |
| SAML and OIDC login pages                                                                 | SPEC-API §2.2. Password login only (SPEC-API §7.2).                                                                                                       |
| Language switcher and the i18n framework                                                  | SPEC-API §2.2 excludes the i18n framework; the KEEP item is English copy. A switcher with one locale is a dead control (R-26).                            |
| Profile page                                                                              | Not in the owner KEEP list.                                                                                                                               |
| Media providers page for web fetch                                                        | No data-plane endpoint exists for web fetch (SPEC-API §7.10 lists `search`, not fetch, and §2.2 drops anything unlisted). The nav item is **Web Search**. |
| Media combo sub-page                                                                      | No backing endpoint in SPEC-API §7.10. A screen without an endpoint cannot be built (R-26).                                                               |
| Cloud relay deployers for proxy pools (Cloudflare, Deno, Vercel)                          | SPEC-API §2.2. Self-hosted proxy rows and tests remain.                                                                                                   |
| Headroom process manager (start, stop, restart)                                           | SPEC-API §2.2 keeps only the external URL call. The panel renders configuration, not process control.                                                     |
| Stats banners, download counters, promo modals from the reference dashboard               | Presentation for a different product. Not in the KEEP list.                                                                                               |
| Interactive element without an endpoint behind it                                         | Rule of the spec: no control ships dead (R-26).                                                                                                           |

## 3. Architecture and data flow

```
Browser (app-ui, SvelteKit)
   │  fetch /api/v1/*  (session cookie, same origin)
   ▼
Reverse proxy (same origin in dev and prod: /api/v1 -> app-serv:8080)
   ▼
app-serv (Go)  ──►  PostgreSQL (state)   Redis (sessions, limits, circuit state)
```

Rules:

1. **One origin.** The panel and the API are served from the same origin so the HttpOnly session cookie
   stays `SameSite=Lax`. The panel's own Bun server hosts that origin and forwards `/api/v1/*` to
   `app-serv` in both dev and production, so a separate reverse proxy is optional rather than required
   (§10.1). This avoids credentialed cross-origin requests, which is the class of setup that silently drops
   cookies.
2. **No direct storage access.** No PostgreSQL driver, no Redis client, no connection string in the panel
   bundle. A string like `DATABASE_URL` must never appear in `app-ui`.
3. **One client module.** All HTTP lives in `src/lib/api/` (§10). Components never call `fetch` directly,
   so validation, error mapping, and drift reporting have exactly one home.
4. **Stateless shell.** Session truth comes from `GET /api/v1/auth/status` on load, not from a token in
   `localStorage`.

## 4. Stack and conventions

| Topic                 | Decision                                                                                                                                                                                                                                   |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Framework             | SvelteKit with Svelte 5 (runes), client-side rendering for panel screens. `ssr = false` on authenticated routes, so no session state is rendered server-side.                                                                              |
| Toolchain and runtime | **Bun** for install, scripts, build, tests, and the production server process; the application runs on Bun, not on Node (owner requirement). Lockfile committed. See §10.1.                                                                |
| Styling               | Tailwind CSS with the shadcn-svelte CSS variable token layer (owner decision, §13.4).                                                                                                                                                      |
| Components            | shadcn-svelte (built on Bits UI and Tailwind CSS) as the primitive layer, extended in `src/lib/components/`. Baseline primitives are a starting point, never a shipped default (§9).                                                       |
| Validation            | **`zod`**, the v4 line, latest at pin time with the exact version recorded in the lockfile, as the only strict validation and sanitization layer at the panel boundary (§7). No second validator, and no hand-rolled regex in a component. |
| Charts                | shadcn-svelte's chart component, which is built on LayerChart. One chart library only.                                                                                                                                                     |
| Icons                 | `@lucide/svelte` (the set shadcn-svelte ships with), one set, no emoji glyphs in UI strings (owner decision, §13.4).                                                                                                                       |
| Language              | English only. No i18n framework, no switcher.                                                                                                                                                                                              |
| API base              | `/api/v1`, relative path in the browser. A build-time env var may override the origin for split deployments, with the same `SameSite` caveat in §3.1.                                                                                      |
| Naming                | Panel names mirror the API: `endpoint`, `gateway_key`, `combo`, `quota_window`, `proxy`, `token_saver`. No second vocabulary in the UI layer.                                                                                              |
| IDs                   | Rendered as returned by the API (ULID with type prefix, SPEC-API §4). Truncated in tables with the full value on hover and in the detail view.                                                                                             |
| Time                  | API sends RFC3339 UTC. Display converts to the browser timezone and shows the zone label. Relative time ("3m ago") is allowed in list rows with the absolute value in a tooltip.                                                           |
| Numbers               | Token counts are integers with grouped separators. Cost arrives as a decimal string (SPEC-API §4) and is displayed as a string with 4 to 6 decimals. The panel never converts a cost to a float for display or arithmetic.                 |
| Pagination            | `page` and `per_page` per SPEC-API §4, `per_page` default 25, panel selector offers 25, 50, 100 and never exceeds 100.                                                                                                                     |
| Caching               | No service worker, no HTTP cache for management reads. Each screen refetches on mount and on explicit refresh.                                                                                                                             |
| Secret display        | Gateway key plaintext appears exactly once, in a modal that states it will not be shown again. Upstream key values and proxy passwords are write-only and render only as `key_hint`.                                                       |

## 5. Route map and navigation

### 5.1 Route table

| Route                      | Purpose                                                                  | Auth                     | Phase     |
| -------------------------- | ------------------------------------------------------------------------ | ------------------------ | --------- |
| `/login`                   | Password login (SPEC-API §7.2)                                           | Public                   | U0        |
| `/`                        | Redirects to `/endpoint-keys`                                            | Session                  | U0        |
| `/endpoint-keys`           | Gateway keys and upstream endpoints with their keys                      | Session                  | U0, U1    |
| `/providers`               | Registry list, category filter, search, status summary                   | Session                  | U1        |
| `/providers/[provider_id]` | Provider detail, model catalog, provider-scoped endpoints, OAuth actions | Session                  | U1, U2    |
| `/combos`                  | Combos and the vision adapter                                            | Session                  | U1, U2    |
| `/usage`                   | Usage overview and raw records                                           | Session                  | U1        |
| `/quota`                   | Quota windows per endpoint, budget caps                                  | Session                  | U1, U2    |
| `/token-saver`             | RTK, Headroom, Ponytail configuration                                    | Session                  | U2        |
| `/media-providers/[kind]`  | Media providers by kind                                                  | Session                  | U2        |
| `/proxy-pools`             | Proxy rows, tests, and outbound proxy settings                           | Session                  | U2        |
| `/skills`                  | Agent skill catalog served by SPEC-API §7.16, one card per row           | Session                  | U3        |
| `/logs`                    | Request logs                                                             | Session                  | U1        |
| `/console-log`             | The console ring buffer                                                  | Session                  | U1        |
| `/playground`              | Playground chat over a gateway key, injected by the panel server         | Session plus gateway key | U3, §6.15 |
| `/changelog`               | Release notes for the gateway, marked against the running build          | Session                  | U0, §6.16 |
| `/api-docs`                | API v1 reference rendered from the served OpenAPI document                | Session                  | U1        |
| `/settings`                | Security, Routing, Network, Logging                                      | Session                  | U1, U2    |
| Unknown path               | Not-found view with a link back to `/endpoint-keys`                      | Session                  | U0        |

### 5.2 Navigation contract

1. The sidebar lists the owner KEEP list plus the three additions recorded in §2.1, in five groups. A
   flat list of nineteen rows is a list, not a structure, so each group answers one question the operator
   has, and the grouping is part of the specification rather than a layout choice:

   | Group     | Question it answers                    | Items                                                                                              |
   | --------- | -------------------------------------- | -------------------------------------------------------------------------------------------------- |
   | Configure | What does the gateway route to?        | Endpoint & Key, Provider, Combo & Vision Adapter, Media Provider (with its six kinds as sub-items) |
   | Observe   | What already happened?                 | Usage, Quota Tracker, Console Log                                                                  |
   | Optimize  | What can be cheaper or reused?         | Token Saver, Skill                                                                                 |
   | Developer | How does a client talk to the gateway? | Playground Chat, API Docs, Changelog                                                               |
   | System    | How does the panel itself behave?      | Proxy Pools, Setting                                                                               |

   Group labels are the panel's own prose and appear in English only. Media Provider is a disclosure, not
   a route: it expands to Embedding, Image, Video, TTS, STT, and Web Search.

2. **Every nav item has a route in §5.1 that exists at the time the item ships.** The panel enforces
   this in code rather than by review: an item carries an `href` only when a route file exists, and an
   item without one renders as inert text with a visible `Planned` chip. A dead link is therefore not
   representable, not merely discouraged (R-24).
3. Items for later phases are visible with the `Planned` label, because the owner asked for the full
   structure up front (owner, 2026-09-17). Hiding an unbuilt screen would misrepresent the panel's scope;
   leaving it clickable would break R-24. The labelled row is the third option and the one used.
4. The header holds the theme toggle, the session identity (no profile page, so it is a label plus a
   logout action), and one global API base control that opens a dialog. The dialog states the address in
   the three forms a client is configured with (the base URL itself, a cURL line, and the two names an
   OpenAI-compatible client reads), each with its own copy control. The address is the gateway's own,
   read from the panel server (the origin of `PANEL_API_TARGET` plus `/api/v1`) rather than derived from
   the browser: `location.origin` is the panel, and it is a bind-all address like `http://0.0.0.0:3000`
   whenever the panel is opened that way, which is not an address any client can call. The read is
   session-gated, a read that fails is stated with the gateway's sentence and a way to ask again, and the
   dialog never falls back to the browser's origin, because an address that looks right and is not is
   worse than no address (R-27). The copy control sits beside the value it copies rather than in the
   header strip, because a refused clipboard write has to be answered where the value is: the panel is
   commonly opened on an origin that is not a secure context, where the clipboard API is absent, so the
   control writes through a selection-based path there and reports a failure only when both paths fail
   (R-26). No search box in the header: search is per-screen and server-side.
5. **Three responsive shapes** (owner requirement, 2026-09-17; DESIGN.md §8). Mobile is a drawer over a
   dimmed overlay, tablet is a 64px icon rail that expands on demand, and desktop is a 264px sidebar that
   collapses to the same rail. The collapsed state is remembered in a cookie, and the viewport decides
   the first visit. Every row is at least 44px tall, and the shell uses `100dvh` so mobile browser chrome
   does not clip the header (R-03).
6. The active row carries the panel's identity motif: a 3px accent marker on the leading edge, always
   paired with the label, so position is never signalled by colour alone (DESIGN.md §6).

## 6. Screen specifications

Conventions for every screen: each view has empty, loading, and error states (R-27); each table is
server-paginated; each destructive action confirms; each control maps to an endpoint in SPEC-API §7 or is
absent.

### 6.1 `/login`

- **Data:** `GET /api/v1/auth/status` on load (renders the form only when `require_login` is true),
  `POST /api/v1/auth/login`.
- **Form:** single password field. No username. Submit disabled while the request is in flight.
- **Errors:** `UNAUTHORIZED` renders "Wrong password. Try again."; `RATE_LIMITED` renders
  "Too many attempts. Try again in 15 minutes." with the retry window from the response when present.
  The lockout message states the rule (5 failures, 15 minutes, SPEC-API §7.2) rather than a vague error.
- **Post-login:** redirect to the requested route, else `/endpoint-keys`.
- **Empty state:** not applicable, the form is the only content.

### 6.2 `/endpoint-keys`

**Tab 1: Gateway keys** (SPEC-API §7.3)

- **Table:** name, `key_hint`, status, last used, request count, created. Row actions: rename, enable or
  disable, revoke.
- **Create:** a name field, then a response modal that shows the full key once with a copy control and
  the sentence: "This key is shown once. Store it now." The modal cannot be dismissed by
  clicking the backdrop; it closes with an explicit control or Escape after the copy control reports
  success or the acknowledgement checkbox is ticked. A refused copy does not open the modal up: the
  plaintext exists nowhere else, and a dismissal on a copy that did not land would lose it.
- **Constraint surfaced in the UI:** revoking is a soft delete and the row leaves the active list.
- **Empty state:** "No gateway keys yet. Create one to let a CLI tool reach the gateway."

**Tab 2: Upstream endpoints** (SPEC-API §7.5)

- **Table:** provider name, endpoint label, auth type, priority, status, key count with healthy count,
  last test result. Filters: provider, status. Sort: priority.
- **Detail drawer** for one endpoint:
  - Endpoint fields: label, priority, status. Priority edits reorder siblings, so the UI refreshes the
    list after a successful save and shows the new order rather than assuming it.
  - **Keys table:** label, `key_hint`, priority, status, last used, consecutive errors, `rate_limited_until`
    countdown. Row actions: edit (label, value, priority, status), test this key, delete.
  - **Which key would route now:** the key the router would pick, computed from priority plus health, and
    labelled as such. This is the panel's answer to "why did my request fail over", and it is derived from
    the fields the API returns, not re-simulated in the panel.
  - **Add key:** one form, plus a repeatable row mode for adding several keys in one submit loop, since
    multi-key per endpoint is the point of this screen.
- **Delete endpoint:** confirmation states that its keys are deleted with it.
- **Delete last active key:** the API returns `CONFLICT` (SPEC-API §7.5). The panel disables the delete
  control with an inline note when the endpoint has one active key and `auth_type = api_key`, and still
  handles a `CONFLICT` response if the state changed under it.
- **Test:** `POST /endpoints/{id}/test` with an optional key target. Result renders pass or fail with the
  upstream status and latency, and refreshes `test_status`.
- **Empty state:** "No upstream endpoints yet. Add a provider connection to start routing."

### 6.3 `/providers` and `/providers/[provider_id]`

**List** (SPEC-API §7.4)

- **Table:** provider name, category, auth type, endpoint count, status summary. Filters: category
  (`apikey`, `oauth`, `free`, `media`, `local`), search over name and ID.
- **Search is not built.** `ProviderListQuery` declares only `category` and `routability` and the handler
  reads no other filter, so a search box would either send a parameter the server ignores or pull the
  registry into a client-side filter, which the next bullet forbids. The control is absent rather than
  fake, and the gap is recorded in §14 Q13.
- **Pagination discipline:** the registry is a data set the API owns. The screen renders
  `GET /api/v1/providers` with the server's paging and never loads the whole registry into a client filter.
- **Custom providers (landed):** a section above the table for the two provider types the embedded registry
  cannot carry, read from `GET /api/v1/provider-nodes` and written through the same route family (§7.4). The
  set is read whole, because §7.4 returns every node and a node set is hand-configured, and it is not
  narrowed by the category filter, which filters the registry and not this set. Two controls add a node, one
  per compatible type. A row states what the registry table cannot: the prefix its models are addressed by,
  the wire shape, and the endpoint URL the gateway will call. Write actions are not repeated here; edit,
  test, and delete live on the node's own detail screen. The screen's one refresh control re-reads both this
  set and the registry (§8.6.2), and each read owns its own failure state, so a node outage does not blank
  the table. **CodeBuddy CN and CodeBuddy Int are not built here**, and cannot be: both are registry entries
  rather than nodes, and the registry's own CodeBuddy entry is hidden. The gap is recorded in
  `docs/DRAFT/011-CODEBUDDY-PROVIDER-READINESS.md`.
- **Row action:** open detail. A provider with zero endpoints shows "No endpoint configured" and links to
  the create form pre-filled with that provider.

**Detail**

- **Header:** provider name, category, auth type, transport defaults, model count summary.
- **Custom provider node (landed):** a provider whose id carries a node prefix (`openai-compatible-` or
  `anthropic-compatible-`, §7.4) is a node rather than a registry entry, and the panel tells them apart from
  the id prefix alone because the provider response carries no flag for it. The screen then states what only
  the node route holds: the model prefix, the base URL, the endpoint the gateway appends to that base URL,
  the wire format, and when the node was created and last changed. Edit, test, and delete live here. Edit
  patches name, prefix, and base URL, and states that type and api type are identity rather than offering
  them as controls that cannot act. Test probes the base URL with a credential the operator may leave empty,
  where empty means "test without a credential" rather than "send an empty one", and the probe's answer is a
  state and a latency rather than an HTTP failure. Delete asks first, and a `CONFLICT` answer (an endpoint
  still references the node) is rendered as the gateway's own sentence with the dialog still open; a
  deleted node returns to the registry list.
- **Model catalog** (SPEC-API §7.6): searchable list with capability filters (`vision`, `tools`), a
  "suggested" toggle, and per-model enable or disable state. Enable and disable write through
  `PUT /api/v1/models/disabled` (phase U2). The search and the two capability filters read
  `GET /api/v1/models/catalog` with `provider_id`, `capability`, and `q`, because that is the only route
  that accepts them. **The "suggested" toggle is not built** and the reason is recorded in §14 Q12.
- **Custom models** (U2): add and remove rows through `POST /api/v1/models/custom` and its delete route.
- **Aliases** (U2, landed): the alias to target table, edited as a full set per `PUT /api/v1/models/aliases`.
  The target picker draws from the model catalog and from combo names, because a target may be either. The
  set is global: neither route takes a provider, and `alias` is the table's primary key, so the table is the
  same on every provider's detail screen and the section says so. A stored alias whose target has since been
  disabled or deleted is rendered as it is, and because a write carries the whole set it refuses every alias
  write until that row is removed (Q21).
- **Provider-scoped endpoints:** the endpoint list filtered by `provider_id`, with the same drawer as §6.2.
- **OAuth section (U2, landed, OAuth providers only):** the section renders for a provider whose registry
  entry says `has_oauth`, and the flow `GET /api/v1/providers/{id}/oauth/status` reports decides what it
  offers: the panel starts `code` and, for `device`, `connector`, and `none`, states that flow's reason
  instead of drawing a control that cannot act. `POST /api/v1/providers/{id}/oauth/start` is called with no
  body, because the gateway derives its own callback from its base URL, and the returned authorize URL is
  rendered as a link rather than followed by the panel: a scripted navigation to a third party is a redirect
  the operator did not ask for, and a link shows the host before it is followed. The registry's one
  `has_oauth` provider declares no `authorize_url`, so the flow it reports is `device` and the start path is
  dormant until a provider declares one (Q23).
- **The callback return (U2, landed):** the gateway returns the browser to the provider detail page with
  `oauth`, `oauth_error`, and `endpoint_id` in its query. The section reads the outcome once, renders it as
  the gateway's report, and drops those three keys from the address with `replaceState`, so a reload does not
  announce a past result and a key belonging to something else is left alone. The status is read once per
  visit and re-read after a refresh or a callback return, not on a timer (Q22).
- **Connected accounts (U2, landed):** one row per OAuth endpoint with its label and id, its status, its
  expiry, its last refresh, and the token state the gateway derives from `refresh_state` (`missing`, `fresh`,
  `due`). The panel does not re-derive that classification; the row adds only what the expiry tells apart, so
  a `due` token whose expiry has passed reads as expired and one still inside its window reads as inside it.
  The manual refresh is per row, because the answer names the accounts it moved, and it re-reads the status
  without blanking the table it just moved.
- **OAuth failure states:** expired token, refresh failed, and state mismatch each have distinct copy. The
  expired token is the panel's own sentence, derived from `refresh_state: due` plus an expiry that has
  passed. A failed refresh and a failed authorization are the gateway's own sentences, attributed to it
  rather than classified by the panel, which would mean pattern-matching the gateway's English. The failed
  authorization leads with what happened and then the reason; none of the three says "error" alone.
- **Empty state (catalog):** "No models found for this filter." with a control that clears filters.

### 6.4 `/combos`

**Tab 1: Combos** (SPEC-API §7.7)

- **Table:** name, strategy, model count, sticky limit, judge model, updated. Actions: edit, test (U2,
  landed), delete.
- **Editor:**
  - Model list is ordered and drag-reorderable, each row a `ref` with a priority. The `ref` field accepts
    `provider/model`, an existing combo name, or an alias, with a picker over the catalog and combos.
    Reordering ships with two paths to the same operation: a pointer drag, and Up and Down buttons that
    work from a keyboard or a touch screen. A drag alone would be mouse-only, and this panel is used on a
    phone. Both call one function, so they cannot disagree about the result, and each move renumbers the
    priorities rather than swapping two of them, because the numbers are the stored form of the order.
    The picker's suggestions are the catalog ids, the combo names on the page, and the alias names: all
    three are sources the API resolves a ref from, and all three are reads the panel already has.
  - `strategy` select: `fallback`, `round_robin`, `fusion`. Explaining copy per strategy comes from the
    SPEC-API §7.7 semantics table, so the panel explains the same behavior the router implements:
    - `fallback`: try models in order until one succeeds.
    - `round_robin`: distribute across models, keep `sticky_limit` requests on one model first.
    - `fusion`: send to several models and let `judge_model` write the final answer.
  - `sticky_limit` renders only for `round_robin`; `judge_model` renders only for `fusion`. A field that
    the strategy ignores is hidden, not disabled.
  - Validation blocks saving a `fusion` combo with no `judge_model`, and a combo with zero models.
- **Delete:** `CONFLICT` when a combo is referenced by an alias (SPEC-API §7.7). The message names the
  alias, and the dialog adds where the fix lives: "The alias set is on any provider's detail screen, under
  Aliases." There is still **no link**, because that route takes a provider id and the refusal names none,
  so the sentence names the place rather than linking to an arbitrary provider (Q20). The dialog's lead
  sentence depends on the error code, not the message: only `CONFLICT` means the combo is still referenced,
  so another failure does not read as though it did.
- **Test (U2, landed):** each row has a Test action that opens a confirmation before sending anything. The
  panel states that the route sends one-token, non-streaming probes sequentially and that probe spend is
  not written to `usage_records`, because one click spends one upstream account at a time as an operator
  diagnostic. After confirmation, the readout has one row per stored reference, with its role (`model` or
  `judge`), the stored ref, the resolved provider/model and endpoint when the probe reached them, latency,
  and either `Answered` or the gateway's own error code and message. A failed member is a result rather than
  a route error, so one dead model does not hide the members that answered. A fusion combo's judge is last,
  and the panel labels it as a judge instead of a chain member. The readout is modal so it can hold the
  complete result beside the row that asked for it; Close stays available while the sequential request runs,
  but it does not pretend to cancel work the API cannot cancel.
- **Empty state:** "No combos yet. A combo is a model string that resolves to several upstream models."

**Tab 2: Vision Adapter** (SPEC-API §7.8)

- **Form:** enabled toggle, `round_robin` toggle, and a model multi-select limited to catalog models with
  the `vision` capability.
- **Scope note shown on screen:** v1 covers vision only. The pdf, audio-input, and video-input adapters
  from the reference are not ported, and the screen does not render controls for them.
- **Empty state:** enabled with no models selected renders a warning that image requests will not be
  adapted until at least one model is chosen.

### 6.5 `/usage`

**Tab 1: Overview** (SPEC-API §7.12)

- **Period selector:** `today`, `24h`, `7d`, `30d`, `60d`, mapped to `from` and `to`.
- **Group-by selector:** provider, model, endpoint, gateway key, mapped to `group_by`.
- **Totals:** requests, tokens in, tokens out, cache read, cache write, cost (string), error rate, p50 and
  p95 latency. Every one of these comes from `GET /api/v1/usage/summary`; the panel computes nothing it
  cannot cite, and a figure the API does not return is not displayed.
- **Chart:** requests and tokens over time from `GET /api/v1/usage/timeseries`, granularity `hour` or
  `day`. The chart has a text summary line and an accessible table fallback, so the numbers are readable
  without the graphic.
- **Cost caveat:** SPEC-API §7.12 states cost figures are estimates. The cost tile carries that note.

**Tab 2: Records**

- **Table:** timestamp, request ID, model, provider, endpoint, gateway key, tokens, cost, latency, status,
  error code. Filters: date range, status, endpoint, model, free-text `q`.
- **Row action:** open the request detail, which is `GET /api/v1/usage/records/{request_id}` joined with
  the captured log when capture is on. When capture is off, the detail view says so instead of showing an
  empty body area.
- **Empty state:** "No requests in this window." plus a link to API Docs, because the likely cause is that
  no client has called the gateway yet.

### 6.6 `/quota`

- **Data:** `GET /api/v1/quotas`, `GET /api/v1/quotas/{endpoint_id}`,
  `PUT /api/v1/quotas/{endpoint_id}` (caps, U2).
- **Table:** provider, endpoint label, window (`5h`, `daily`, `weekly`, `monthly`), used, limit, percent
  used, resets at with a countdown, source badge.
- **Source badge:** `computed` or `reported` (SPEC-API §7.12). The badge is functional, not decorative: it
  tells the operator whether the number came from local accounting or from the provider.
- **Budget caps (U2, landed):** a picker plus one form, keyed by endpoint rather than by window, because
  the collection route carries no cap and reading one per endpoint to fill a column would be an N+1
  (SPEC-API §7.12). The picker offers the endpoints the label read returned plus any endpoint the window
  table names past that list's first page. Choosing one reads its cap, so the form is seeded with the
  stored amounts and the fields stay read-only until that answer lands. Saving replaces both caps at once
  and re-reads them, and the sentence under the form is that read: "no cap is stored" is stated as a rule
  rather than as a ceiling of zero. The warning §6.6 asks for is stated where the cap is saved: an empty
  field clears that cap, and once the month-to-date spend reaches a cap the router stops picking the
  endpoint. The bounds and the degenerate zero-cost rule are the API's own, refused by the form before the
  round trip. The section renders whether or not any window exists, because a cap is legal before the
  first routed request, and an endpoint the gateway does not carry is refused with the gateway's sentence.
- **Empty state:** "No quota windows yet. Quota tracking starts after the first routed request."
- **Refresh:** the countdown ticks locally; the data refetches on an interval (§8.6).

### 6.7 `/token-saver`

- **Data:** `GET /api/v1/token-saver`, `PUT /api/v1/token-saver` (SPEC-API §7.9).
- **Sections, in this order:**
  1. **RTK:** enabled toggle and its filter list.
  2. **Headroom:** enabled toggle, external URL, `compress_user_messages` toggle. The copy states the
     service is external, is called with a 5 second timeout, and fails open, which is what SPEC-API §7.9
     specifies. No start, stop, or restart control: process management is not ported.
  3. **Ponytail:** enabled toggle and level.
- **`caveman` is DEPRECATED (owner, 2026-09-16):** this screen renders no section, no control, no label,
  and no upgrade hint for it. The API keeps the key frozen for round-trip compatibility (SPEC-API §7.9), so
  the panel's schema parses the field and ignores it. When `/api/v2` removes the key, the schema field is
  dropped with it.
- **Native engine:** reserved area labelled "Planned" that names the later spec
  (`002-TOKEN-SAVER`, per SPEC-API §12 and §10 P3). It is text, not a disabled control.
- **Per-request bypass:** the screen documents the `X-Token-Saver: off` header with a copy control, so an
  operator can hand the header to a client author (SPEC-API §4).
- **Save behavior:** partial update per section. Saving one section never rewrites another, and the form
  re-reads the full config after a successful write.

### 6.8 `/media-providers/[kind]`

- **Kinds:** `embedding`, `image`, `video`, `tts`, `stt`, `web`. `web` maps to the API kind `search`.
- **Data:** `GET /api/v1/media-providers?kind=`, `GET /api/v1/media-providers/{provider_id}`,
  `PATCH /api/v1/media-providers/{provider_id}` (SPEC-API §7.10).
- **Each kind page:** provider cards with configured endpoints, a default model selector, and a `base_url`
  field for self-hosted providers.
- **Self-hosted rule displayed on the field:** a self-hosted provider without `base_url` fails validation
  (SPEC-API §7.10). The form blocks the save with that exact message instead of letting the server reject
  it, and the server still validates.
- **No cloud fallback claim:** the screen never implies a fallback path that the API does not implement.
- **Empty state per kind:** "No provider configured for this kind." with a link to Providers.

### 6.9 `/proxy-pools`

- **Data:** SPEC-API §7.11.
- **Table:** label, protocol, host, port, username, enabled, last test state and latency. Actions: edit,
  test, delete.
- **Form:** label, protocol (`http`, `https`, `socks5`), host, port, username, password. Password is
  write-only; after save the row shows no password, only that one is set.
- **Batch add:** multi-line paste of proxy URLs, parsed in the panel, previewed as a table, then submitted
  one row at a time. Raw text is not submitted; the parsed and validated rows are (§7).
- **Test:** per row and for a candidate before saving (`POST /api/v1/proxies/test`). The result shows
  state, latency, and checked time.
- **Outbound assignment:** the global settings from SPEC-API §7.14 (`network.outbound_proxy_enabled`,
  `outbound_proxy_url`, `outbound_no_proxy`) are edited on this screen, and they are marked as global.
  Per-endpoint binding is deferred by SPEC-API §7.11 and the screen says so, so nobody hunts for a
  control that does not exist.
- **Empty state:** "No proxies yet. Add one to route upstream calls through it."

### 6.10 `/skills`

- **Purpose:** give an operator the install line for each capability skill so they can paste it into an AI
  client. The gateway serves the catalog (`GET /api/v1/skills`, SPEC-API §7.16) and never a document body,
  so every line on this screen is composed from the row's own `raw_url` rather than stored beside it.
- **Entries:** one row per capability endpoint the gateway serves, the entry skill first. The entry skill
  indexes the others; each remaining row teaches one route. There is no directory of extras, and the
  catalog is the whole list. (Amended 2026-09-21: the earlier text named two entries, `/antislop` AI and
  SuperPowers, which no route ever served. §7.16 is the catalog of record, and §14 Q2 is closed with it.)
- **Per entry:** name, one-line description, the endpoint it teaches (or the fact that it teaches none),
  the two addresses the row carries, and a copy control with the install instruction line.
- **The two addresses differ by reader.** `raw_url` is what an agent fetches and what the copy control
  hands out; `blob_url` is what a person reads, because it renders the file instead of downloading it. The
  panel renders each as what it is: the raw address as copyable text, the blob address as a link.
- **Availability is asked, not assumed.** The catalog carries a path, not a file, so the panel requests
  each `raw_url` and reports the answer. A row that answered gets its copy control and its link; a row that
  did not states the cause and keeps the address on screen, because that address names the path that has to
  be published. A copy control that copies a broken link is not shipped, which is the rule this replaces:
  the earlier text asked for an unavailable state while a source was undecided, and the source is now
  decided and checkable.
- **No invented metadata:** no download counts, no version numbers, no user ratings. There is no such data
  (R-17).

### 6.11 `/logs`

**Tab 1: Requests** (SPEC-API §7.13)

- **Table:** timestamp, request ID, gateway key, model, endpoint, status, latency, error. Filters: date
  range, status, endpoint, model, free-text `q`.
- **Detail:** the captured request and response bodies when
  `settings.logging.request_capture_enabled` is true, rendered verbatim in read-only blocks with escaping
  and a byte-size note. The viewer shows the recorded payload, including anything unexpected in it; it
  does not rewrite or prettify the stored text into something else (§7.5).
- **Capture off:** the screen states capture is off and links to Settings.
- **Purge:** `DELETE /api/v1/logs/requests`, with a confirmation that names the retention setting and
  requires the operator to type `purge` when the selected range would delete more than 1000 rows.
- **Truncation notice:** when the API truncates a body at `capture_body_max_bytes`, the viewer says so at
  the truncation point instead of ending silently.

**Tab 2: Console**

Moved to its own screen at `/console-log` (§6.14) on 2026-09-17, at the owner's request, because console
output is read while a request is being debugged and a tab behind the Requests list made it a
second-class view. The endpoints are unchanged. This section is kept as a pointer so a reader who
remembers the tab finds the new home rather than an empty heading.

### 6.12 `/api-docs`

- **Purpose:** one in-panel reference for the v1 contract, so an operator can wire a client without leaving
  the panel.
- **Content:** base URL, the session versus gateway key distinction (SPEC-API §4), the endpoint catalog
  grouped by the served document's own tags, the error code table the document carries, and one curl
  example per endpoint group.
- **Source of truth:** the screen renders `GET /api/v1/openapi.json`, the artifact SPEC-API §7.17 serves
  from the binary. A hand-written second copy of the contract is a defect (it drifts), so nothing on the
  screen restates SPEC-API: the grouping, the credential per operation, the example call, and the error
  codes are all derived from the document, and a block the document omits is stated as omitted rather than
  filled in from this spec. The document is generated from `docs/CONTRACT/001-CONTRACT-API-V1.yaml` by
  `app-serv/tools/openapi-gen`, and `TestOpenAPICoversEveryRegisteredRoute` pins it to the router in both
  directions: every pattern the mux registers is named in the document, and every route the document
  advertises is registered.
- **Copy controls:** copy base URL, copy a curl example with a `sk-...` placeholder, never a real key. The
  example is composed from the operation's own method, path, and declared credential; a path parameter is
  printed as `<id>`, so no example is a URL with literal braces in it.
- **Phases:** the screen states the document's version and the address it was read from instead of a phase
  per group. The pin above is why: a group on the screen is a route the router registers, so there is no
  built-versus-planned distinction left for a phase marker to carry. Should `app-serv` ever publish a phase
  per group, the screen renders it as one more derived field.
- **Amended 2026-09-21 (F1 of `docs/DRAFT/007-UI-ENDPOINT-READINESS.md`).** Three details changed when the
  artifact landed. The catalog is grouped by the document's tags rather than by the hand-list this section
  carried: that list named 15 groups, and the document declares 23, adding OAuth, Provider Nodes, Endpoint
  Keys, Media, Skills, API Docs, and Changelog, and splitting Usage from Quota Tracker. The error table is
  the document's `x-contract.planes` block, with each meaning taken from the response that carries that
  plane's error envelope, rather than a transcription of SPEC-API §8. And the phase bullet above replaced
  the earlier "states the phase of each endpoint group, using SPEC-API §10".

### 6.13 `/settings`

Tabs, each mapping to one group of keys in SPEC-API §7.14.

| Tab         | Fields                                                                                                                             | Notes                                                                                                                                      |
| ----------- | ---------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| Security    | `security.require_login`, `security.require_api_key`, change password, logout all sessions if the API exposes it                   | Turning `require_login` off shows a warning that the panel becomes open on the network. Change password asks for current and new password. |
| Routing     | `routing.combo_strategy`, `routing.combo_sticky_limit`, `routing.sticky_limit`                                                     | Defaults for new combos and routing; per-combo values win, and the tab says so.                                                            |
| Network     | `network.outbound_proxy_enabled`, `network.outbound_proxy_url`, `network.outbound_no_proxy`                                        | Shared surface with `/proxy-pools`; both read the same endpoint, and the tab links there instead of duplicating the form.                  |
| Logging     | `logging.request_capture_enabled`, `logging.retention_days`, `logging.capture_body_max_bytes`, `logging.observability_max_records` | Capture sets a privacy cost, so the toggle carries the warning that request bodies will be stored.                                         |
| Token Saver | Read-only summary plus a link to `/token-saver`                                                                                    | One editor for one config. A second editor is how configs drift.                                                                           |

- **Save behavior:** `PATCH /api/v1/settings` per changed field group, with a dirty indicator and a
  discard action. Secrets are never returned by the API, so no field renders an existing secret value.
- **Password rules:** current password, new password, confirm. Trimming is not applied to password fields
  (§7.2).

### 6.14 `/console-log`

- **Data:** `GET /api/v1/logs/console`, `DELETE /api/v1/logs/console` (SPEC-API §7.13).
- **View:** ring buffer lines in a monospace block, newest last, with a pause and resume control, a
  follow toggle, and a clear action.
- **Honesty rule:** the buffer is polled, because management API v1 exposes no stream (SPEC-API §7.13). The
  control is labelled "Auto refresh", not "Live", and shows the poll interval.
- **Clear:** `DELETE` removes the buffer server-side, so the confirmation names that consequence rather
  than presenting it as a local view reset.
- **Empty state:** "Console buffer is empty." plus the reason it can be empty: the gateway records console
  output only while it is running, so a restart also empties the buffer.
- **Phase:** U1, with the Requests screen it was split from.

### 6.15 `/playground`

**Status: built and wire-verified 2026-09-21, phase U3. The live pass drove the panel's own routes and
client modules against a booted gateway, including the unset-key and refused-key paths (§16). The browser
click-through is the outstanding verification: the panel is client-rendered, so the render half is covered
by jsdom tests only. §14 Q7.**

The screen sends a chat request by hand and shows what the gateway answered, so an operator can check a
route without wiring a client first.

**The authentication model is decided, and it is the reason this screen is not a security problem.** The
browser never holds a gateway key. The panel's own server performs the data-plane call, reading the
credential from the environment, so the key material never enters the page, the bundle, or browser
storage (owner decision, 2026-09-17).

| Question                               | State                                                                                                                                                                                                                                                                                                                                                                                                       |
| -------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Which API does it call?                | The data plane, SPEC-API §7.15 (`GET /api/v1/models`, `POST /api/v1/chat/completions`). No playground-specific route exists.                                                                                                                                                                                                                                                                                |
| How does it authenticate?              | **Decided and implemented.** The panel's own routes (`GET /playground/models`, `POST /playground/chat`) perform the data-plane call and add the credential on the outbound request. The browser is never given a key, and no key is stored client-side.                                                                                                                                                     |
| Why not hold a key in the browser?     | A key in the page is readable by any script on the origin, visible in devtools, and persists in storage if it is saved. The owner's rule is blunt on this point: a browser-resident credential is not safe, and the exposure is obvious. Server-side injection removes the class of problem rather than mitigating it.                                                                                      |
| Where does the key come from?          | `PANEL_PLAYGROUND_KEY` in the panel's server environment, read through `src/lib/schemas/env.ts` so a malformed value fails fast with the variable name. It is a **gateway key**, not a provider key: the gateway still does its own auth, routing, quota, and usage accounting. Unset or blank is a supported state rather than a boot failure, because a panel without a key still has every other screen. |
| What if it is unset?                   | The screen renders the unavailable state, and the sentence naming the variable is the one the panel's own route wrote: the browser bundle is not allowed to name it. It does not fall back to asking the operator to paste one, and it renders no send control, because a control that cannot send is a dead control (R-26).                                                                                |
| What does the panel server do with it? | Adds it as the `Authorization: Bearer` header on the forwarded request only, in one function, so a second call site cannot invent a second placement. It is never logged, never echoed in a response, and never sent to the browser. A gateway answer that quoted the key back is redacted before it is forwarded.                                                                                          |
| Is there a phase?                      | **Yes: U3.** SPEC-API §10 P4 assigns the page over §7.15 with no new gateway route, and the panel side is built. The sidebar row carries an `href` rather than a Planned chip.                                                                                                                                                                                                                              |
| Does the request cost money?           | Yes. It routes to a real provider, consumes quota, and writes a usage record. The composer states that next to the send control, before the operator sends one.                                                                                                                                                                                                                                             |

**What the screen renders.** A model picker fed by `GET /api/v1/models` through the panel's own route, a
message box, and a send control with the cost sentence beside it. The answer streams: the text grows frame
by frame and a Stop control aborts it. Beside the answer are the facts rule 4 asks for, each reported
rather than assumed: the resolved model, the finish reason, the token counts, and the HTTP status the
panel's route answered with. A fact the wire did not state reads as `not stated` rather than as a zero or a
blank. A disclosure at the bottom holds the frames exactly as they arrived, which is what makes a contract
divergence visible instead of mysterious. Three stream endings get three sentences, because a stream that
ended on the documented `[DONE]` sentinel, one the operator stopped, and one that closed without the
sentinel are not the same fact: the third means the answer may be cut off.

**The relay streams, and what the gateway does with that is the gateway's.** The panel hands the upstream
body through rather than reading it to the end, which the live pass proved against a gateway that writes
frame by frame: the answer grew over five reads spread across 700 ms and the stream ended on `done`. The
gateway this panel talks to today buffers its upstream answer before writing anything and glues the
`[DONE]` sentinel to the last chunk, so in practice the whole answer arrives at once and the screen
reports the stream as truncated. That sentence is the honest reading of the bytes the panel received, and
the defect is recorded as a request to `app-serv` in §16 rather than absorbed by teaching the reader to
tolerate a malformed tail.

**Failure vocabulary.** The panel's routes answer in the panel's own envelope
(`PLAYGROUND_KEY_MISSING`, `UNAUTHORIZED`, `VALIDATION_ERROR`, `GATEWAY_UNREACHABLE`,
`GATEWAY_KEY_REFUSED`, `GATEWAY_ERROR`, `INTERNAL_ERROR`) and keep the gateway's error object inside it.
The reason is the collision this design creates: both planes use the code name `UNAUTHORIZED`, so a
forwarded 401 would sign the operator out for a gateway key problem. Each code names whose failure it is,
and the gateway's own machine code stays visible beside the sentence.

**A session is required, and it is checked by the gateway.** Every playground route first asks
`GET /api/v1/auth/status` with the caller's cookie, so the session's truth stays in `app-serv`'s store and
an unauthenticated browser cannot spend the panel's key. The key check comes first, so an unconfigured
panel dials nothing at all.

Rules the implemented screen must satisfy:

1. **No credential in the browser, at any point.** Not in the bundle, not in `localStorage`, not in a
   cookie readable by script, not in a form field, not in a URL.
2. **The panel server is the only holder.** The key is read once at boot through the typed config and used
   only on the outbound header. A request that would echo it back returns the gateway's own error instead.
3. **The injection is not a new auth path for the panel.** Playground calls do not grant management access;
   they are ordinary data-plane calls, so a compromised playground request is bounded by the gateway key's
   own scope.
4. **The screen states the cost before the send**, and shows the resolved model and the upstream status so
   a failure is diagnosable rather than mysterious.

**How the rules are held.** Rule 1 by a source-location guard (the variable may only be named by the schema
that declares it and the server modules that read it, and the browser's own files may not name a store or a
credential header) plus a build-time grep of the client bundle. Rule 2 by a test that the credential is
written in exactly one function and by a redaction guard on forwarded sentences. Rule 3 by the session
check above. Rule 4 by the composer's cost sentence and the answer's facts row. The render half is covered
by jsdom tests and the wire half by the live pass recorded in §16.

### 6.16 `/changelog`

**Status: screen built 2026-09-17, reads the served release notes since 2026-09-21. §14 Q11 is closed.**

The screen answers one question: what changed, and is this build behind? The order follows that question
rather than the usual changelog order, because an operator comes here while diagnosing a change in
behaviour, not to browse history.

**Header**

- **Running version** is the focal point, read from `GET /api/v1/version` (SPEC-API §7.1), which reports
  `version`, `commit`, `build_date`, `go_version`, and `registry_revision`. The figure is real rather than
  a placeholder (R-17), and before the read answers the slot says it is reading rather than claiming the
  version is unknown, because "unknown" is a fact about a read that finished.
- **Status line** states either how many releases are newer than this build, or that this build is the
  newest listed. A running version that cannot be compared gets one of two sentences rather than a count of
  zero: the read failed, or the gateway reports a value that is not a release number. Neither is "up to
  date", and the second is the case a `-dev` build produces.

**Release list**

- The releases come from `GET /api/v1/changelog` (SPEC-API §7.18), which the binary serves from the history
  it was built with.
- Newest first. Two releases on the same day are ordered by version, so the list does not reorder itself
  between two reads, which would read as a bug.
- Each release carries its version verbatim as the gateway reports it, its date, its title, and its note.
  The version string is never rewritten, and the date is the gateway's own calendar date rendered as it was
  sent: a date has no zone, and a zoned timestamp formatter would print a day the gateway never reported.
- **Status marker per release:** `Running` for the build in use, `Newer` for a release ahead of it,
  `Installed` for one behind it. When the running version cannot be compared, **no release is marked**: "I
  could not place your version" is a different statement from "you are behind on all of these", and the
  second one is confidently wrong in a way that would tell an operator to upgrade something they already
  run. The marker is the 3px identity motif on the card's leading edge, paired with the chip text beside it,
  so the state is never carried by the colour alone.
- The title and the note are the gateway's own words, rendered as one block. The panel does not split a note
  into bullets and does not invent a category: the served entry has neither, and a split would attribute a
  structure to a text nobody wrote it in.

**States**

- **Empty:** "The gateway serves no release notes", naming the route that answered with an empty list. The
  notes travel inside the binary, so an empty list is the gateway reporting its own history as empty rather
  than a source the panel is missing.
- **Loading:** a skeleton matching the final layout.
- **Error:** the release read failed, with the gateway's own message and a retry. A version read that failed
  does not hide the list: the releases are worth reading without a version, and the status line says the
  comparison is unavailable.

**Why the screen does not bundle a release file.** A bundled `CHANGELOG.md` would drift from the running
gateway and would be the hand-written second copy that §6.12 already calls a defect for the API contract:
it goes stale and then it lies. §14 Q11 chose the served route for that reason, so the screen renders what
the gateway answers and holds no release text of its own.

**Copy:** `src/lib/strings/changelog.ts`. English only, no em dash (R-02), no marketing vocabulary (R-16),
and no figure the source did not provide.

## 7. Validation and sanitization (Zod)

Zod is the panel's only validation and sanitization mechanism (owner decision, §13.5). It runs on four
boundaries:

1. **Form input**, parsed on submit and on blur, with field-level issues rendered next to the field.
2. **Route and search parameters**, parsed before any fetch is issued, so a hand-edited URL cannot reach
   the API.
3. **API responses**, parsed before any value reaches a component, so a contract change is reported instead
   of rendered.
4. **Panel environment configuration**, parsed at build time, so a missing or malformed base URL fails the
   build instead of producing silent `undefined` requests at runtime.

One package, one version, one direction: schemas are authored once in `src/lib/schemas/` and imported by
the API client, by forms, and by route loaders. There is no second validation path in the panel.

### 7.1 Role and limits

1. **Strict by default on input.** Form schemas parse with `.strict()` semantics: an unknown key is a
   validation failure, not a silent drop. Route params and search params are parsed the same way, so a
   hand-edited URL with `?per_page=100000` fails into the clamped value with a visible notice.
2. **Responses are validated, not trusted.** Every response body is parsed by a response schema before it
   reaches a component. A shape mismatch is surfaced as a screen-level error with the offending path, not
   as `undefined` rendered into a table cell.
3. **The panel is not a security boundary.** Zod here protects the view and the operator's intent.
   `app-serv` remains authoritative and re-validates every request (SPEC-API §4, §8). The panel never
   claims to sanitize for security; it claims to validate for correctness. Config values are the API's
   business to enforce.
4. **One schema module.** Schemas live in `src/lib/schemas/`, one file per API resource, exported as
   `schemaXxx` plus `type Xxx = z.infer<typeof schemaXxx>`. A form schema and a response schema may differ,
   but the field list is shared and a drift test asserts that the form schema's keys are a subset of the
   response schema's keys.
5. **No `any` at the boundary.** The only place untyped data exists is the moment before `parse`. After
   that, every value crossing into a component is a typed inference. This matches the no-`any` rule in
   SPEC-API §9.3 for the Go side, applied to the panel.
6. **Version policy.** The panel takes the latest published Zod at install time and pins the exact version
   in the lockfile. The version is recorded in `app-ui/README.md` when it changes, and a major upgrade is
   its own pull request, because schema semantics are the panel's contract surface.
7. **No ad hoc validation.** A component may not call `trim`, run a regex, or test a shape on its own. If a
   rule is needed, it is added to a schema in `src/lib/schemas/` and reused, so one rule has one home and
   one message.
8. **Messages live in the schema.** Each check carries its English message where the check is declared, so
   the same field produces the same wording on every screen that edits it. Components render the message,
   they do not compose it. Message style follows §8.10.

### 7.2 Sanitization rules

Sanitization normalizes input into the shape the API expects. It never changes meaning, and it never edits
what the API returned (see §7.5).

| Field class                                                                | Rule                                                                                                                                                                                                                                            |
| -------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Display labels (endpoint label, gateway key name, combo name, proxy label) | Unicode NFC, trimmed, internal whitespace runs collapsed to one space, control characters removed (`U+0000` to `U+001F`, `U+007F`), max length 120 characters. Angle brackets are rejected with a message that markup is not allowed in labels. |
| Model refs and aliases                                                     | Trimmed, no whitespace inside a segment, must match `provider/model`, a known combo name, or a known alias. The picker is the normal path; typing is validated against the same schema.                                                         |
| Gateway key input                                                          | Must start with `sk-` and match the API's documented pattern; spaces and newlines stripped (a pasted key often carries a trailing newline).                                                                                                     |
| Upstream key value                                                         | Trimmed of surrounding whitespace only; length bounded; the value is never echoed back into the DOM after submit.                                                                                                                               |
| Proxy host                                                                 | Lowercased, no scheme, no path, no port, valid hostname or IPv4 or IPv6 literal; max 253 characters.                                                                                                                                            |
| Proxy port                                                                 | Integer, 1 to 65535, from a string or a number. `0`, negatives, and non-integers fail.                                                                                                                                                          |
| Proxy URL (batch paste)                                                    | Parsed by the panel into protocol, host, port, username, password; the preview table shows the parse result before submit, and unparsable lines are listed as rejected with a line number.                                                      |
| `base_url` (self-hosted media provider)                                    | Absolute URL, scheme `http` or `https`, no trailing slash, no whitespace.                                                                                                                                                                       |
| `outbound_no_proxy`                                                        | Comma-separated host list, trimmed, duplicates removed, empty entries dropped.                                                                                                                                                                  |
| Timestamps                                                                 | RFC3339 only. A value that does not parse renders as "Invalid timestamp" with the raw string in a tooltip, never as "Invalid Date" from a native date call.                                                                                     |
| Cost amounts                                                               | Math string pattern only (digits with an optional decimal part), no sign, no exponent. Displayed verbatim as a string.                                                                                                                          |
| Token counts                                                               | Non-negative integers. A fractional or negative value fails the response schema, which is a contract bug worth seeing.                                                                                                                          |
| ULIDs and prefixed IDs                                                     | Validated against the prefix set in SPEC-API §4 (`ep_`, `uky_`, `gky_`, `cmb_`, `prx_`), so a wrong-ID navigation fails fast instead of issuing a doomed request.                                                                               |
| Search and filter text                                                     | Trimmed, control characters removed, max 200 characters, passed as a query value (never concatenated into a path).                                                                                                                              |
| Numeric pagination                                                         | `page` at least 1, `per_page` in 1 to 100, clamped with a visible notice when a URL says otherwise.                                                                                                                                             |
| Password fields                                                            | Length bounded only, from 1 to 200 characters. Never trimmed, never normalized, never logged, never stored.                                                                                                                                     |

### 7.3 Module layout and reuse

```
src/lib/schemas/
  primitives.ts     # label, key, host, port, absolute URL, RFC3339, cost, token count, prefixed ID
  sanitize.ts       # named transforms: normalizeLabel, stripControlChars, normalizeHost, normalizeNoProxyList
  error.ts          # the management error envelope from SPEC-API §8, parsed like any other resource
  env.ts            # panel environment: API base URL, build mode
  <resource>.ts     # one file per API resource, exporting the response schema and its inferred type
  forms/<resource>.ts # form schema for the same resource, sharing primitives and sanitize transforms
```

Rules:

1. **Sanitization is a Zod transform, not a helper called from a component.** A normalization such as
   label trimming is written once as a transform in `sanitize.ts` and composed into every schema that has
   such a field. This is what makes "sanitized everywhere" a property of the schema rather than a habit.
2. **Shared primitives.** A field class from §7.2 is one exported primitive. Two screens editing the same
   field class cannot disagree, because there is only one definition to import.
3. **Response schema and form schema share field names.** They may differ in strictness, since a form is
   stricter on input than a response is on unknown keys, but a drift test asserts that every form field
   name exists in the response schema (§7.6.5).
4. **Environment schema.** `env.ts` parses the public base URL and build mode. A malformed value fails the
   build with the variable name and the reason, which is the panel-side equivalent of the typed config
   requirement in SPEC-API §9.3.
5. **Types are inferred, never duplicated.** `type Endpoint = z.infer<typeof schemaEndpoint>` is the only
   place the shape is written down. A hand-written interface that mirrors a schema is a defect.

### 7.4 Response parsing and drift

1. `parseResponse(schema, payload)` returns the parsed data or a typed failure carrying the Zod issue path,
   which the screen shows as "Unexpected response from the gateway at `<path>`" with a refresh action.
2. Response schemas tolerate **additive** fields: an unknown key in a response is not an error, because
   `app-serv` must be able to add a field without breaking the panel. Unknown keys are collected and,
   in development, logged once per schema.
3. Response schemas are strict about **required** fields, types, and enums. A renamed field, a changed
   type, or a new enum member the panel does not know is an error, which is exactly the drift the project
   gates are supposed to catch.
4. The collected drift notes feed the API drift gate in `scrypts/` (planned, §11.5) so drift is reported
   as a build artifact rather than discovered by an operator.

### 7.5 Where sanitization must not happen

1. **Captured request and response bodies in `/logs`** render verbatim in an escaped, read-only block
   (§6.11). Rewriting stored payloads would make the audit view lie about what the gateway recorded.
   Escaping is a rendering concern (text nodes only, no raw HTML), not a data rewrite.
2. **Console buffer lines** render verbatim, escaped, in a monospace block.
3. **Error messages from the API** are displayed for the operator's troubleshooting, with the panel's own
   copy added around them, never instead of them.
4. **No `{@html}` with API data or user input.** The panel has no raw HTML sink. If a payload needs
   highlighting, it is tokenized into text nodes.
5. **No client-side "fixing" of server data.** If a value fails validation, the panel reports it. It does
   not coerce a bad value into a plausible one.

### 7.6 Testing (per `docs/RULLES/TDD.md`)

1. Every schema gets a table-driven test with at least five cases: a typical value, a boundary value, an
   empty value, a negative or invalid value, and an extreme value. A single-case test per schema is a
   protocol violation under the project TDD rules.
2. Sanitization tests assert the transformation, not just acceptance: collapsed whitespace, stripped
   control characters, rejected markup, trimmed pasted keys.
3. Response-schema tests include an additive-field case (must pass) and a renamed-field case (must fail).
4. Tests run through Vitest with `bun test` as the documented entry point.
5. Every schema file listed in §7.7 has a matching test file. A resource with no schema test is a gap the
   drift gate reports.
6. The protocol order in §10.2 is mandatory for schema and client work: analysis first, then the failing
   test, then the implementation, then the compliance self-check, as written in
   [`docs/RULLES/TDD.md`](../RULLES/TDD.md) §3. A schema written before its test is discarded and
   restarted from the analysis, the same treatment the protocol gives a hardcoded implementation.

### 7.7 Schema coverage checklist

The drift gate in `scrypts/` (planned, §11.5) uses this table: each row must exist as a schema, and each
schema must have a table-driven test.

| Schema file         | Covers                                                                     | API reference  |
| ------------------- | -------------------------------------------------------------------------- | -------------- |
| `auth.ts`           | login request, auth status, change password                                | SPEC-API §7.2  |
| `gateway_key.ts`    | gateway key list, create response with one-time plaintext, patch           | SPEC-API §7.3  |
| `provider.ts`       | registry list and detail, category and auth type enums                     | SPEC-API §7.4  |
| `endpoint.ts`       | upstream endpoint list, create, patch, delete result                       | SPEC-API §7.5  |
| `upstream_key.ts`   | key list with health fields, create, patch with write-only value           | SPEC-API §7.5  |
| `model.ts`          | catalog entries, custom models, disabled set                               | SPEC-API §7.6  |
| `model_alias.ts`    | alias to target set                                                        | SPEC-API §7.6  |
| `combo.ts`          | combo list and detail, strategy enum, ordered model refs                   | SPEC-API §7.7  |
| `vision_adapter.ts` | adapter config and its vision-capable model list                           | SPEC-API §7.8  |
| `token_saver.ts`    | RTK, headroom, ponytail; `caveman` parsed as DEPRECATED and never rendered | SPEC-API §7.9  |
| `media_provider.ts` | media provider list and detail, kind enum, base URL                        | SPEC-API §7.10 |
| `proxy.ts`          | proxy list and detail, protocol enum, write-only password                  | SPEC-API §7.11 |
| `usage.ts`          | summary, timeseries buckets, records, single record detail                 | SPEC-API §7.12 |
| `quota.ts`          | quota windows, source enum                                                 | SPEC-API §7.12 |
| `quota-cap.ts`      | budget cap read shape, whole-set write body, form rules                    | SPEC-API §7.12 |
| `request_log.ts`    | request log rows, captured bodies, console lines                           | SPEC-API §7.13 |
| `settings.ts`       | every settings group in SPEC-API §7.14, secrets excluded                   | SPEC-API §7.14 |
| `error.ts`          | the management error envelope and the code enum                            | SPEC-API §8    |

## 8. Cross-cutting UI behavior

### 8.1 Session lifecycle

1. `GET /api/v1/auth/status` runs on shell mount. `require_login = false` skips the login screen.
2. Any management response with `UNAUTHORIZED` clears panel state and routes to `/login` with the current
   path preserved, then returns to it after login.
3. Logout calls `POST /api/v1/auth/logout`, clears state, and routes to `/login`. No token is cached
   anywhere.

### 8.2 Error mapping

The API error envelope and codes come from SPEC-API §8. Each code has one panel treatment, one message
voice, and a recovery action where one exists.

| Code                                 | Panel treatment                                                                                                          |
| ------------------------------------ | ------------------------------------------------------------------------------------------------------------------------ |
| `VALIDATION_ERROR`                   | Field-level messages when the server names fields; otherwise a form-level banner. Never a raw JSON dump.                 |
| `UNAUTHORIZED`                       | Session flow in §8.1. For a data-plane path shown in docs, the copy explains the gateway key header instead.             |
| `FORBIDDEN`                          | Banner with the action that was refused and no retry control.                                                            |
| `NOT_FOUND`                          | Empty state with a "Back to list" action, plus the resource ID that was requested.                                       |
| `CONFLICT`                           | Inline conflict message that names the conflicting object (last active key, alias referencing a combo, duplicate label). |
| `RATE_LIMITED`                       | Banner with the retry window and a disabled submit until it elapses.                                                     |
| `NO_PROVIDER_AVAILABLE`              | Read-only informational state: no routing candidate is healthy. Shown where routing health is visible.                   |
| `UPSTREAM_ERROR`, `UPSTREAM_TIMEOUT` | Test-result and log-detail contexts only; a red banner that states which upstream failed.                                |
| `INTERNAL_ERROR`                     | Banner that shows the `request_id` with a copy control, because that is what an operator hands to a log search.          |

Message rules: English, name the field or object, state the fix, no em dash, no buzzwords (R-02, R-16).

### 8.3 UI states

Every data view implements three states, and they are part of the screen definition, not extra work:
empty (with why it is empty and the action that fills it), loading (skeleton matching the final layout, so
the page does not jump), and error (with the recovery action from §8.2). A view that only handles success
does not pass review.

### 8.4 Tables, filters, forms

1. Server-side pagination and filtering on every list. Client-side filtering of a full data set is
   forbidden, because it either hides rows behind a page boundary or loads everything.
2. Filters live in the URL as search params, so a filtered view is shareable and back or forward navigation
   restores it.
3. Form submit is disabled while in flight. Double submit is not possible.
4. Dirty state warns before leaving a form with unsaved changes. The guard lives once, in the root
   layout, and each form that holds a server-confirmed draft registers its own dirty state with it
   (draft 007, F10). A navigation inside the panel is confirmed by the panel; a navigation that
   unloads the document is confirmed by the browser.
5. Multi-field forms validate on submit, and the operator reads the schema's own message for the field
   the schema names. **Amended 2026-09-21** (owner decision D5 of draft 007): the panel does not
   validate on blur. Every form parses its whole draft against its Zod schema in the submit handler,
   so a half-typed value is never reported, which is the purpose this rule states; blur validation
   would add a touched-field state and a field-to-issue map to every form to say what the submit
   already says. Three live messages stay live, and none of them is schema validation: the Headroom
   warning states a combination the API accepts, the purge confirmation follows the typing because
   the phrase is what enables the control, and the batch add shows the lines it parsed before any
   write.

### 8.5 Destructive actions

| Action                                            | Confirmation                                                                                                                      |
| ------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| Revoke gateway key                                | Modal naming the key and its `key_hint`. States the consequence: clients using it stop working.                                   |
| Delete upstream endpoint                          | Modal stating that its keys are deleted with it. Requires the endpoint label to be typed when the endpoint has more than one key. |
| Delete upstream key                               | Modal naming the key. Blocked in the UI when it is the last active key (server still enforces).                                   |
| Delete combo, proxy, custom model, media override | Modal naming the object.                                                                                                          |
| Purge logs                                        | Modal stating the range and row count, with typed `purge` confirmation over 1000 rows.                                            |
| Disable login requirement                         | Modal stating that the panel becomes reachable without a password on the network.                                                 |

### 8.6 Refresh and polling

1. Polling intervals are visible, pausable, and stop when the tab is hidden. `/logs` console and `/quota`
   use polling because management API v1 has no stream (SPEC-API §7.13).
2. Every list view has an explicit refresh control, because operators distrust auto-refresh they cannot
   trigger.
3. After a write, the panel re-reads the affected resource instead of patching local state, so the view
   reflects what the gateway actually stored.

### 8.7 Responsive and input

1. **Three shapes, not two** (owner requirement, 2026-09-17; DESIGN.md §8). The breakpoints are mobile
   under 768px, tablet 768px to 1023px, and desktop 1024px and up:

   | Band    | Shell                                       | Notes                                                                                                                                                     |
   | ------- | ------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
   | Mobile  | Off-canvas drawer over a dimmed overlay     | Opens from the header control, closes on Escape, on the overlay, on navigation, and on the close control. Safe-area insets pad the drawer and the header. |
   | Tablet  | 64px icon rail, expandable to full width    | This band previously received a drawer, which wasted the space a tablet actually has. The rail keeps navigation reachable while a form is open.           |
   | Desktop | 264px sidebar, collapsible to the same rail | The collapsed state is remembered in a cookie.                                                                                                            |

2. Tables become stacked key-value rows under 768px, or scroll horizontally inside their own container
   when a comparison layout is essential; the page body never scrolls horizontally (R-03).
3. Tap targets are at least 44px on touch layouts, including the control that opens the mobile drawer.
4. The shell height is `100dvh`, not `100vh`: `100vh` is taller than the visible area under mobile browser
   chrome, which clips a sticky header on a phone. Only the content column scrolls.
5. No hover-only information: every tooltip value is present in the detail view or a table column. On the
   tablet rail, a collapsed row's label is available as a tooltip and the row keeps an accessible name,
   so collapsing the sidebar never removes the label from a screen reader.

### 8.8 Accessibility

1. Contrast meets WCAG AA: 4.5:1 for body text, 3:1 for large text, measured per theme (§9.4). Contrast is
   measured on the real rendered pair, not assumed from token names.
2. Full keyboard operation: logical tab order, Enter or Space activation, Escape closes dialogs, focus
   returns to the invoking control after a dialog closes, and focus is trapped inside a modal.
3. No `outline: none` without a replacement focus indicator that is visible in both themes.
4. Toasts and validation summaries announce through a live region.
5. Form errors are linked to their inputs, not just colour-coded.

### 8.9 Theme

1. Light and dark are both shipped, with a header toggle. Both are verified before a screen ships (R-34).
2. Theme choice follows the system on first load and is remembered after an explicit choice.
3. Dark is a legitimate default for a developer-facing tool (R-21), and it is not delivered by inverting
   the light theme: each theme's tokens are authored.

### 8.10 Copy contract

1. English only, no i18n framework, one strings module per screen for the panel's own prose.
2. No em dash in any UI string (R-02). Use a comma, period, colon, or parentheses.
3. No marketing vocabulary: no "AI Powered", "Seamless", "Powerful", "Effortless", "Next Generation"
   (R-16). Operator-facing text states what a control does.
4. No invented numbers, uptime figures, user counts, or compliance claims anywhere in the panel (R-17,
   R-36).
5. Button labels name the action and its object: "Create gateway key", "Test this key", "Purge captured
   logs". "Get Started", "Learn More", and "Try Now" do not appear (R-15).
6. Empty-state copy explains why the area is empty and what fills it, not "No data".

### 8.11 Icons and no-emoji rule

1. One icon set: `@lucide/svelte`, the set the component layer ships with. Mixing sets is forbidden. The
   older `lucide-svelte` package is deprecated on npm in favour of `@lucide/svelte` and is no longer a
   dependency.
2. Every icon used in navigation and section headers is recorded in one icon map file with a one-line
   reason for the choice (R-04, R-31). An icon with no writable reason is removed, not decorated. The file
   is `src/lib/icons.ts`, and it is the only place an icon name appears.
3. **Sidebar map (owner requirement: fresh shadcn icons).** The sidebar takes its icons from the same set
   the component layer ships with, so the panel has one visual language. Each row states what the icon has
   to communicate. The identifier column names the shape, and the assertion that it resolves lives in
   `tests/navigation/icons.test.ts`, which fails the build when a name does not exist in the installed
   package, so no guessed name can reach the build.

| Nav item                                     | Icon has to communicate         | Identifier            | Reason                                                                                          |
| -------------------------------------------- | ------------------------------- | --------------------- | ----------------------------------------------------------------------------------------------- |
| Endpoint & Key                               | A credential                    | `key-round`           | The screen is about keys, client-facing and upstream.                                           |
| Provider                                     | An upstream machine             | `server`              | A provider is a remote host the gateway calls.                                                  |
| Combo & Vision Adapter                       | Ordered stacking                | `layers`              | A combo is an ordered list of models tried in sequence.                                         |
| Media Provider                               | A non-text model                | `image`               | The group covers non-text models, and image is the kind most operators configure first.         |
| Media kind: Embedding                        | Vectors                         | `binary`              | Embeddings are numeric vectors.                                                                 |
| Media kind: Image                            | Image generation                | `image`               | Direct match to the kind.                                                                       |
| Media kind: Video                            | Moving image                    | `film`                | Direct match to the kind.                                                                       |
| Media kind: TTS                              | Speech output                   | `volume-2`            | The provider produces audio.                                                                    |
| Media kind: STT                              | Speech input                    | `mic`                 | The provider consumes audio.                                                                    |
| Media kind: Web Search                       | Retrieval                       | `search`              | The kind is a query against the web.                                                            |
| Usage                                        | Consumption over time           | `chart-line`          | Usage is a time series, not a single number.                                                    |
| Quota Tracker                                | A limit with a remaining amount | `gauge`               | Quota is headroom against a cap, which a gauge shows at a glance.                               |
| Console Log                                  | Captured terminal output        | `terminal-square`     | The buffer is terminal output, not a written audit trail, so it is a different glyph from Logs. |
| Token Saver                                  | Trimming                        | `scissors`            | The savers cut tokens out of a request before it is sent.                                       |
| Skill                                        | Assembled pieces                | `blocks`              | A skill is a reusable instruction block handed to a client.                                     |
| Playground Chat                              | Sending a request by hand       | `message-square-code` | The screen sends a chat request by hand to see what the gateway answers.                        |
| API Docs                                     | A reference to read             | `book-open`           | The screen is documentation, not an action.                                                     |
| Changelog                                    | A record ordered by time        | `history`             | A changelog is a record read backwards through time.                                            |
| Proxy Pools                                  | A routing path                  | `network`             | Proxies are intermediate hops in the outbound path.                                             |
| Setting                                      | Configuration                   | `settings`            | Direct match to the screen.                                                                     |
| Request logs (section header, not a nav row) | A written record                | `scroll-text`         | Request logs are records read after the fact.                                                   |

4. No emoji anywhere in UI strings, buttons, empty states, or headings (owner decision, §13.4). Unicode
   emoji are also excluded from code comments and commit prose in `app-ui`.
5. No sparkle, star, lightning, robot, or orb glyphs as feature icons (R-04).
6. Status is never carried by colour alone: a status chip pairs colour with a text label and, where a
   glyph helps, an icon whose meaning is written in the icon map.
7. No custom SVG is drawn for navigation. If the installed set has no icon that fits, the item ships with
   a text label only, which is better than a glyph that means nothing (R-04).
8. The icon map holds one entry per icon plus the nav item it serves, so a reviewer can check relevance
   without opening the sidebar component.

## 9. Design direction and antislop binding

antislop is applied to this project in both modes: **during** work and as an **after** audit. This section
records what is fixed by the rules versus what needs the owner's direction.

### 9.1 Direction status

| Item                                      | State                                                                                                                                                                                                                   |
| ----------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Owner-stated direction                    | Legacy panel parity in structure and density, delivered fresh: a cleaner component layer (shadcn-svelte), fresh icons, no emoji, and explicit mobile, tablet, and desktop behaviour (owner, 2026-09-16 and 2026-09-17). |
| `DESIGN.md`                               | **Present at the repository root** (added 2026-09-17). It holds the identity, the palette, the typeface, the radius and elevation scale, the identity motif, and the reason log.                                        |
| Palette, typography, logo, identity motif | Supplied through `DESIGN.md`. The logo was supplied by the owner at `app-ui/assets/static/logo.png` on 2026-09-17 and is cropped to a circular mark for the panel.                                                      |

Consequence: the earlier "draft without direction" state is closed. A screen built from this spec and
`DESIGN.md` is a deliverable, and `DESIGN.md` is where a visual value is decided. Where the two files
disagree about a visual value, `DESIGN.md` wins; where they disagree about a behaviour, an endpoint, or a
route, this spec wins.

The palette decision worth recording here, because it differs from the legacy panel on purpose: the
legacy coral `#E56A4A` measures 3.23:1 as a button fill with white text, below the AA floor. The light
theme therefore uses the same hue one step darker, and the dark theme keeps the exact legacy coral with a
near-black label. `DESIGN.md` §3.1 and §3.3 carry the measured ratios.

### 9.2 Dials

| Dial   | Value | Reason                                                                                                                                                                                                                                                         |
| ------ | ----- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| ENERGY | 1     | An instrument states its values and stops. Density is the point, not decoration.                                                                                                                                                                               |
| RHYTHM | 2     | The shell is deliberately uniform so muscle memory holds across screens, while the sidebar's five task groups and the screen bodies vary in composition. A flat 1 would make every screen one undifferentiated strip; a 3 would cost the operator orientation. |
| MOTION | 1     | Hover, focus, and state transition only, plus the mobile drawer slide. No scroll choreography on a screen an operator opens twenty times a day.                                                                                                                |

`DESIGN.md` §2.1 is the source of these values. A change to a dial is a one-line change there, and this
table follows it.

### 9.3 Binding to the component layer

shadcn-svelte is a primitive layer, and its untouched defaults are close to the pattern antislop calls a
sterile default: near-white surfaces, thin grey borders, small radii, and a generic font. Shipping those
defaults unmodified is a fail. Therefore:

1. Core and accent colours, the typeface, the radius scale, and the density scale are authored in
   `DESIGN.md` first, then expressed as tokens. Components consume tokens, never raw hex values.
2. The palette stays at 2 to 3 core colours plus one accent (R-29). Neutral greys do not count toward the
   cap.
3. Radius is a hierarchy tool with a consistent scale; not every element becomes a pill (R-11).
4. Shadows mark elevation only, on surfaces that actually float, such as a modal or a drawer (R-12).
5. One identity motif is chosen and repeated, for example a specific table row treatment or a specific
   status chip shape, so the panel belongs to this product and not to its component library (R-20).
6. Badges exist only where a value is real: endpoint status, key health, quota source, request status.
   No capsule badge for decoration (R-09).
7. Charts get one accent series colour and neutral comparison series; no rainbow fills, no glow (R-01,
   R-13).

### 9.4 Verification duties before a screen ships

1. Contrast measured per theme for every text, chip, and border pair in use, with the measured ratios
   recorded in the pull request (R-25).
2. Both themes exercised on every screen (R-34).
3. Mobile breakpoints exercised, with no horizontal page overflow (R-03).
4. Keyboard-only pass through every interactive element, including dialog open, Escape close, and focus
   return (R-32).
5. A recorded click-through of every interactive element on the screen, with the outcome per element
   (R-35). A pass claimed without that list is not a pass.

### 9.5 Reason log (R-31)

| Decision                                                                       | One-line reason                                                                                                                                                          |
| ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| SvelteKit with CSR on authenticated routes                                     | The session lives in an HttpOnly cookie owned by the API; server-rendering panel state would duplicate that truth.                                                       |
| shadcn-svelte as the primitive layer                                           | The owner asked for a fresher component layer; it provides accessible primitives for dialogs, comboboxes, and tables that are expensive to build correctly from scratch. |
| Zod as the single boundary validator                                           | One schema per resource gives forms, URL params, and responses the same rules, so validation cannot disagree with itself.                                                |
| Same-origin serving with a dev proxy                                           | Keeps the session cookie first-party, which removes a whole class of credentialed-CORS failures.                                                                         |
| Filters kept in the URL                                                        | An operator's filtered view is shareable and survives back navigation.                                                                                                   |
| Cost kept as a string end to end                                               | The API sends a decimal string; converting to a float in the panel would introduce rounding that the operator would read as a real cost.                                 |
| Polling instead of a live stream                                               | Management API v1 exposes no stream; labelling a poll "Live" would be a false claim.                                                                                     |
| Deprecated Caveman config not exposed                                          | A deprecated switch teaches an operator to depend on something scheduled for removal.                                                                                    |
| Media nav item labelled Web Search, not Web Fetch & Search                     | There is no fetch endpoint in SPEC-API §7.10, and a nav label must match what exists (R-24).                                                                             |
| Modal for the one-time gateway key                                             | The API returns the plaintext once; a modal is the only surface that makes the one-time nature explicit.                                                                 |
| Bun as the runtime, not only the toolchain                                     | A single runtime for dev, test, build, and serving removes the Node and Bun split that causes adapter drift, and the owner asked for Bun for application speed (§10.1).  |
| The panel server forwards `/api/v1` instead of calling `app-serv` cross-origin | A first-party origin keeps the session cookie on `SameSite=Lax` and removes an entire class of credentialed-CORS failures.                                               |
| Strict test-first on schema and client code                                    | Zod schemas and the HTTP client are the panel's contract surface; a hardcoded fix there passes today's test and breaks on the next response shape.                       |

## 10. Code structure and boundaries

Target tree, replacing the current placeholder directory for `app-ui/`:

```
app-ui/
├── src/
│   ├── lib/
│   │   ├── api/            # one HTTP client, endpoint functions, error mapping
│   │   ├── schemas/        # Zod schemas per API resource (§7)
│   │   ├── components/     # panel components built on the primitive layer
│   │   ├── primitives/     # shadcn-svelte generated components, unmodified
│   │   ├── stores/         # session, theme, sidebar preference
│   │   ├── strings/        # panel prose per screen, English only
│   │   ├── icons.ts        # the one icon map, one written reason per icon (§8.11)
│   │   ├── navigation.ts   # the sidebar tree as data (§5.2)
│   │   └── utils.ts        # class-name helper for the primitive layer
│   ├── routes/             # routes from §5.1
│   ├── app.css             # the token layer, DESIGN.md §3 to §5
│   ├── app.html
├── assets/static/          # owner-supplied source assets (logo originals)
├── static/                 # served assets: logo mark, favicon, self-hosted fonts
├── tests/                  # schema, component, token, and navigation tests
├── components.json         # the primitive layer's install configuration
├── package.json
└── README.md
```

Two files carry structure that a reader would otherwise have to reconstruct from components:

- **`src/lib/navigation.ts`** holds the sidebar tree: the five groups, each with a written reason, and
  every item with its label, its icon key, and either an `href` or a `Planned` state. A screen is added in
  one place. The rule from §5.2.2 is enforced by the type rather than by review, because an item without
  `href` has nothing for the renderer to link to.
- **`src/lib/icons.ts`** holds one entry per navigation row, each with a one-line reason (R-04, R-31).

Boundaries:

1. `src/lib/api/` is the only place that calls `fetch`, and the only place that knows about HTTP status
   codes. Components receive typed data or a typed error.
2. `src/lib/schemas/` is the only place that defines a field list. Forms and response parsing import the
   same file.
3. No component imports from another route's directory. Shared UI moves to `src/lib/components/`.
4. No secrets, tokens, or connection strings in the bundle. A build-time check greps the built output for
   key-shaped strings and fails on a match.
5. Server-only concerns stay out: the panel has no database client, no Redis client, and no long-lived
   process.
6. `src/app.css` is imported once, from the root layout, because that is the one component every route
   renders through. A route cannot load without it. This is stated as a rule because the opposite
   happened: the token layer was written and tested in U0 and never imported anywhere, so the whole panel
   rendered unstyled while its contrast test passed. The test measured the file, not the page.
7. `src/lib/primitives/` is generated by the component layer's CLI and is not hand-edited, except for a
   token value that has to satisfy `DESIGN.md` (the sidebar rail width). A change there is recorded in
   `DESIGN.md` §11 with its reason, and it is re-applied after a regeneration rather than assumed to
   survive one.

File and header rules: source files stay under 250 lines (warning at 200, matching the project gates),
every file opens with a header comment stating its purpose and the API endpoints it touches, and no file
carries a `TODO` without an owner and a phase.

### 10.1 Runtime: Bun

1. **Bun everywhere.** `bun install`, `bun run dev`, `bun run build`, `bun test`, and the production start
   command all run on Bun. The panel process is a Bun process, which is the owner requirement and the
   reason the panel is fast to start on a small host.
2. **Production adapter.** SvelteKit produces a server build through an adapter. The panel uses the Bun
   adapter. If a required capability is blocked by that adapter, the fallback is the Node adapter output
   executed by the Bun runtime, and the switch is recorded as a one-line note in `app-ui/README.md` with
   its reason. The runtime stays Bun either way.
3. **The panel server owns the same origin.** Its server layer serves the built assets and forwards
   `/api/v1/*` to `app-serv` (§3.1), so the session cookie is first-party and no credentialed CORS setup
   exists to misconfigure. The forward preserves method, headers, and streaming responses.
4. **No Node-only dependency.** A dependency that requires Node-specific APIs and cannot run on Bun is
   rejected at review time, before it reaches the lockfile.
5. **Startup check.** The start command prints the Bun version, the panel version, and the resolved API
   target once at boot, so an operator can see what is running without reading a config file.

### 10.2 Strict TDD protocol (mandatory)

Every `app-ui` task follows [`docs/RULLES/TDD.md`](../RULLES/TDD.md) as written. It is not a style
preference, and this section does not paraphrase it away.

1. **Test first.** A task starts from the tests, not from the implementation. A task with no failing test
   first is not started.
2. **Mandatory output order**, per `docs/RULLES/TDD.md` §3: analysis, then implementation, then a
   compliance self-check. The analysis names three things before any code is written: the core logic the
   test demands, the identified edge cases, and the dynamic transformation strategy. Writing the analysis
   after the code is a protocol violation.
3. **No hardcoding.** No conditional branch whose only purpose is to match a test input, no literal
   return that happens to satisfy an assertion, no lookup table built from the assertions.
4. **Table-driven tests.** Every test function uses a table of at least three to five distinct input
   variations, covering a typical value, a boundary value, a zero or empty value, a negative or invalid
   value, and an extreme value, per `docs/RULLES/TDD.md` §2.5.
5. **Fuzz resilience.** The implementation must hold for inputs that never appear in the suite. A pull
   request states why the logic generalizes, in one line.
6. **Where it binds hardest in this panel.** Zod schemas (§7), the API client and its error mapping (§10),
   pagination and filter parsing (§8.4), and formatting utilities such as cost and token display (§4).
7. **Evidence.** Each pull request carries the analysis, the test file, and the compliance self-check. A
   missing analysis is the same defect class as a failing test.
8. **Repository rules.** [`AGENTS.md`](../../AGENTS.md) is present and governs the Go services
   (`app-*/**`). For the panel, this spec is the rule file: the 250-line limit, the per-file header
   comment, English copy, and the gate set in `scrypts/` (planned) apply as written in §10 and §11.
   The shared rules are kept deliberately in step rather than inherited, and §14 Q10 asks whether the
   panel should also adopt the tagged header format `AGENTS.md` §1.2 uses for Go files.

## 11. Non-functional requirements

1. **Build and run.** `bun install`, then `bun run build`, then the panel starts with the documented Bun
   start command from `app-ui/`, all without errors and without a runtime network call to a third party.
   Lockfile committed. The process runs on Bun (§10.1).
2. **Test.** Table-driven schema tests (§7.6, checklist in §7.7) and component tests for the flows in §12 run through
   `bun test`. Three specific assertions carry structural rules, and each is a test rather than a review
   habit:
   - **Navigation.** The nav tree is checked as data against the route files discovered on disk, so a row
     with an `href` that no route provides fails the build (R-24). The reverse direction is checked too: a
     route-shaped item the owner did not list fails, and so does a navigation row with no icon or no
     written reason (R-04, R-31).
   - **Tokens.** Every colour pair the panel renders is measured against WCAG AA in both themes, read from
     `src/app.css` rather than restated, so a palette regression fails the build instead of shipping
     (R-25). The accent count per theme is asserted, which is how the R-29 cap is held (one accent).
   - **Sidebar preference.** The initial open state is a function of the stored cookie and the viewport
     band, tested table-driven, because the tablet band is the one that silently regresses.
3. **Types.** Type checking passes with no implicit `any`. Types crossing the API boundary are Zod
   inferences, not hand-written duplicates.
4. **Lint.** Formatter and linter configured for the Svelte and TypeScript sources, wired to the project
   hooks. Gitleaks runs over the panel as well, since it is where a key could be accidentally pasted into
   a fixture.
5. **Drift.** A gate compares the panel's schema field lists and route constants against SPEC-API §7 and
   fails on a missing endpoint or a renamed field. This is the same class of gate the project runs on the
   Go side.
6. **Size.** The initial panel bundle is tracked per pull request; a rising trend is a review item, not an
   automatic failure.
7. **No telemetry.** The panel sends nothing to a third party. No analytics script, no error-reporting
   service, no font CDN: fonts are self-hosted, which also avoids a third-party request on an internal
   tool.
8. **Browser floor.** Current Chrome, Firefox, and Safari, plus the mobile browsers of the same vendors.
   No polyfill for a browser the owner does not use.
9. **Behaviour in every state.** Verified per §9.4, including the states in §8.3.
10. **Test-first protocol.** Every task follows [`docs/RULLES/TDD.md`](../RULLES/TDD.md) as specified in
    §10.2, with the analysis, the table-driven tests, and the compliance self-check attached to the pull
    request. This is mandatory, and a pull request missing any of the three is incomplete.
11. **Mandatory repository rules.** [`AGENTS.md`](../../AGENTS.md) governs the panel together with this
    spec. A conflict between them is resolved by editing this spec in the same pull request, never by
    silently following one and ignoring the other.

## 12. Delivery phases

Phases mirror SPEC-API §10, so the panel never ships against an endpoint that does not exist.

| Phase  | Panel scope                                                                                                                                                                                                                                                   | Exit criteria                                                                                                                                                                                                                                                                                                       |
| ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **U0** | SvelteKit shell on the Bun runtime (§10.1), token layer, theme toggle, `/login`, session handling, `/endpoint-keys` gateway keys tab, Settings Security tab, Zod foundation (§7.1 to §7.3), strict TDD protocol active (§10.2), `AGENTS.md` in place (§14 Q8) | Login, logout, and gateway key create, rename, disable, revoke each exercised against `app-serv`, with the one-time key modal verified. Schema tests for the U0 resources are table-driven and green, and each U0 pull request carries the analysis plus compliance self-check required by `docs/RULLES/TDD.md` §3. |
| **U1** | Endpoint & Key upstream tab with multi-key CRUD and test, Providers list and detail, model catalog views, Combos (fallback, round_robin), Vision Adapter, Usage, Quota read views, Logs (requests and console), API Docs, Settings Routing, Network, Logging  | An operator connects a provider endpoint with two keys, builds a `fallback` combo, routes one request through `app-serv`, and reads the request in the panel: usage row, quota window, and log detail all match. Click-through recorded per §9.4.5.                                                                 |
| **U2** | OAuth start, callback return, and status, fusion combos and combo test, media provider screens, proxy pools including batch add and tests, Token Saver (RTK, Headroom, Ponytail), quota budget caps, custom models, aliases, disabled models                  | OAuth round trip from the panel; an image generation request routed through a media provider; a proxy tested from the panel; token saver config saved and reflected by the API.                                                                                                                                     |
| **U3** | Skills tab over the served catalog (SPEC-API §7.16), Playground Chat over the data plane (§6.15; SPEC-API §10 P4 assigns the page, no new gateway route), native token saver surface when `002-TOKEN-SAVER` exists                                            | Each catalog row either resolves to a real source with a working copy control, or states the cause from §6.10. No row ships with a broken link. The playground holds §6.15's four rules: no credential in the browser, the panel server as the only holder, no new auth path, and the cost stated before the send.  |

**U0 status, 2026-09-18: implementation complete, verification run outstanding.** Every U0 obligation
above exists in `app-ui/`: the SvelteKit shell on the Bun runtime, the token layer, the theme toggle,
`/login`, session handling, the `/endpoint-keys` gateway keys tab with create, rename, disable, and
revoke, the Settings Security tab, the Zod foundation, and the not-found view §5.1 assigns to the phase.
The U0 schema and route tests are table-driven and green, and `bun run test`, `bun run check`,
`bun run lint`, and `bun run build` all pass. One exit step is a verification run rather than unbuilt
work: the recorded click-through against a running `app-serv` (§9.4.5), which needs the service,
PostgreSQL, and Redis up. `app-serv` now exposes the auth and gateway-key endpoints the panel calls, so
that run is unblocked on the panel side and U0 closes when its result is recorded.

One owner addition assigned to this phase is also complete. The changelog screen (§6.16) now reads the
release notes `GET /api/v1/changelog` serves (SPEC-API §7.18) and marks each release against the running
build, so §14 Q11 is closed and the phase carries no open source question.

**U1 status, 2026-09-21: every screen the row lists is built, and the exit criterion is a verification run
that has not been made.** Endpoint & Key's upstream tab with multi-key CRUD and test, the Providers list and
detail, the model catalog views, Combos with their two strategies and the Vision Adapter, Usage, the Quota
read views, Logs (requests and console), API Docs, and the Settings Routing and Logging tabs all exist in
`app-ui/` and are reachable from the sidebar, which carries no `Planned` row any more (§15). The Network tab
is the pointer to Proxy Pools that §6.9 assigns, not a second editor. The exit criterion is the recorded
click-through: an operator connects a provider endpoint with two keys, builds a `fallback` combo, routes one
request through `app-serv`, and reads the usage row, quota window, and log detail that all match. That run
needs the service, PostgreSQL, and Redis up, and it is outstanding; draft 007's F12 is the finding that
closes it. What the panel side has proved without a browser is the wire half: the live passes recorded in
`app-ui/README.md` drive the panel's own `/api/v1` routes and parse every answer through the schema the
screen uses.

**U2 status, 2026-09-20: all nine capabilities the U2 row lists are landed.** Token Saver (§6.7)
shipped first: RTK with its twelve-filter allowlist, Headroom with the external URL and the fails-open
copy, and Ponytail with its level, each saved as a whole-document `PUT` because that is the only write
route the API has, with the draft helpers merging the edited group over the last document read so a save
of one group cannot rewrite another. The deprecated `caveman` key is parsed and never rendered, per §13
item 6. Proxy Pools (§6.9) shipped second: the pool table, the add and edit dialog with a candidate test,
the batch paste add, the per-row test, the delete dialog, and the outbound settings card. The Settings
Network tab folded into that screen, so `SettingsNetworkTab.svelte` is deleted and `/settings` links to
`/proxy-pools` and `/token-saver` rather than embedding either. Media Provider (§6.8) shipped third: one
screen at six addresses, a card per registry provider with its configured-endpoint count, a `base_url`
override, and a default model selector over the declared set. The disabled and custom model writes (§6.3)
shipped fourth, both on the provider detail screen: a Disable action per catalog row, a list of the models
this provider cannot route with an Enable action per row, and a custom-model table with an add form and a
removal dialog. The alias set (§6.3) shipped fifth, as the last model write on that screen: the alias to
target table with a form that adds a row or changes what an existing one targets, each change a whole-set
`PUT`, with the target field suggesting the catalog ids and the combo names. The OAuth section (§6.3) shipped
sixth, as the last section on that screen: it renders only for a provider the registry marks `has_oauth`,
reads the status once per visit, and lets the reported flow decide what it offers, so only `code` gets a start
button and the other three flows get their reason instead. The authorize URL is a link the panel does not
follow, the callback's outcome is read from this page's query and its three keys are dropped from the address
after they are read, and a manual refresh is per account and re-reads the state it moved. The combo test
(§6.4) shipped seventh: a confirmed one-token probe per stored reference, sequentially, with a modal result
row for every model and fusion judge, including failed members and the gateway's own reason. The quota
budget caps (§6.6) shipped eighth and last: a per-endpoint picker, a form whose two fields are seeded from
the stored cap, a whole-set write followed by the read that proves it, and the routing warning §6.6 asks
for stated where the cap is saved.

That fourth area is where the spec and the implementation disagree, and the disagreement is recorded
rather than papered over. §6.3 asks for "per-model enable or disable state" on the catalog, but the merged
catalog _excludes_ disabled models (`internal/service/model_catalog.go`), so a disabled model has no row
there and a state control on that list could only ever read "enabled". The panel renders the two lists the
API can serve: the catalog, where a row can be disabled, and a disabled list, which is the only place a
disabled model is visible and the only place it can be turned back on (Q19).

The alias set is the one resource on that screen that is not the provider's. Neither its read nor its write
route takes a provider, and `alias` is the table's primary key, so the section renders the whole set on
every provider's detail screen and says so in as many words. Two consequences are recorded rather than
hidden. §6.4 asks the combo delete refusal to link to the fixed set, and no link can be built, because the
only screen that offers the fix takes a provider id and the refusal names none, so the dialog names the
place instead (Q20). And an alias whose target has since been disabled or whose target combo has since been
deleted cannot be written back, while every write replaces the whole set, so one such row refuses every
alias edit until it is removed (Q21).

Five facts were settled by reading the implementation rather than the spec, and all five are recorded in
§14 instead of worked around. The pool routes nothing: the egress path reads
`settings.network.outbound_proxy_url` and nothing else, so §6.9's empty-state sentence promised
behaviour the API does not have, and the screen states the truth (Q15). An enabled proxy with an empty
URL dials direct, which is the bypass §7.11 says the setting exists to prevent, so the panel refuses to
write that combination and names it when a stored document already holds it (Q16). A kind that declares
no models cannot have a default model set from the panel, because a selector over an empty set is a dead
control and §6.8 asks for no other shape (Q17). Clearing a `base_url` override is a save the panel cannot
check, because the registry's own value is not on the wire while an override exists (Q18).

The media kinds are the first sidebar rows to link to a parameterised route. A row may do that when the
row itself fixes the parameter, which is the case here: the kind is part of the row's identity rather
than something an operator picks after opening the screen. `navigation.ts` gained a `link` shape for it,
`/providers/[provider_id]` stays excluded for the opposite reason, and a test asserts that every
placeholder a row's route declares is filled and no parameter is passed that the route does not declare.

Nothing in U2 is still open on the panel side. §6.4's combo delete refusal names where the fix lives instead
of linking to it, because the alias set is global and the only screen it has takes a provider id (Q20).

**U2 exit criteria, 2026-09-20: two of the four are met against a running `app-serv`, and the other two
are not.** A live pass booted the service and the built panel and drove the panel's own `/api/v1` routes,
parsing every response through the schema the screen uses: 25 checks, 0 failures. "A proxy tested from
the panel" is met, with the probe confirmed in a loopback proxy's own log rather than inferred from the
answer, and so is "token saver config saved and reflected by the API", with the saved document returned
verbatim and agreed by a fresh read. The pass also covered the pool read, create, patch, and delete, the
candidate test's live, refused, and protocol-mismatch answers, and the outbound settings write and
re-read. "OAuth round trip from the panel" and "an image generation request routed through a media
provider" stay open, and now for a different reason than when the pass ran: both screens exist, so what is
missing is the evidence rather than the surface. The OAuth round trip additionally needs a provider account
the pass does not have, since the registry marks one provider (`xai`) as `has_oauth`, and that provider
reports the `device` flow, so no provider currently offers the start path at all (Q23). The quota budget
caps landed after that pass, so they were checked in a separate one: a driver logged in, created an
endpoint, and drove the cap routes while parsing every response through the panel's own schemas, 23 checks
with 0 failures, covering the null cap, both amounts written and read back, the form's round trip, an
omitted field clearing one cap while the other survived, an empty body answering a cap object with no
amounts, and four refusals including `404 upstream endpoint not found`. The database was returned to its
baseline, counted before and after. One limit is recorded rather than glossed: the panel is client-rendered
(`ssr = false`), so the pass proves the wire contract and not the rendered output, and a browser
click-through of both screens is still outstanding.

**U3 status, 2026-09-21: the Skills half is built, and its exit criteria is met on the panel side; the
native token saver half still waits on `002-TOKEN-SAVER`.** The catalog of record is the route, so the
screen renders the seven rows `GET /api/v1/skills` serves and nothing else: an entry skill that indexes the
rest, and one row per capability endpoint (§7.15). The two addresses a row carries are rendered as what
they are, `raw_url` as the address an agent fetches and `blob_url` as the link a person reads. The seven
documents live at `skills/<id>/SKILL.md` in this repository, which is where the catalog's own constants
point; the route serves metadata and never a body, so no document can drift from what the gateway serves.

The exit criterion is "resolves, or states the cause", and the panel can only answer it by asking: the
catalog carries a path rather than a file, so each row's address is requested and the answer decides what
renders. A row that answered gets its copy control and its link; a row that did not states the cause (a 404
as "not published at the ref the catalog names", any other status by number, a timeout, a network failure)
and keeps the address on screen, because that address names what has to be published. The check runs on
load and again on request, and the count it reports is measured rather than asserted. The source request is
a `HEAD`, so the panel asks whether the file is there without downloading it. The live pass on 2026-09-21
ran that whole path against a booted `app-serv`: the catalog read through the panel's own schema, every
derivation driven over the result, and all seven addresses asked with real requests. 26 checks, 0 failures,
including two controls that keep the answer from being vacuous, a file that is on the ref reading available
and a path that is not reading missing.

Two limits are recorded rather than glossed. The panel is client-rendered, so the render evidence is the
jsdom tests and the wire evidence is the live pass; a browser click-through is outstanding, as it is for
U0 and U1. And the seven documents are in the working tree but not yet on `refs/heads/main` on the origin,
which is the ref the catalog names, so every row reports the not-published cause until the owner pushes
that commit. That is the honest state of the criterion rather than a pass claimed early: the panel's half
is done, and the source half is one push away.

**U3 status, 2026-09-21: the Playground Chat half is built, and its credential path is held by tests rather
than by review.** The screen sends one message through the data plane and shows what the gateway answered:
a model picker fed by `GET /api/v1/models`, a message box, a send control with the cost sentence beside it,
a streamed answer with a Stop control, and the facts rule 4 asks for (the resolved model, the finish
reason, the token counts, and the HTTP status) beside a disclosure of the frames exactly as they arrived.
The credential never reaches the browser: the panel's own routes read it from the server environment and
attach it to the outbound request, and three tests hold that shape, a source-location guard on the
variable's name, a rule that the credential is written in exactly one function, and a rule that the
browser's own files name neither a store nor a credential header. The built client bundle was grepped for
the variable's name and does not contain it. Two facts the wire did not state read as `not stated` rather
than as zeroes, and the three stream endings get three sentences. What remains is the click-through against
a running gateway with a configured key, which is the same outstanding verification U0, U1, and U3's Skills
half carry.

## 13. Locked decisions

1. The panel consumes `/api/v1` with a session cookie and never touches PostgreSQL or Redis (SPEC-API §11.5).
2. Multi-key per upstream endpoint is a first-class panel concept, not a single hidden field (owner
   requirement; SPEC-API §11.2).
3. The nav list equals the owner KEEP list plus Skills. No screen beyond §2.1 is built.
4. Component layer is shadcn-svelte and icons are `@lucide/svelte` with no emoji in UI copy (owner,
   2026-09-16). Legacy panel parity is the structural reference.
5. **Zod is the single strict validation and sanitization layer in `app-ui`**, taking the latest published
   version at install time (the v4 line), applied to form input, route and search params, API responses,
   and panel environment configuration (owner, 2026-09-16; §7). The panel validates for correctness, and
   `app-serv` remains the security boundary. No second validator and no ad-hoc checks in components.
6. Token Saver keeps RTK, Headroom, and Ponytail as live sections. **`caveman` is DEPRECATED** (owner,
   2026-09-16): the API keeps the key frozen for round-trip compatibility, the panel parses it and never
   renders it, and it is removed with `/api/v2` (SPEC-API §7.9, and §6.7 here). Drop was considered and
   rejected for one reason: the key still carries migration information for anyone importing a reference
   configuration, which a silent deletion would lose.
7. The Skills tab carries the served catalog and nothing else, and the Token Saver screen stays limited to
   the legacy savers (owner, 2026-09-16; amended 2026-09-21). The two skill names this item originally
   listed, `/antislop` AI and SuperPowers, were never served by any route; SPEC-API §7.16 defines the
   catalog of record, and the owner chose it over inventing a second list (§6.10, §14 Q2). The half that
   did not change is the split: skills are not mixed into the Token Saver screen.
8. One HTTP module, one schema module, one strings module per screen (§10).
9. Light and dark are both shipped and both verified (R-34; legacy parity, since the reference panel ships
   a theme provider and a toggle).
10. Every control is backed by an endpoint in SPEC-API §7. A control without one is removed (R-26).
11. Cost values stay decimal strings end to end; token counts stay integers.
12. No third-party runtime service: no analytics, no error-reporting SaaS, no font CDN, no CAPTCHA.
13. **Bun is the runtime.** The panel is built, tested, and served with Bun, and the application process
    is a Bun process, not only a Bun package manager invocation (owner requirement, §10.1).
14. **Governance is mandatory and explicitly linked.** [`docs/RULLES/TDD.md`](../RULLES/TDD.md) is
    binding on every panel task through §10.2, and [`AGENTS.md`](../../AGENTS.md) is present for the Go
    services. The panel keeps this spec as its own rule file, so a Go rule cannot be applied or skipped
    here without an edit that says so.

## 14. Open questions

1. **Closed 2026-09-17: `DESIGN.md` is present.** The owner supplied the direction the same day: legacy
   structural parity, delivered fresh, with mobile, tablet, and desktop behaviour as an explicit
   requirement, and the logo at `app-ui/assets/static/logo.png`. `DESIGN.md` at the repository root holds
   the identity, palette, typeface, radius and elevation scale, identity motif, and reason log, so the
   "draft without direction" state in §9.1 is closed. One follow-up stays open: the typeface is Inter,
   chosen for legibility at 12px in dense tables and for parity with the legacy panel, and it is a value a
   later `DESIGN.md` revision can change in one line.
2. **Closed 2026-09-21: the source is the served catalog, and the documents are in this repository.** The
   question was where `/antislop` AI and SuperPowers live for `pannelAI`. Neither name survives: no route
   ever served them, and SPEC-API §7.16 defines the catalog of record instead, one entry per capability
   endpoint the gateway serves, the entry skill first. The gateway derives both addresses from constants
   beside that catalog (`skillsRepo`, `skillsRef = main`, path `skills/<id>/SKILL.md`), so the source is
   decided in code rather than left open, and the seven documents now exist at that path: `pannelAI` as the
   index plus `pannelai-chat`, `-image`, `-tts`, `-stt`, `-embeddings`, and `-web-search`. Each is written
   from the contract the gateway serves (SPEC-API §7.3, §7.10, §7.15), so none advertises a capability §7
   does not define. §6.10 is amended to that catalog, and the panel renders it: `/skills` shows one row per
   capability, hands out the install line, and asks each address before it offers a copy control, so no row
   ships a broken link. The earlier text's "two owner-selected skills" is superseded; the panel's own rule
   for what it may show is unchanged, which is that a control copies only what it has confirmed.
3. **Closed 2026-09-21: the machine-readable contract exists, and it is served rather than filed.** The
   artifact SPEC-API described as `docs/SPEC-API/002-SPEC-API-openapi.md` never landed under that name.
   What landed is an OpenAPI 3.1 document at `GET /api/v1/openapi.json` (SPEC-API §7.17), generated from
   `docs/CONTRACT/001-CONTRACT-API-V1.yaml` by `app-serv/tools/openapi-gen` and embedded in the binary. It
   carries 67 paths, 23 tags, 161 schemas, the two security schemes, the per-plane error code map, and an
   `x-contract` block that names its own source and generator. The panel consumes it: `/api-docs` renders
   it (§6.12). The panel's own response schemas stay hand-written from SPEC-API §7, because the document
   describes the gateway's shapes rather than the subset the panel parses. Drift is gated on the generator
   side (`scrypts/gates/contract-openapi.sh` compares the artifact against its YAML), and
   `TestOpenAPICoversEveryRegisteredRoute` compares it against the router, so a route added without a
   document entry fails a build instead of shipping a lagging page.
4. **Cost granularity.** Should `/usage` show cost with 4 or 6 decimals, and should the panel display a
   currency code? The API returns a decimal string with no currency field.
5. **Provider registry scope.** SPEC-API §12 asks whether all registry providers land in P1 or the API-key
   category first. The answer changes the Providers screen's default filter and paging behaviour.
6. **Purge guard threshold.** Is 1000 rows the right typed-confirmation threshold for log purge, or should
   the confirmation be based on the retention window instead?
7. **Closed 2026-09-21: the screen is built, the credential stays on the server, and the phase is U3.** The
   owner added Playground Chat to the sidebar on 2026-09-17, so the earlier recommendation to leave it out
   permanently is withdrawn, and settled the auth question in the same decision:
   - **Auth: decided, server-side injection.** The panel's own server performs the data-plane call and
     injects a gateway key read from its environment. The browser is never given a credential, and none is
     stored client-side, because a credential in a page is readable by any script on the origin, visible in
     devtools, and persistent if saved. That exposure is obvious and the owner rejected it outright.
   - **Key scope: a gateway key, not a provider key.** The gateway still does its own auth, routing, quota,
     and usage accounting, so the panel does not become a second path to a provider.
   - **Phase: U3.** SPEC-API §10 P4 assigns the page over §7.15 with no new gateway route, and the panel
     side is built: `GET /playground/models` and `POST /playground/chat` read the key from
     `PANEL_PLAYGROUND_KEY`, check the caller's session against `GET /api/v1/auth/status`, and forward the
     call. The sidebar row carries an `href` instead of a Planned chip.
   - **Cost disclosure.** A sent request routes through a real provider, consumes quota, and writes a usage
     record, so the composer states it before the operator sends one.
   - **What remains** is the click-through against a running gateway with a configured key, which the owner
     provides (draft 007, decision D6), and it is the same outstanding verification U0, U1, and the Skills
     half of U3 carry. §6.15 records the model, the failure vocabulary, and the rules the screen holds
     itself to.
8. **Closed 2026-09-18: the governance follow-ups are resolved.** [`AGENTS.md`](../../AGENTS.md) landed
   at the repository root and scopes itself to Go services under `app-*/**`, so the earlier blocking note
   in this section is withdrawn: it does not gate the panel. Both follow-ups it raised are now closed.
   `SYSTEM_MAP.md` exists at the repository root, and the panel keeps this spec as its own rule file
   rather than inheriting the Go rules by accident. `scrypts/` exists and carries the shared gates and git
   hooks, so the panel's gate set is wired rather than assumed. The earlier sentence that made the
   absence of `scrypts/` the reason U0 could not be called complete no longer applies.
9. **Enumerate gateway key status values.** SPEC-API §7.3 accepts `status` on PATCH but never lists the
   allowed values, and §6 does not either. The panel therefore writes `active` and `disabled` from two
   constants in `src/lib/schemas/gateway-key.ts` and renders any other value verbatim, so it cannot
   reject a value the server accepts. The API spec needs the enum before U1 adds endpoint key health,
   where the same question returns with circuit-breaker states.
10. **Panel header format.** `AGENTS.md` §1.2 defines a tagged header for Go files (`@file`, `@for`,
    `@uses`, `@reason`, `@author`, `@layer`, `@stability`, `@since`), while the panel files carry a
    free-form rationale comment plus a purpose line. Should the panel adopt the same tags so a reviewer
    greps one format across both apps, or keep free form because tags such as `@layer` and `@stability`
    describe Go package concepts that the panel does not have?
11. **Closed 2026-09-21: the source is the route, and the screen renders it.** The owner added Changelog to
    the sidebar on 2026-09-17 and asked for the panel side to be prepared while the `app-serv` side was
    built separately. That side landed at P4 as `GET /api/v1/changelog` (SPEC-API §7.18), which serves the
    five releases the binary carries, newest first, as `{data: [{version, date, title, notes}]}`. The
    question's first option, an endpoint, is what shipped, so the panel reads it: `/changelog` renders the
    served releases and marks each against the running version from `GET /api/v1/version`, and its empty
    state names the route rather than a source the panel is missing. §6.16 is amended from the shape this
    question left open (`category` plus `items[]`) to the shape the route serves, because the panel-side
    mapping would have had to invent a category and split a paragraph into bullets nobody wrote. The other
    two options stay rejected: a build-time file would drift from the running gateway, and removing the
    screen would leave the owner's row without its page. The live pass on 2026-09-21 read the served notes
    through the panel's own schema and derivations, 18 checks with 0 failures, and the panel bundles no
    release text of its own.

12. **The model catalog has no "suggested" flag to filter on.** §6.3 asks for a "suggested" toggle on the
    provider detail catalog, and SPEC-API §7.4 offers `?suggested=true` on
    `GET /providers/{provider_id}/models`. The parameter is a documented no-op: `ProviderModelsFrom` sets
    `Suggested: true` on every row, because the embedded registry carries no suggestion flag and a
    hardcoded subset would be a guess the panel then renders as advice. A toggle over a constant would
    either filter nothing or empty the table, which is a control that lies (R-26), so the panel ships no
    toggle and the catalog reads `GET /models/catalog` instead, which is the route that carries the search
    and capability filters §6.3 also asks for. Decide whether the registry gains a real suggestion flag or
    §6.3 drops the toggle.
13. **The provider list has no search, and the API is why.** §6.3 asks for a search over name and ID on
    `/providers`. SPEC-API §7.4 declares `?category` and `?routability` only, and the handler reads nothing
    else, so the panel cannot send a search term and §6.3's own pagination discipline forbids filtering the
    registry in the browser. The list therefore ships with the category filter alone. Decide whether
    SPEC-API adds `?q` (the shape `/models/catalog` already uses) or §6.3 drops the requirement.
14. **The RFC3339 check is looser than §7.2 says, and it is one primitive away from every screen.** §7.2
    lists timestamps as "RFC3339 only", and `src/lib/schemas/primitives.ts` implements `rfc3339Timestamp`
    as `!Number.isNaN(Date.parse(value))`. `Date.parse` accepts strings RFC3339 does not allow, for
    instance `19 September 2026`, so a timestamp the API should never send still parses. The consequence is
    bounded: the panel renders a correct time or fails on a value no parser can read, and the drift is
    caught later than §7.4.3 intends rather than shown wrongly. Found while writing the proxy pool
    contract, and deliberately not fixed there: `rfc3339Timestamp` is composed into every response schema
    in the panel, so tightening it is a change across unrelated screens rather than a fix in one. Decide
    whether the primitive gains a real RFC3339 pattern, with every schema re-checked in one pass, or §7.2
    accepts `Date.parse` as the panel's reading of "RFC3339 only".
15. **The pool's empty state promises routing the API does not do.** §6.9 gives the empty state as "No
    proxies yet. Add one to route upstream calls through it.", while the same section's outbound bullet
    says the global `network.outbound_proxy_*` settings are what this screen edits. SPEC-API §7.11
    settles which of the two carries traffic: the outbound URL does, per-endpoint binding is deferred, and
    a pool row is a stored candidate. Checked against the implementation rather than inferred: the egress
    path reads `settings.network.outbound_proxy_url` and nothing else
    (`app-serv/cmd/app-serv/egress_wiring.go`), and the pool repository is read by the proxy CRUD service
    alone. The screen therefore ships "No proxies yet" with "Add one to keep and test an address before
    you point the outbound setting at it.", states the same fact on the outbound card, and says in the
    page header that the pool routes nothing. Decide whether §6.9's sentence is corrected, or whether a
    later phase makes a pool row routable and the original sentence becomes true.
16. **An enabled proxy with no URL dials direct, which is the bypass §7.11 says the setting exists to
    prevent.** SPEC-API §7.11 states the intent plainly: "a settings read or a proxy URL that fails
    refuses the call rather than dialing direct, since quietly bypassing a proxy the operator enabled is
    the failure this setting exists to prevent." The implementation covers the failing read and the
    unparseable URL, and not the empty one: `proxyRoute` returns `nil, nil` when
    `outbound_proxy_enabled` is true and `outbound_proxy_url` is empty
    (`app-serv/cmd/app-serv/egress_wiring.go`), so the call goes direct while the setting reads as
    proxied. Nothing in `domain.Settings.Validate()` or `NetworkSettingsPatch` rejects the combination
    either, so the API stores it. The panel refuses to write it (a cross-field rule in
    `src/lib/schemas/settings.ts`, with a message naming both ways out) and states it when a document
    already holds it, because the panel cannot prevent a state it did not write. Decide whether
    `proxyRoute` refuses an empty URL when proxying is enabled, or §7.11 records the empty value as
    "off" and the panel's rule is dropped.
17. **A kind that declares no models cannot have its default model set from the panel.** §6.8 asks for a
    default model selector. `validateMediaModel` returns early when the kind declares no models
    (`app-serv/internal/service/media_provider_resolve.go`), so the API accepts any string for such a
    kind, but a selector over an empty set has no options and is therefore a dead control (R-26). The
    panel renders the fact instead: "This service declares no models, so there is nothing to choose."
    The result is a capability the API has and the panel does not expose, and the alternative the panel
    rejected was a free-text field §6.8 does not ask for, which would invite a model name nothing in the
    registry can route. Decide whether §6.8 gains a second control shape for that case, or whether the
    API refuses a `default_model` for a kind that declares none, which would make the panel's silence
    the correct behaviour rather than a gap.
18. **Clearing an override is a save the panel cannot check.** The server refuses a save that would
    leave a provider with no base URL from either source, and the panel blocks the one case it can prove:
    `base_url_source` is `registry` and the value is empty, so the registry declares none and an empty
    field means no base URL at all. When the source is `override` the registry's own value is not on the
    wire, because `viewFor` reads it only after the override resolves empty
    (`app-serv/internal/service/media_provider_resolve.go`), so the panel cannot tell whether clearing
    the field is safe. It sends the clear and shows the server's answer, which is honest but means an
    operator can be refused for a state the panel could have predicted. Decide whether §7.10 adds the
    registry's own value beside the resolved one, which would make every case provable, or whether the
    current split is accepted as the cost of a two-field block.
19. **§6.3's per-model enable or disable state cannot live on the catalog, because the catalog hides a
    disabled model.** `ModelCatalogService.lookups` skips every pair in the disabled set
    (`app-serv/internal/service/model_catalog.go`), so `GET /models/catalog` never returns a disabled row
    and a state control on that list could only ever read "enabled". The panel renders two lists instead:
    the catalog, where each row offers Disable, and "Models this provider cannot route", which is the only
    place a disabled model is visible and the only place it can be turned back on. `GET /models/disabled`
    reads every provider's pairs, so the disabled list is narrowed to the screen's provider for display
    while the whole set is what a write carries, because `PUT /models/disabled` replaces the set for every
    provider at once. Decide whether §6.3's catalog bullet is amended to describe the two lists the API
    can serve, or whether the catalog gains the state some other way, which would need a route the API
    does not have.
20. **§6.4 asks for a link to the alias set, and the only screen that holds it is provider-scoped.** The
    set is global: `GET /models/aliases` and `PUT /models/aliases` take no provider
    (`app-serv/internal/router/router.go`), and `model_aliases.alias` is the primary key
    (`app-serv/migrations/000006_model_catalog.up.sql`), so an alias belongs to no provider and to all of
    them at once. §6.3 places the table on the provider detail screen, and §6.4 asks the combo delete
    refusal to link to it. The refusal names an alias and no provider, so there is no provider id to build
    the route from and no honest link to render: R-24 forbids a link that does not offer the fix. The panel
    renders the table on every provider's detail screen with a sentence saying the set is global, and the
    dialog says where the fix lives in words instead of linking. Decide whether §6.4's sentence is amended
    to match, or whether the alias set gets a provider-independent home, which would mean a screen or a
    route the nav list does not currently carry (§2.1).
21. **One alias whose target stopped resolving refuses every alias write, because the write replaces the
    whole set.** `ReplaceAliases` validates each entry against the merged catalog and the combo names before
    it writes anything (`app-serv/internal/service/model_catalog_writes.go`), and the merged catalog skips
    the disabled set, so an alias written before its target was disabled no longer passes validation. The
    read path does not re-check, so the row still renders (`RehydrateModelAlias`), but every write is
    refused until that row is removed, and the refusal names the alias and its target, which is what tells
    the operator which row to remove. Removing it is a write that succeeds, because the rest of the set
    validates. Decide whether §7.6 documents that a stored alias can become unwritable, or whether the
    validation should pass an entry whose target is unchanged from what is already stored.
22. **§6.3 says the panel "polls" the OAuth status, and the panel deliberately runs no timer.** The section
    reads `GET /providers/{id}/oauth/status` once per visit and re-reads it after a refresh or a callback
    return, which are the two events that move what it renders. A timer would be a request the operator did
    not ask for, repeating on its own, and the panel's rule for a screen is that a read follows a visit or an
    action. The consequence is narrow but real: `refresh_state` is derived from the clock
    (`app-serv/internal/domain/oauth_refresh.go`), so a token can cross into its refresh window while the
    section sits open and its row keeps reading "outside its refresh window" until something re-reads. The
    only control that re-reads is a row's Refresh, which also refreshes the token, plus a page reload. Decide
    whether §6.3's "polls" means a timer the panel should run, or whether the read-on-visit and
    re-read-on-action is the intent and the sentence is amended to say so.
23. **The one provider with `has_oauth` reports the `device` flow, so the panel's start action has no provider
    to act on.** `xai` is the only registry entry with `has_oauth: true`, and its `oauth` block declares
    `client_id`, `token_url`, and `refresh_url` and no `authorize_url`
    (`app-serv/internal/registry/registry.yaml`). `flowKind` classifies a provider by its exchange, its
    authorize URL, and nothing else: `RequiresCustomExchange` is false for xai (no `state_url`,
    `initiate_url`, or `poll_url_base`), so the empty authorize URL falls through to `device`
    (`app-serv/internal/service/oauth_flow_refresh.go`). The panel follows the API's classification, which
    means the section renders for xai, states the device reason, and offers no start button, and the start
    path is dormant until a provider declares an authorize URL. The same classification is imprecise for this
    entry: xai declares no device endpoint either, so the copy's "connect it through its device endpoint"
    points at something the registry does not carry. Decide whether the registry gains an `authorize_url` for
    xai (or another provider gains `has_oauth`), whether `flowKind` distinguishes a refresh-only block from a
    device flow, or whether the start path stays dormant by design and §6.3 records it as such.
24. **The cap form accepts a narrower spelling than the API's parser does, and the two are not the same
    rule.** `PUT /quotas/{endpoint_id}` takes the cost as a string and parses it with
    `big.Rat.SetString` (`app-serv/internal/domain/decimal.go`), which accepts forms a budget field should
    not: `1e9` parses as a billion, and `1/2` as half a dollar. The panel's form accepts a plain decimal
    only (`/^\d+(\.\d+)?$/`), so a value the API would take is refused in the panel, and the two disagree
    about what a legal amount looks like. The read is unaffected in practice: the API prints a stored amount
    through `Decimal.String()` with eight places, so whatever spelling was written comes back as a plain
    decimal the panel parses. Decide whether SPEC-API narrows the wire to a decimal grammar (which would
    make the panel's rule a copy rather than a stricter one), or whether the panel should accept the
    parser's full grammar and display whatever it is given.

## 15. Evidence for numbers and paths used here

Every count in this document is measured, and every referenced path is marked present or planned. This
table exists so a reader can re-run the measurement instead of trusting the sentence.

| Claim                                          | How it was measured                                                                    | Result                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| ---------------------------------------------- | -------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Reference repository identity                  | `git log -1` and `node -e` on `package.json` in `/home/rusmanadodi/ai-gateway`         | HEAD `9766494`, dated 2026-09-09; package `9router-app` version `0.5.55`                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| Provider registry size                         | `ls open-sse/providers/registry/ \| wc -l` in the reference                            | 121 files                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| Reference management API breadth               | `find src/app/api -name route.js` excluding `api/v1` and `api/v1beta`                  | 129 route files                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| Reference data-plane API breadth               | `find src/app/api/v1 src/app/api/v1beta -name route.js`                                | 23 route files                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| Reference dashboard breadth                    | `find 'src/app/(dashboard)/dashboard' -maxdepth 1 -type d`                             | 16 feature directories under the dashboard directory itself                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| Reference shared component count               | `ls src/shared/components/*.js \| wc -l`                                               | 46 files                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| Reference token saver flag set                 | `grep -n "DEFAULT_SETTINGS" -A 60 src/lib/db/repos/settingsRepo.js`                    | Flags include `rtkEnabled`, `headroomEnabled`, `cavemanEnabled`, `ponytailEnabled`                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| Component layer facts                          | shadcn-svelte documentation and migration notes                                        | Components are built with Bits UI and Tailwind CSS, with Svelte 5 and Tailwind v4 support; the chart component is built on LayerChart                                                                                                                                                                                                                                                                                                                                                                                      |
| Validation library line                        | The `zod` page on npm, and the Zod 4 release notes on zod.dev                          | Zod 4 is the stable line, with 4.x published at the time of writing. The panel installs the latest at pin time and records the exact version in the lockfile.                                                                                                                                                                                                                                                                                                                                                              |
| `/logs` has no stream endpoint in v1           | SPEC-API §7.13 lists list, detail, purge, and console routes only                      | No SSE route for management logs, so the panel polls                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| `DESIGN.md` present                            | `ls DESIGN.md` at the `pannelAI` root                                                  | Present, added 2026-09-17. Holds identity, palette, typeface, radius and elevation, identity motif, and the reason log.                                                                                                                                                                                                                                                                                                                                                                                                    |
| `AGENTS.md` present                            | `ls AGENTS.md` at the `pannelAI` root and `grep -c` on it                              | Present. Its scope line names Go services under `app-*/**`, so it does not govern the panel.                                                                                                                                                                                                                                                                                                                                                                                                                               |
| `pannelAI` root contents                       | `ls -la` at the project root                                                           | `.gitignore`, `README.md`, `AGENTS.md`, `DESIGN.md`, `SYSTEM_MAP.md`, `docs/`, `deployment/`, `scrypts/`, `app-ui/`, `app-serv/`, `backups/`                                                                                                                                                                                                                                                                                                                                                                               |
| Panel test count                               | `bun run test` in `app-ui/`                                                            | 2185 passing across 111 files after the F10 and F11 batch (was 2168 across 108 after the F5 to F9 batch, 1053 across 39 after the Logs work, 769 across 27 after the Combos work, 634 across 24 after the Providers work, 499 across 19 after the U0 closure work, and 396 across 13 before it)                                                                                                                                                                                                                                                                                                                   |
| Panel type check                               | `bun run check` in `app-ui/`                                                           | 0 errors, 0 warnings                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| Sidebar row count                              | `bun -e` over `NAV_GROUPS` and `allNodes` in `src/lib/navigation.ts`, counting a leaf as routable when it carries `href` or `link`, cross-checked by `bun run test` | 20 nodes in 5 groups: 19 routable leaves (13 static routes plus the 6 media kinds), 0 rows labelled `Planned`, 1 container. Measured 2026-09-21 after `/playground` (was 18 routable and 1 planned after `/skills`, 17 routable and 2 planned after `/api-docs`, and 16 routable and 3 planned before it). One flag remains, on the Media Provider container, which keeps it for shape uniformity and renders as a disclosure rather than a chip, so `grep -c 'planned: true'` finds 1 while the rendered label count is 0 |
| Accent usage in the shell                      | `grep -rn "color-accent"` across `src/lib/components`                                  | Active row marker, primary action, focus ring, link, and active tab only (DESIGN.md §3.4)                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| Logo source                                    | `file app-ui/assets/static/logo.png`                                                   | JPEG data, 1254x1254, despite the `.png` extension. Cropped to `static/logo-mark.png` (512x512 RGBA, circular alpha mask)                                                                                                                                                                                                                                                                                                                                                                                                  |
| Legacy coral fails AA as a fill                | Contrast calculation over the legacy token from `apps/9router/src/app/globals.css`     | `#E56A4A` with white text measures 3.23:1, below the 4.5:1 floor, which is why the light theme uses `#B8412A`                                                                                                                                                                                                                                                                                                                                                                                                              |
| SvelteKit needs an adapter, and Bun can run it | SvelteKit adapter documentation, the Bun SvelteKit guide, and the Bun adapter template | A production SvelteKit server comes from an adapter; Bun documents building SvelteKit apps and a Bun adapter exists, so serving the panel and proxying `/api/v1` from the panel's own Bun process (§10.1) is a supported setup, not a workaround.                                                                                                                                                                                                                                                                          |

Path status for every path referenced in this document:

| Path                                      | Status                                                                                                                                                                                              |
| ----------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `docs/SPEC-API/001-SPEC-API.md`           | Present                                                                                                                                                                                             |
| `docs/RULLES/TDD.md, & OWASP.md`          | Present                                                                                                                                                                                             |
| `README.md`                               | Present                                                                                                                                                                                             |
| `docs/SPEC-API/002-SPEC-API-openapi.md`   | Superseded 2026-09-21: the machine-readable contract is served as `GET /api/v1/openapi.json`, generated from `docs/CONTRACT/001-CONTRACT-API-V1.yaml`. See §14 Q3.                                  |
| `docs/CONTRACT/001-CONTRACT-API-V1.yaml`  | Present. The contract of record the served document is generated from; `scrypts/gates/contract-openapi.sh` fails when the two drift.                                                                |
| `AGENTS.md`                               | Present at the repository root, scoped to Go services (`app-*/**`). It does not govern `app-ui`.                                                                                                    |
| `app-ui/`                                 | Present. All 19 sidebar leaves are routable and no row is `Planned`; the U0 to U3 screens are built, and draft 007 records each readiness finding. The recorded browser click-through is the outstanding verification (§12).                                          |
| `skills/`                                 | Present 2026-09-21: seven documents, `skills/<id>/SKILL.md` for `pannelAI` and the six capability skills. This is the path `GET /api/v1/skills` derives both of its addresses from (§6.10, §14 Q2). |
| `app-serv/`                               | Present, past P0. The auth and gateway-key endpoints the panel calls now exist, so U0 can be exercised against it; the recorded click-through (§9.4.5) is the remaining verification step.          |
| `scrypts/`                                | Present: gates and git hooks, see `scrypts/README.md`.                                                                                                                                              |
| `DESIGN.md`                               | Present at the repository root, added 2026-09-17                                                                                                                                                    |
| `app-ui/static/logo-mark.png`             | Present, cropped from the owner's `assets/static/logo.png`                                                                                                                                          |
| `app-ui/static/fonts/InterVariable.woff2` | Present, self-hosted (344 KB), because a font CDN is excluded by §11.7                                                                                                                              |
| `002-TOKEN-SAVER` (native saver spec)     | Planned, per SPEC-API §10 P3                                                                                                                                                                        |

## 16. antislop gate record for this document

Mode: DURING while writing, AFTER as this record. This is a specification, not a rendered interface, so
the table separates what a document can be checked for from what can only be checked once a screen exists.
A deferred item names the check and where the evidence must appear.

**Document-level checks**

| Rule                                                                         | Result                     | Evidence                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| ---------------------------------------------------------------------------- | -------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| R-02 (no em dash)                                                            | PASS                       | Verified 2026-09-16: zero occurrences of the em dash character (U+2014) in the file, and zero en dashes (U+2013). The title separator is a colon, and negative terms such as "Not ported" use plain wording.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| R-17 (numbers need a source)                                                 | PASS                       | Every count appears in §15 with the command that produced it. No figure is carried over from the reference README or the API spec without a citation.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| R-24 (no navigation without a destination)                                   | PASS                       | §5.2.2 makes it a rule, and it is now enforced by the data model rather than by review: a navigation row carries an `href` only when a route file exists, so a dead link is not representable. `tests/navigation/navigation.test.ts` cross-checks the tree against the routes discovered on disk. Re-verified 2026-09-18 with all 19 owner rows in the sidebar: 3 rows are real links (`/endpoint-keys`, `/changelog`, `/settings`), 16 are labelled `Planned`. Measured again 2026-09-21, after `/playground`: 20 nodes, 19 routable leaves (13 static routes plus the 6 media kinds), 0 rows labelled `Planned`, and 1 container. No row is inert any more, so the rule has nothing left to hold back. The one `planned` flag still in the file sits on the Media Provider container, which renders as a disclosure, and the test asserts the flag only for a node with no route and no children.                    |
| R-38 (real content or honest placeholder)                                    | PASS                       | §15 marks every path present or planned, and the entries are updated rather than left stale. The OpenAPI companion is no longer missing: §14 Q3 closed 2026-09-21 with the contract served at `GET /api/v1/openapi.json`, and `/api-docs` renders that document rather than a copy of the spec tables (§6.12). No sidebar row is labelled `Planned` as of 2026-09-21: Skill, Changelog, API Docs, and Playground Chat all ship as links, the `/skills` screen asks each source address before it offers a copy control rather than printing a link it has not confirmed (§6.10), and the playground states a fact the wire did not provide as `not stated` rather than filling it in (§6.15). The changelog renders the releases the gateway serves rather than an empty list that claimed no source existed, and its empty state names the route that answered with an empty list (§6.16, §14 Q11 closed 2026-09-21). |
| R-36 (no fabricated claims)                                                  | PASS                       | The document makes no security, compliance, uptime, or performance claim about the panel. §7.1.3 explicitly denies a security-boundary claim, and the Bun runtime claim in §10.1 exists in §15 with a source rather than as an assertion about speed.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| Governance links are real                                                    | PASS                       | Both linked files resolve: [`docs/RULLES/TDD.md`](../RULLES/TDD.md) and [`AGENTS.md`](../../AGENTS.md). Each link states its scope, so a reader cannot mistake the Go rules for the panel's rules.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| Mandatory protocol stated, not implied                                       | PASS                       | §10.2 states the test-first order, the analysis content, the table-driven requirement with case types, and the pull request evidence, all bound to `docs/RULLES/TDD.md` §2.3, §2.5, and §3.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| Cross-spec consistency                                                       | PASS                       | The one contradiction between this spec and SPEC-API was the `caveman` key, where the API carried a live config field and the panel refused to render it. Both specs now mark it DEPRECATED with the same removal path (`/api/v2`), so an implementer reading either document reaches the same conclusion (§13.6, SPEC-API §7.9).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| R-16 (no marketing vocabulary)                                               | PASS                       | A case-insensitive scan for the forbidden vocabulary returns exactly one line, which is the rule statement in §8.10.3 that names the words to avoid. That is the carve-out for documenting the rule, not a claim about the product.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| Emoji in the document                                                        | PASS                       | A Unicode-range scan for emoji returns 0 matches. This matches the owner requirement that no emoji appear in UI strings, and the document holds itself to the same rule.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| Inline bold labels in bullet lists                                           | Disclosed, LOW, deliberate | 90 bullets carry a bold label, for example a screen field name such as the data source or the empty-state text. In a specification this is a definition list: a reviewer scans labels to find a field. The reason is recorded here so the pattern is not mistaken for accidental template text, and it is listed in the follow-up audit for the owner to accept or reject.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| R-23 / R-37 (no invented assets, direction required)                         | PASS                       | No logo, avatar, statistic, or testimonial is invented anywhere. §9.1 records that direction now exists in `DESIGN.md` and states where it wins over this spec. The one asset question R-23 raises is closed by the owner supplying the logo, and it is cropped rather than redrawn.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| R-18 (testimonials), R-28 (FAQ), R-05 pricing block, R-14 feature-card grids | Not applicable             | The KEEP list has no marketing surface. These sections are excluded by scope, so they cannot be violated.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| C-5 (evidence over claims)                                                   | PASS                       | §15 and the per-screen constraints cite the API spec section for each behaviour instead of asserting panel behaviour the API does not have.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |

**Checks deferred to implementation, with the required evidence**

| Rule                                                         | Where it is enforced                               | Required evidence                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| ------------------------------------------------------------ | -------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| R-25 (contrast)                                              | §9.4.1, and `app-ui/tests/tokens/contrast.test.ts` | Satisfied for the shell on 2026-09-18: 50 assertions over both authored themes, covering the 14 text pairs, the graphic pairs, the per-theme accent-derivation of the active-row wash, typography, and palette hygiene. Lowest text pair 4.87:1 against the 4.5:1 floor. Measured on the composited colour, so an accent wash is measured against what a reader receives rather than against its raw channel triple. The test reads `src/app.css`, so a palette edit re-measures itself. Each new screen adds its own pairs to that file: the changelog added the warn marker on a release row (§6.16), which brings the file to 52 assertions as measured on 2026-09-21. |
| R-27 (empty, loading, error states)                          | §8.3, and each screen section                      | Screenshot or recording of all three states per screen. `/endpoint-keys` already implements all three and they were observed in the 2026-09-17 pass; the remaining screens owe the same.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| R-32 (keyboard)                                              | §9.4.4                                             | Satisfied for the shell on 2026-09-17: recorded tab order through the sidebar and header, every stop with a 2px accent outline, Enter navigates a row, Enter and Space toggle the media disclosure, Escape closes the mobile drawer, and focus returns to the header control. Each new screen owes the same pass.                                                                                                                                                                                                                                                                                                                                                         |
| R-03 (mobile)                                                | §8.7, §9.4.3                                       | Satisfied for the shell on 2026-09-17: 390px, 820px, and 1440px exercised, no page overflow at any of the three, dark and light both. Tablet shows the 64px rail, mobile shows the drawer at 293px with an overlay, and the drawer closes on navigation. Each new screen owes the same pass.                                                                                                                                                                                                                                                                                                                                                                              |
| R-34 (both themes work)                                      | §8.9, §9.4.2                                       | Satisfied for the shell on 2026-09-17: both themes at all three breakpoints, body background measured as `rgb(252, 250, 247)` light and `rgb(26, 25, 23)` dark, no overflow in either. Each new screen owes the same pass.                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| R-26 (no dead controls)                                      | §13.10, §5.2.2                                     | Satisfied for the shell on 2026-09-17: the navigation test asserts that a row without a route carries no `href`, and the click-through confirmed every rendered control acts. The one control that had none (a planned sub-item rendered as a link without a destination) was found in this pass and rewritten as an inert row.                                                                                                                                                                                                                                                                                                                                           |
| R-35 (verified before delivery)                              | §9.4.5                                             | Satisfied for the shell on 2026-09-17. The recorded click-through is in `app-ui/README.md`, and it includes the two defects it found: the token layer was never imported, so the entire panel rendered unstyled while the contrast test passed, and three component files carried their header comment outside the script block, so Svelte rendered the comment as page text.                                                                                                                                                                                                                                                                                             |
| R-04 (icon relevance)                                        | §8.11.2                                            | Satisfied on 2026-09-17: `src/lib/icons.ts` holds one entry per navigation row with a written reason, and `tests/navigation/icons.test.ts` fails the build on an icon with no row, a row with no icon, a reason under 20 characters, or a banned glyph.                                                                                                                                                                                                                                                                                                                                                                                                                   |
| R-21 (theme choice)                                          | §8.9                                               | Both authored themes, with the dark default rationale recorded.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| R-29, R-11, R-12, R-13, R-01, R-09 (purpose-gate techniques) | §9.3                                               | Token file showing the palette cap, the radius scale, and the single accent use.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| R-31 (reason per decision)                                   | §9.5                                               | The reason log, extended in the pull request for decisions this spec does not yet cover.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |

---

_Changelog 2026-09-21: the Changelog screen reads the release notes the gateway serves, closing §14 Q11._

_`GET /api/v1/changelog` (SPEC-API §7.18) serves the five releases the binary carries, newest first, and
`/changelog` now renders them: each release as its version, its calendar date, its title, and its note, with
the `Running` / `Newer` / `Installed` marker computed against `GET /api/v1/version`. §6.16 is amended from
the shape the section specified while no source existed (`category` plus `items[]`) to the shape the route
serves, because a panel-side mapping would have had to invent the category and split a paragraph into
bullets nobody wrote. Two smaller defects went with it: the page's route comment cited §6.18 instead of
§6.16, and `app-ui/README.md` still stated that no release source existed. The screen also gained two honest
answers it did not have: a running version that cannot be compared (a `-dev` build reports a value that is
not a release number) gets its own sentence instead of "this build is the newest release listed", and the
empty state names the route that answered with an empty list rather than a source the panel is missing. The
live pass read the served notes through the panel's own schema and derivations, 18 checks with 0 failures,
including four controls._

_Changelog 2026-09-21: the Playground Chat screen is built, and the credential path it needs is a server
route rather than a browser one._

_`/playground` now exists, so §5.1's Playground Chat row carries an `href` and the sidebar has no Planned
row left. The screen sends one message through the data plane and shows what the gateway answered: a model
picker fed by `GET /api/v1/models`, a message box, a send control with the cost sentence beside it, a
streamed answer with a Stop control, the facts §6.15 rule 4 asks for, and a disclosure holding the frames
exactly as they arrived. The request streams, so the screen reads server-sent events frame by frame; a
stream that ended on the `[DONE]` sentinel, one the operator stopped, and one that closed without the
sentinel get three different sentences, because the third means the answer may be cut off._

_The injection path is the part that was not a page. Two panel routes outside `/api/v1`
(`GET /playground/models`, `POST /playground/chat`) perform the data-plane call, read
`PANEL_PLAYGROUND_KEY` from the server environment, and attach it to the outbound request in one function.
They answer in the panel's own failure envelope rather than forwarding the gateway's, because both planes
use the code name `UNAUTHORIZED` and a forwarded 401 would sign the operator out for a gateway key problem;
the gateway's own error object travels inside the panel's so a developer still sees the machine code. Every
route asks `GET /api/v1/auth/status` with the caller's cookie first, so the session's truth stays in
`app-serv`'s store and an unauthenticated browser cannot spend the panel's key._

_Rule 1 of §6.15 is held by tests rather than by review, which is the part worth recording: the variable's
name may only appear in the schema that declares it and in `src/lib/server/`, the credential is written in
exactly one function, the browser's own files name neither a store nor a credential header, and a forwarded
sentence has the key redacted out of it. The production build was grepped for the variable's name and the
client bundle does not contain it. A panel without a key is a supported state rather than a boot failure:
the screen renders the unavailable state using the sentence the panel's own route wrote, which is the only
place allowed to name the variable, and renders no send control (R-26)._

_§6.15 was rewritten from a decided model into a built one, §2.1 and §5.1 assign the row its U3 phase,
§12's U3 row and status paragraph carry the screen, and §14 Q7 is closed. §15's sidebar count was
re-measured and §16's R-24 and R-38 rows now read 19 routable leaves and 0 planned rows._

_Gates pass on 2026-09-21: `bun run test`, `svelte-check` clean, Prettier and ESLint clean, and a
production build. The wire half was verified in four phases against a booted `app-serv` built from
committed `HEAD` (another actor held the working tree) with a gateway key minted for the pass: the fixture
provision, the unset-key path (5 checks: 503 naming the variable, and neither the gateway nor the upstream
stub dialed at all), the keyed path (23 checks: the session gate, the models read, the streamed answer with
its facts, the empty-body refusal, no response carrying either credential, and the stub log proving the
endpoint key and `stream_options.include_usage`), and the refused-key path (4 checks: `GATEWAY_KEY_REFUSED`
with the gateway's `UNAUTHORIZED` inside, and the panel session untouched). A fifth phase pointed a second
panel at a frame-by-frame stub, because the gateway's own buffering would otherwise hide the relay: the
answer arrived over five reads 600 ms apart and the reader ended `done`._

_Three `app-serv` defects came out of that pass and are recorded here as requests to that actor, not
absorbed by the panel. `mustFrame` in `internal/dataplane/translate_stream_openai.go` marshals a payload
without framing it, so the stream carries a bare JSON object and the terminal `data: [DONE]` is glued to
it with no blank line, which is why a spec-correct reader reports the stream as truncated.
`internal/handler/chat.go` commits HTTP 200 and the SSE content type before the model is resolved, so a
streamed request for an unknown model answers 200 with an error body under an SSE type where the
non-streamed path answers 400. And the gateway writes the whole answer at once, so its `sseSink.Flush()`
never reaches the client and nothing streams. The live database was restored to its exact baseline
(`usage=2 logs=1 keys=0 endpoints=0 upkeys=0 nodes=0 caps=0 settings=1 auth_null=true media_settings=0
proxies=0`) and the pass's temp files were removed._

_Resolved 2026-09-22: the third defect above is closed by `app-serv` draft 010 F5. Every middleware
wrapper now forwards `Flush()` and `Unwrap()`, and the SSE sink flushes through
`http.ResponseController`, so a frame reaches the client while its handler is still open. The evidence
is permanent (`internal/router/router_stream_flush_test.go`, first frame before the handler releases) and
live (`cmd/app-serv/playground_live_flush_test.go` against a throwaway PostgreSQL + Redis and a spaced
upstream: first frame 16.7 ms with the fix, 770.7 ms without it, measured on the same stack). The first
two defects remain open with `app-serv`, and this paragraph keeps recording them as requests rather than
absorbing them into the panel._

_The browser click-through is still outstanding, as it is for U0, U1, and U3's Skills half._

---

_Changelog 2026-09-21: the API Docs screen renders the served contract, closing §14 Q3._

_`/api-docs` now exists, so §5.1's API Docs row carries an `href` instead of a Planned chip. The
screen renders `GET /api/v1/openapi.json` rather than a written-out copy of it: the base URL from the
document's own `servers` block, one section per tag the document declares, a row per operation with
its method, path, summary, and credential, one composed `curl` example per section, and one
error-code table per plane in `x-contract.planes`. A block the document does not carry is stated as
absent, because an empty space reads as a rendering fault rather than as a fact about the contract._

_Nothing on the screen is a second copy. The catalog order is the document's path and tag order, the
credential wording is derived from each scheme's own declaration, the example is composed from the
operation's method and path, and an error meaning is the description of the response whose body is
that plane's envelope. A document that grows a tag, a path, or a code renders with no edit to the
panel._

_The document grew between the draft that planned this screen and the tree it reads. §6.12's
amendment of 2026-09-21 records the change: 67 paths, 93 operations, 23 tags, 161 schemas, 18
responses, and an `x-contract` block naming each plane's codes and its envelope, where the draft
recorded a document with no schemas, no error table, and no phase markers. The phase markers are
still absent and are no longer needed: the document is generated from
`docs/CONTRACT/001-CONTRACT-API-V1.yaml` and pinned to the registered routes by
`TestOpenAPICoversEveryRegisteredRoute`, so a group on screen is a route that answers._

_Q3 asked whether the machine-readable contract existed. It does, and it is served rather than filed.
§15's path rows now name the contract of record instead of the superseded OpenAPI spec file._

_Gates pass on 2026-09-21: 1868 tests across 79 files, `svelte-check` clean, Prettier and ESLint clean,
and a production build. The wire half was verified against a booted `app-serv`: the live document parsed
through the panel's own schema and every derivation was driven over it, 31 checks, 0 failures. The browser
click-through of the new screen is still outstanding, and `app-ui/README.md` records it as such: the
render half is covered by the jsdom test and the wire half by the pass above._

---

_Changelog 2026-09-18: Combos and the Vision Adapter, U1 slice three._

_`/combos` now exists, so §5.1's Combo & Vision Adapter row carries an `href` instead of a Planned chip.
Tab one renders the combo table and its editor; tab two renders the vision adapter form and states its own
scope, because the reference shipped four adapters and this port ships one._

_The editor mirrors the strategy rules the API enforces in its domain layer, so a save is refused before
the round trip: `sticky_limit` belongs to `round_robin` and `judge_model` to `fusion`, and §6.4's "hidden,
not disabled" is what the form does. The body it sends carries only the fields the chosen strategy reads,
so a combo that changed strategy cannot carry a value the new one refuses._

_Reordering the model list has two paths to one operation: a pointer drag and Up and Down buttons that
work from a keyboard or a touch screen. Each move renumbers the priorities rather than swapping two of
them, because the numbers are the stored form of the order._

_One bug was found and fixed outside this slice's scope. `apiRequest` assigned the request body only inside
its `bodySchema` branch, so every route with a body and no schema sent nothing at all. That included
`POST /endpoints/{id}/test` from slice one. The body is now whatever the caller passed, with `bodySchema`
narrowing it, and two tests hold the behaviour._

_Two forms also lost their native `min` and `max` attributes. A native bound blocks the submit before the
schema sees the value, so the message the operator reads would be the browser's, in the browser's language,
which is a second validator the panel cannot keep in English. The bounds are stated in the field hints and
enforced by Zod._

_Gates pass on 2026-09-18: 769 tests across 27 files, `svelte-check` clean, Prettier and ESLint clean, and a
production build. §15's counts were re-measured._

---

_Changelog 2026-09-18: the Providers screens, U1 slice two._

_`/providers` and `/providers/[provider_id]` now exist, so §5.1's Provider row carries an `href` instead
of a Planned chip. The list renders the registry with the server's paging and the category filter the API
accepts; the detail renders the transport defaults beside the facts, the model catalog, and the
provider-scoped endpoint list that reuses the §6.2 drawer._

_Two §6.3 requirements are deliberately not built, and both are API limits rather than unfinished panel
work. The list's search over name and ID cannot be sent: `ProviderListQuery` declares only `category` and
`routability`. The catalog's "suggested" toggle has nothing to filter: `ProviderModelsFrom` sets
`Suggested: true` on every row because the registry carries no suggestion flag. Each would be a control
that cannot do what it says (R-26), so each is absent and recorded in §14 Q12 and Q13._

_The catalog therefore reads `GET /models/catalog` rather than `GET /providers/{id}/models`, because that
is the route carrying the `provider_id`, `capability`, and `q` parameters §6.3's search and two capability
filters need. The provider-scoped model shapes were removed with it rather than left unused, and a new
`src/lib/schemas/model.ts` holds the §7.6 shapes. `src/lib/navigation.ts` gained `NavHref`, which subtracts
parameterised routes from the generated `RouteId`, so a sidebar row cannot link to `/providers/[provider_id]`
without the parameters `resolve` needs._

_Gates pass on 2026-09-18: 634 tests across 24 files, `svelte-check` clean, Prettier and ESLint clean, and
a production build. §15's counts were re-measured._

---

_Changelog 2026-09-18: the U0 remainder, closed on the panel side._

_The not-found view §5.1 assigns to U0 now exists: `src/routes/+error.svelte` names the requested path
for a 404, gives every other status one panel voice without echoing the server's message, and always
offers the way back to `/endpoint-keys`. Session handling gained the §8.1.2 behaviour it was missing: an
unauthenticated visit carries the route it wanted to `/login` and returns there after signing in, with
the destination validated by `src/lib/utils/redirect.ts` so a crafted `redirectTo` cannot leave the
origin or bounce back to the login screen._

_The sidebar was hardened against its own design direction: the dark active-row wash now derives from the
accent instead of the warn colour, a tablet expands over the content instead of pushing it (§8), the
navigation is a named `nav` landmark, and one module owns the preference cookie. That last item is also
where a boundary was corrected: the first attempt removed the write from the primitive's provider, which
§10.7 does not permit, so the primitive was restored and the duplication removed from the application
side instead. The one primitive edit that rule does allow, the 264px sidebar width, is recorded in
`DESIGN.md` §11._

_§12 gained a U0 status note, §14 Q8 was closed on both governance follow-ups, and §15 was corrected
where its counts and status rows had gone stale. The panel's gates pass on 2026-09-18: 499 tests across
19 files, `svelte-check` clean, Prettier and ESLint clean, and a production build. U0's remaining exit
step is the recorded click-through against a running `app-serv`, which is a verification run rather than
unbuilt work._

---

_Changelog 2026-09-17: sidebar, design direction, and three owner additions._

_The owner supplied the direction that §9.1 was waiting for: legacy structural parity, delivered fresh,
with mobile, tablet, and desktop behaviour as an explicit requirement. `DESIGN.md` was written the same day
and holds the identity, palette, typeface, radius and elevation scale, identity motif, and reason log, so
the "draft without direction" state is closed and §9.1 and §9.2 were rewritten to point at it. The logo was
supplied at `app-ui/assets/static/logo.png` and cropped to a circular mark. The light accent was re-tuned
from the legacy coral because the legacy value measures 3.23:1 with white text and fails R-25; §9.1 records
the measurement and `DESIGN.md` §3.1 records the decision._

_The sidebar was rebuilt: 19 owner rows in five task groups, and the table in §5.2 now states the grouping
rather than listing items flat. §5.2.2 was strengthened from "an unbuilt item is absent" to "an item
carries an `href` only when a route file exists", which turns R-24 from a review rule into a type-level
one. §8.7 was rewritten for three responsive shapes, replacing the previous two, because the tablet band
previously received a drawer and wasted the space it has. §8.11 was rewritten with the shipped identifier
per icon, the icon set corrected to `@lucide/svelte` (the older `lucide-svelte` package is deprecated on
npm), and two rows added._

_Three owner additions. Console Log was split out of `/logs` into its own route, with the original tab left
as a pointer in §6.11 and the screen specified in §6.14; its endpoints are unchanged, so only the route and
the spec text moved. Playground Chat and Changelog were added to the sidebar and cannot be built yet, so
§6.15 and §6.16 record the shape and the blocking questions instead of a specification: the playground
crosses the session-versus-gateway-key auth boundary (§14 Q7), and Changelog has no endpoint at all
(§14 Q11). Both rows carry `Planned` and no route, so R-24 and R-26 hold._

_§11.2 now names three structural assertions as tests rather than habits: the navigation tree against the
routes on disk, the colour pairs against the AA floor, and the sidebar's initial open state against the
viewport band. §10 gained two boundaries, both from defects this work found: `src/app.css` is imported once
from the root layout, because in U0 it was written and tested but never imported, so the panel rendered
unstyled while its contrast test passed; and `src/lib/primitives/` is generated code, with the one token
change recorded rather than assumed to survive a regeneration. §15 was updated where a path moved from
missing to present, and §16 records the evidence for the checks that are now satisfied rather than
deferred._

---

_Changelog: 2026-09-16, initial draft. Mapped from the KEEP list against SPEC-API-001 and the reference
`~/ai-gateway` at `9766494` (9router-app 0.5.55). Owner decisions recorded the same day: Headroom kept in
Token Saver, `/antislop` AI and SuperPowers on their own Skills tab, shadcn-svelte as the component layer
with fresh icons and no emoji, and Zod as the single strict validation and sanitization layer in `app-ui`.
Zod section strengthened the same day at the owner's request: four validation boundaries, module layout and
reuse rules, environment parsing, and a per-resource schema coverage checklist. Also added the same day at
the owner's request: explicit governance links to `AGENTS.md` and `docs/RULLES/TDD.md`, a mandatory strict
TDD protocol section (§10.2), and a Bun runtime section (§10.1). `AGENTS.md` landed the same day, scoped
to the Go services, so the earlier blocking note was withdrawn and a question about adopting its header
format was opened instead. Caveman resolved the same day: marked DEPRECATED in this spec and in SPEC-API, with removal
scheduled for `/api/v2`, replacing the earlier half-state where one document carried the key and the other
only mentioned it._
