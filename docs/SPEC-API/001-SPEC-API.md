# SPEC-API-001 — pannelAI Gateway API v1

| | |
|---|---|
| **Spec ID** | 001-SPEC-API |
| **Status** | Draft → Review |
| **Date** | 2026-09-11 |
| **Organization** | KENTANG TECH |
| **Author** | Dodi Rusmana <rusmanadodi@kentangtech.com> |
| **GitHub** | https://github.com/rusmanadodi2598 |
| **Source Rewrite (Reference)** | `/home/rusmanadodi/ai-gateway` (9Router, Next.js monolith) |
| **Target** | `pannelAI/` → `app-serv/` (Go API) + `app-ui/` (Svelte panel) |
| **Governance** | `AGENTS.md` (root) — mandatory for all implementation work |
| **Companion spec** | [`docs/SPEC-UI/001-SPEC-UI.md`](../SPEC-UI/001-SPEC-UI.md) (panel contract; wire shapes remain normative here) |

> **SPEC FIRST.** This document is the contract of record for API v1. Implementation starts only against
> what is written here. Any deviation requires editing this spec in the same PR. The machine-readable
> OpenAPI 3.1 companion — [`002-SPEC-API-openapi.md`](./002-SPEC-API-openapi.md) — renders this contract
> (§4, §6, §7, §8) as a complete YAML document (79 operations, 119 schemas, Redocly-clean); it is
> normative for wire shapes, this document is normative for semantics.

---

## 1. Purpose

Rewrite the 9Router AI gateway as a clean two-app system:

- **`app-serv`** — Go 1.26 API server. Management plane + OpenAI-compatible data plane. PostgreSQL + Redis.
- **`app-ui`** — Svelte panel (Bun toolchain). Consumes `app-serv` management APIs only. No direct DB access.

The reference is a single Next.js app (SQLite, JS). We port the **routing engine concepts**, not the code:
format translation, multi-provider registry, combos/fallback, quota tracking, token savers, logging.

## 2. Scope

### 2.1 KEEP (ported into v1)

| Feature | Reference source | v1 owner |
|---|---|---|
| Endpoint & Keys (client-facing gateway keys) | `src/app/api/keys/*`, `apiKeysRepo.js` | app-serv |
| Providers (registry + upstream endpoints) | `open-sse/providers/registry/*` (120+ entries), `connectionsRepo.js` | app-serv |
| Multi API Key per upstream endpoint | extension of `connectionsRepo.js` (1 conn = 1 key) → **N keys per endpoint** | app-serv (new) |
| Combo & Vision Adapter | `open-sse/services/combo.js`, `capacityAdapter.js`, `settingsRepo.capacityAdapter.vision` | app-serv |
| Usage | `src/app/api/usage/*`, `usageRepo.js` | app-serv |
| Quota Tracker | `quotaAutoPing.js`, provider quota windows | app-serv |
| Token Saver | RTK / Headroom / Caveman / Ponytail flags (`settingsRepo.js`) | app-serv (config only in v1; `caveman` is DEPRECATED per §7.9; native + customizable engine is a later spec) |
| Media Provider | `src/app/api/media-providers/*`, `sse/handlers/{tts,stt,imageGeneration,videoGeneration,search,embeddings}.js` | app-serv |
| Proxy Pools | `src/app/api/proxy-pools/*`, `proxyPoolsRepo.js` | app-serv |
| Logs | `requestDetailsRepo.js`, `consoleLogBuffer.js`, `ENABLE_REQUEST_LOGS` | app-serv |
| English | all client-facing strings/errors in English | both |
| Settings | `settingsRepo.js` (subset, see §7.14) | app-serv |

### 2.2 NOT PORTED (left out on purpose — do not spec, do not build)

Cloud sync, tunnels (Cloudflare/Tailscale), MITM router, pxpipe, CLI-tools config writers
(`src/app/api/cli-tools/*`), translator playground page, skills, basic-chat page, headroom process
manager (start/stop/restart — v1 only calls an external Headroom URL), proxy-pool cloud deployers
(Cloudflare/Deno/Vercel), SAML/OIDC/SSO, MCP, shutdown/update endpoints, i18n framework.
Anything not listed in §2.1 is assumed **not wanted**; it may be added later via a new spec only.

## 3. Architecture & Data Flow

```
CLI tools (Claude Code, Codex, Cursor, ...)          Browser (app-ui)
        │  OpenAI/Anthropic/Gemini wire format               │  fetch /api/v1/... (session cookie)
        ▼                                                    ▼
┌────────────────────────────── app-serv (Go) ─────────────────────────┐
│  Data plane   /api/v1/chat/completions, /messages, /embeddings, ...  │
│  Mgmt plane   /api/v1/{providers,endpoints,keys,combos,...}          │
│  layers:      schema → domain → repository → service → handler → router │
└───────┬─────────────────────────────┬────────────────────────────────┘
        │ upstream HTTPS (per-provider format, proxy pool aware)       │
        ▼                                                             ▼
  Provider upstreams (api_key / oauth / no-auth)              PostgreSQL (state)
                                                       Redis (rate limit, sticky RR,
                                                       key circuit state, quota counters,
                                                       login limiter, session revocation)
```

- Reference storage was SQLite (`src/lib/db/*`); v1 uses **PostgreSQL** (schema in `app-serv/migrations/`).
- OAuth token refresh, quota re-check, log flushing run as **workers** with explicit retry policy and
  panic recovery (`AGENTS.md` §1.6).

## 4. General Conventions

| Topic | Decision |
|---|---|
| Base URL | `http://localhost:8080` (dev), configurable via typed `Config` env struct (fail fast at boot) |
| Versioning prefix | **`/api/v1`** for everything (management + data plane). Breaking change ⇒ `/api/v2`, old version kept ≥ 1 release cycle with `Deprecation` + `Sunset` headers |
| JSON | `snake_case` keys. IDs are ULID strings with type prefixes (`ep_`, `uky_`, `gky_`, `cmb_`, `prx_`). Timestamps RFC3339 UTC. Money/cost as decimal strings (`"cost_usd":"0.0042"`), tokens as integers |
| Management auth | Dashboard session: `POST /api/v1/auth/login` (password, bcrypt hash in DB) → HttpOnly cookie (HMAC-signed, Redis-tracked, revocable). Login is Redis rate-limited (mirrors reference `loginLimiter.js`) |
| Data plane auth | `Authorization: Bearer <gateway key>` (`sk-...`). Enforced when `settings.security.require_api_key = true` (default `true`) |
| Key masking | Keys are never returned in full after creation; responses carry `key_hint` = `"sk-…abcd"` |
| Management error envelope | `{"error":{"code":"NOT_FOUND","message":"..."}}` — `code` machine-parseable, `message` English (AGENTS §1.3) |
| Data plane error envelope | OpenAI shape for client compatibility: `{"error":{"message":"...","type":"invalid_request_error","code":"..."}}` |
| Pagination | `?page=1&per_page=25` (max 100) on list endpoints → `{"data":[...],"meta":{"page":1,"per_page":25,"total":123}}` |
| Streaming | SSE passthrough (`text/event-stream`), heartbeat disabled by default, `X-Accel-Buffering: no`; usage chunk emitted when `stream_options.include_usage=true` |
| Timeouts | Every upstream call runs under `context` timeout: connect 10s, total 120s default (streaming: no total cap, idle-read 300s); per-provider override in registry |
| Retries | Default `{max_attempts:3, backoff:"exponential", jitter:true}` on 429/5xx/network; per-provider override; never retry non-idempotent POST bodies with side effects beyond 1 attempt |
| Token saver bypass | Request header `X-Token-Saver: off` skips all savers for that request (renames reference `X-9Router-Token-Saver`) |
| Rate limiting | Gateway-level limit on all public endpoints (Redis token bucket); login endpoint stricter; limits documented in config, not hardcoded |
| Idempotency | Data plane is naturally idempotent-free (proxy); OAuth callbacks carry `state` replay-guard via Redis (single-use, TTL 10m) |
| Observability | `slog` structured logs, every request tagged with `request_id`; internal logs may be Indonesian, client payloads never |

## 5. Terminology (Ubiquitous Language)

| Term | Meaning | Reference equivalent |
|---|---|---|
| **Provider** | Upstream vendor definition (registry entry: transport, auth, models) | `open-sse/providers/registry/{id}.js` |
| **Upstream Endpoint** | One configured account/connection at a provider | `providerConnections` row |
| **Upstream Key** | One API key credential under an upstream endpoint (multi-key per endpoint — new capability) | `connection.apiKey` (was 1:1) |
| **Gateway Key** | Client-facing key CLI tools use against pannelAI | `apiKeys` row |
| **Combo** | Ordered model list with a strategy used as a model string | `combos` table |
| **Vision Adapter** | Capability fallback: routes image-bearing requests to vision-capable models | `capacityAdapter.vision` |
| **Quota Window** | Usage window per endpoint (5h / daily / weekly / monthly) with reset instant | provider quota tracking |
| **Data plane** | OpenAI-compatible proxy endpoints consumed by CLI tools | `src/app/api/v1/*` |
| **Management plane** | Panel/admin endpoints consumed by app-ui | `src/app/api/*` (non-v1) |

## 6. Data Model (PostgreSQL)

| Table | Key columns | Notes |
|---|---|---|
| `upstream_endpoints` | `id, provider_id, label, auth_type, priority, status, oauth(jsonb, redacted on read), test_status(jsonb), rate_limited_until, created_at, updated_at` | replaces `providerConnections`; unique `(provider_id, label)` |
| `provider_nodes` | `id, type, name, prefix, api_type, base_url, created_at, updated_at` | custom OpenAI/Anthropic-compatible providers (§7.4); `prefix` is **UNIQUE** and must not collide with a registry id or alias |
| `upstream_keys` | `id, endpoint_id→upstream_endpoints, label, value_encrypted, key_hint, priority, status, last_used_at, last_error, consecutive_errors, rate_limited_until, created_at, updated_at` | **N per endpoint**; value encrypted at rest (AES-GCM, key from env) |
| `gateway_keys` | `id, name, value_hash, key_hint, status, last_used_at, request_count, created_at, revoked_at` | SHA-256 hash lookup, plaintext shown once; `name` is **UNIQUE** (duplicate ⇒ `CONFLICT`) |
| `combos` | `id, name, strategy, sticky_limit, judge_model, models(jsonb), created_at, updated_at` | `models`: `[{ref, priority}]` |
| `model_aliases` | `alias, target, created_at` | alias → `provider/model` or combo name |
| `models_custom` | `id, provider_id, model_id, display_name, capabilities(jsonb), created_at` | user-added models |
| `models_disabled` | `provider_id, model_id, disabled_at` | hidden from catalog/routing |
| `usage_records` | `id, request_id, ts, endpoint_id, provider_id, gateway_key_id, model, combo, tokens_in, tokens_out, tokens_cache_read, tokens_cache_write, cost_usd, latency_ms, status, error_code` | partitioned by month; index `(ts)`, `(provider_id, ts)`, `(endpoint_id, ts)` |
| `quota_windows` | `endpoint_id, window, used_units, limit_units, resets_at, source` | window ∈ `5h|daily|weekly|monthly`; counters cached in Redis, flushed to PG |
| `request_logs` | `request_id, ts, gateway_key_id, endpoint_id, model, status, latency_ms, request_body, response_body, error` | bodies stored only when capture enabled; retention job deletes > `retention_days` |
| `proxies` | `id, label, protocol, host, port, username, password_encrypted, enabled, status(jsonb), created_at, updated_at` | protocol ∈ `http|https|socks5` |
| `quota_caps` | `endpoint_id, monthly_cost_usd, monthly_tokens, updated_at` | optional budget caps; a router stops picking an exhausted endpoint (§7.12) |
| `settings` | `key (pk), value(jsonb), updated_at` | typed accessors, defaults merged at read (mirrors `mergeWithDefaults`) |

Provider **registry is static config embedded in the Go binary** (YAML generated from the reference
registry port), not a DB table. DB rows reference registry IDs; unknown provider ID ⇒ `VALIDATION_ERROR`.

## 7. Endpoint Catalog

Legend: **Auth** `S` = session cookie · `K` = gateway key (data plane) · `P` = public.
All management endpoints are session-gated; data plane endpoints are gateway-key-gated when
`require_api_key=true`.

### 7.1 System

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/health` | P | Liveness + DB/Redis reachability | P0 |
| GET | `/api/v1/version` | P | Build version, commit, registry revision | P0 |

### 7.2 Auth (dashboard session)

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| POST | `/api/v1/auth/login` | P | `{password}` → sets session cookie; Redis-limited (5 fails → 15m lockout) | P0 |
| POST | `/api/v1/auth/logout` | S | Revokes session | P0 |
| GET | `/api/v1/auth/status` | P | `{authenticated, require_login, password_configured}` | P0 |
| POST | `/api/v1/auth/change-password` | S | `{current_password, new_password}` | P0 |

### 7.3 Gateway Keys (Endpoint & Key)

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/gateway-keys` | S | List (hints only) | P0 |
| POST | `/api/v1/gateway-keys` | S | `{name}` → returns **full key once** | P0 |
| GET | `/api/v1/gateway-keys/{id}` | S | Detail (hint only) | P0 |
| PATCH | `/api/v1/gateway-keys/{id}` | S | `{name?, status?}` | P0 |
| DELETE | `/api/v1/gateway-keys/{id}` | S | Revoke (soft) | P0 |

### 7.4 Providers (registry + OAuth)

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/providers` | S | Registry list: `{id, name, category, auth_type, routability, endpoint_count, status_summary}`; filter `?category=apikey|oauth|free|media|local`, `?routability=native|connector` | P1 |
| GET | `/api/v1/providers/{provider_id}` | S | Registry detail incl. transport defaults, model catalog summary, `routability` ∈ `native|connector`, and `media` (per-kind base URL and credential placement) | P1 |
| GET | `/api/v1/providers/{provider_id}/models` | S | Full model list; `?suggested=true` returns suggested set | P1 |
| POST | `/api/v1/providers/{provider_id}/oauth/start` | S | `{redirect_uri?}` → `{authorize_url, state}` (OAuth providers only) | P2 |
| GET | `/api/v1/providers/{provider_id}/oauth/callback` | P | Completes flow (validates `state` replay-guard), creates upstream endpoint + tokens; `302` redirect for browsers, `200` JSON body for headless callers (`Accept: application/json`) | P2 |
| GET | `/api/v1/providers/{provider_id}/oauth/status` | S | Token expiry / refresh state per endpoint | P2 |
| POST | `/api/v1/providers/{provider_id}/oauth/refresh` | S | Force token refresh (worker also auto-refreshes at `refresh_lead`) | P2 |

**The callback's two answers.** `GET .../oauth/callback` is the one route in §7.4 that is **public**: the
provider redirects a browser to it, and a browser cannot present the dashboard session cookie for that
redirect. Its replay guard is the single-use `state` (§4), not a session. It answers by audience:

- `Accept: application/json` → `200` with `{endpoint_id, provider_id, label, account_email?, token_hint,
  created}`; a failure is the §8 envelope at its own status.
- anything else (a browser) → `302` to `{origin}/providers/{provider_id}` carrying the outcome as query
  parameters: `?oauth=connected&endpoint_id=ep_…` on success, and
  `?oauth=error&oauth_error=<english reason>` on failure. The reason is the `apperror` message, never a
  wrapped chain.

`origin` is the configured `PUBLIC_BASE_URL` when one is set; otherwise the origin the flow staged from
its `redirect_uri`, and only when that is an absolute http(s) URL. With neither, the callback answers the
JSON shape rather than redirecting to a host it cannot vouch for, which is what keeps this route from
being an open redirect.

**Custom endpoints (owner requirement).** Besides the embedded registry, an operator defines their
own OpenAI-compatible or Anthropic-compatible base URL. A provider node is that definition: it is
not an endpoint, because a node has no credential of its own. Its id carries the type prefix
(`openai-compatible-…` / `anthropic-compatible-…`), and its `prefix` is the model-string namespace
(`prefix/model`), so a node becomes routable exactly like a registry provider. A node prefix that
collides with a registry id or alias is refused (`CONFLICT`): two providers answering to one model
string is unresolvable. Nodes are P1 because the owner list places them beside the registry
providers; the reference implements the same feature at `POST /api/provider-nodes`.

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/provider-nodes` | S | List custom provider nodes; `?type=openai-compatible\|anthropic-compatible` | P1 |
| POST | `/api/v1/provider-nodes` | S | `{name, prefix, type, api_type?, base_url}` → node. `type=openai-compatible` requires `api_type` ∈ `chat\|responses`; `type=anthropic-compatible` refuses `api_type`. `base_url` must be an absolute http(s) URL | P1 |
| GET | `/api/v1/provider-nodes/{id}` | S | Node detail | P1 |
| PATCH | `/api/v1/provider-nodes/{id}` | S | `{name?, prefix?, base_url?}`; a prefix that would collide is refused | P1 |
| DELETE | `/api/v1/provider-nodes/{id}` | S | Remove the node; refused (`CONFLICT`) while an endpoint still references it | P1 |
| POST | `/api/v1/provider-nodes/{id}/test` | S | Validate that `base_url` answers `/models` (or the Anthropic equivalent) with the node's credential | P1 |

### 7.5 Upstream Endpoints & Multi API Keys (core new capability)

An **upstream endpoint** holds **1..N API keys**. Routing picks endpoint by priority, then a healthy
key inside it (priority order, circuit-broken keys skipped). This generalizes the reference 1-connection-1-key model.

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/endpoints` | S | List; `?provider_id=&status=`; sorted by `priority` | P1 |
| POST | `/api/v1/endpoints` | S | `{provider_id, label, auth_type, priority?, keys: [{label?, value}]}` (≥1 key for `auth_type=api_key`) | P1 |
| GET | `/api/v1/endpoints/{id}` | S | Detail incl. keys summary + test status | P1 |
| PATCH | `/api/v1/endpoints/{id}` | S | `{label?, priority?, status?}` — priority change reorders siblings transactionally | P1 |
| DELETE | `/api/v1/endpoints/{id}` | S | Delete endpoint + its keys | P1 |
| POST | `/api/v1/endpoints/{id}/test` | S | Connectivity test; `{key_id?}` targets one key else first active; updates `test_status` | P1 |
| POST | `/api/v1/endpoints/{id}/keys` | S | Add key `{label?, value}` | P1 |
| GET | `/api/v1/endpoints/{id}/keys` | S | List keys (hints only) with health fields | P1 |
| PATCH | `/api/v1/endpoints/{id}/keys/{key_id}` | S | `{label?, value?, priority?, status?}` — value is write-only | P1 |
| DELETE | `/api/v1/endpoints/{id}/keys/{key_id}` | S | Remove key (endpoint must keep ≥1 active key if `auth_type=api_key`) | P1 |

**Multi-account and bulk onboarding (owner requirement).** Every provider is multi-account: an
account is one endpoint, so N accounts are N endpoints under one `provider_id`, distinguished by
label and, for OAuth, by the account identity (`email` / `workspace_id`). Three routes cover the
onboarding paths the owner listed. They are **all-or-nothing**: a batch is validated in full and
applied in one transaction, so a rejected row never leaves a half-imported account list. Each row's
outcome is reported by index, which is why the response carries `results` rather than a bare list.

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| POST | `/api/v1/endpoints/bulk` | S | Create several endpoints in one call. Body `{provider_id, auth_type, endpoints: [{label, priority?, keys: [{label?, value}]}]}`. Returns `{created: [...], results: [{index, id?, error?}]}` | P1 |
| POST | `/api/v1/endpoints/{id}/keys/bulk` | S | Add several API keys to one endpoint. Body `{keys: [{label?, value, priority?}]}`. Duplicate labels within the batch are `VALIDATION_ERROR` | P1 |
| POST | `/api/v1/providers/{provider_id}/oauth/bulk` | S | Import already-obtained OAuth credentials as endpoints (the path for accounts on a machine with no browser callback). Body `{accounts: [{label?, access_token, refresh_token?, expires_at?, scopes?, account?}]}`. `access_token` is encrypted at rest and never returned; the response carries `key_hint`-style summaries only | P1 |

`POST /api/v1/endpoints` also accepts a `keys` array so a single account with several keys is one
request; `/endpoints/bulk` is for several accounts at once. Both go through the same validation, so
the invariant below holds whichever path is used.

Key health model (circuit breaker per key): on repeated upstream auth/429 failures → `consecutive_errors++`;
≥3 ⇒ key marked `error` + `rate_limited_until = now + backoff`; successful call resets. Redis-backed.

### 7.6 Models (catalog, custom, alias, disabled)

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/models/catalog` | S | Merged catalog across providers; `?provider_id=&capability=vision|tools&q=`; excludes disabled | P1 |
| POST | `/api/v1/models/custom` | S | `{provider_id, model_id, display_name, capabilities?}` | P2 |
| DELETE | `/api/v1/models/custom/{id}` | S | Remove custom model | P2 |
| GET / PUT | `/api/v1/models/aliases` | S | `[{alias, target}]`; PUT replaces full set (validated: target exists) | P2 |
| GET / PUT | `/api/v1/models/disabled` | S | `{provider_id, model_id}` list; PUT replaces set | P2 |

### 7.7 Combos

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/combos` | S | List | P1 |
| POST | `/api/v1/combos` | S | `{name, strategy, models:[{ref, priority}], sticky_limit?, judge_model?}` — `ref` = `provider/model`, combo name, or alias (one deref level); strategy ∈ `fallback|round_robin|fusion` | P1 |
| GET | `/api/v1/combos/{id}` | S | Detail | P1 |
| PATCH | `/api/v1/combos/{id}` | S | Update any field | P1 |
| DELETE | `/api/v1/combos/{id}` | S | Delete (references in aliases must be cleaned first → `CONFLICT`) | P1 |
| POST | `/api/v1/combos/{id}/test` | S | Sends a 1-token ping through the combo chain, reports per-model result | P2 |

Strategy semantics (ported from `combo.js`):
- `fallback` — try models in priority order until success or all exhausted.
- `round_robin` — distribute; `sticky_limit` = consecutive requests kept on one model before rotating.
- `fusion` — fan out to N models, `judge_model` synthesizes the final answer.

The fusion execution the reference fixes, and the gateway implements:
- Panel calls are **forced non-streaming and carry no tools** (the judge needs complete prose, and a member
  that called a tool would return none). The panel answers are read back in the client's own wire format.
- The judge receives the **client's original request plus one appended user turn** carrying the directive —
  so the client's stream flag and tool declarations survive to the served call. Sources are anonymized
  (`[Source N]`) and the judge is told not to mention the panel.
- Degradation: **0 panel answers** is an upstream error (the panel's own failure is reported, so a
  validation failure stays a validation failure); **exactly 1 answer** is served directly, with no judge
  call — and a client that asked for SSE is re-issued the surviving member with its own request, because a
  streamed client must receive a stream rather than the panel's prose-only answer; a member that no longer
  resolves costs its own slot, not the request.
- A member's tokens are spent but are **not folded into the served request's usage row**, which records the
  judge's accounting — one row per request stays the accounting unit.

`round_robin` execution: the engine asks `ComboService.Order` — through the `dataplane.ComboOrderer` seam —
for the order, which reaches the rotation store with the **combo's own `sticky_limit`** (not the data plane
default); it serves the returned leader and fails over through the rest of the rotated order. Rotation is an
optimisation: an order that cannot be produced, or one of a different length, serves the stored priority
order instead of failing the request.

### 7.8 Vision Adapter

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/vision-adapter` | S | `{enabled, round_robin, models, updated_at}` | P1 |
| PUT | `/api/v1/vision-adapter` | S | Replace config; `models` entries validated against catalog (`vision` capability) | P1 |

Behavior (ported from `capacityAdapter.vision`): when a request carries image content and the resolved
model lacks vision, the router prepends adapter models (respecting `round_robin`) and strips the
adapter model from the response identity. v1 ships **vision only**; the adapter framework is
capability-generic internally (pdf/audio/video adapters from the reference are **not** ported).

### 7.9 Token Saver (config only in v1)

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/token-saver` | S | Full config (below) | P2 |
| PUT | `/api/v1/token-saver` | S | Replace config (validated) | P2 |

```json
{
  "rtk":      { "enabled": true,  "filters": ["git-diff", "grep", "smart-truncate"] },
  "headroom": { "enabled": false, "url": "http://localhost:8787", "compress_user_messages": false },
  "caveman":  { "enabled": false, "level": "full" },
  "ponytail": { "enabled": false, "level": "full" }
}
```

- `rtk.enabled` v1 = pass-through flag consumed by the native engine **once built** (later spec:
  native, customizable filter pipeline — explicit user decision, not ported from reference JS).
- `headroom` calls the external `/v1/compress` endpoint with a 5s timeout; **fails open** on error.
- `caveman` is **DEPRECATED** (owner decision, 2026-09-16). The key stays accepted so a configuration
  exported from the reference still round-trips, and it is frozen at the default shown above. No new
  client code must set it, `app-ui` never renders a control for it (its spec is
  [`docs/SPEC-UI/001-SPEC-UI.md`](../SPEC-UI/001-SPEC-UI.md) §6.7 and §13.6), and it is removed in the
  next breaking version (`/api/v2`, §4).
- `X-Token-Saver: off` per-request bypass (§4).

### 7.10 Media Providers

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/media-providers` | S | Media-capable providers by kind: `tts|stt|embedding|image|video|search`; `?kind=` | P2 |
| GET | `/api/v1/media-providers/{provider_id}` | S | Detail incl. models + per-endpoint `base_url` (self-hosted support) | P2 |
| PATCH | `/api/v1/media-providers/{provider_id}` | S | Set default model / per-endpoint `base_url` override | P2 |

Data-plane media endpoints (OpenAI-compatible shapes, gateway-key auth, format per reference handlers):

| Method | Path | Phase |
|---|---|---|
| POST | `/api/v1/audio/speech` | P2 |
| POST | `/api/v1/audio/transcriptions` | P2 |
| GET | `/api/v1/audio/voices` | P2 |
| POST | `/api/v1/embeddings` | P1 |
| POST | `/api/v1/images/generations` | P2 |
| POST | `/api/v1/videos/generations` | P2 |
| POST | `/api/v1/search` | P2 |

Self-hosted media providers read `base_url` from their upstream endpoint (exact `/v1`-inclusion rule
from the reference README is preserved in adapter docs); no silent cloud fallback — missing `base_url`
⇒ `VALIDATION_ERROR`.

`POST /audio/transcriptions` keeps the OpenAI-compatible multipart passthrough for OpenAI-shaped providers.
The first provider-specific adapter is Deepgram STT: the gateway sends the uploaded audio bytes with a
sanitized audio MIME type, `model`, `smart_format=true`, `punctuate=true`, and either `language` or
`detect_language=true`, authenticates as `Authorization: Token`, and normalizes
`results.channels[0].alternatives[0].transcript` to `{text}`. A provider format without an adapter remains
an explicit `PROVIDER_NOT_ROUTABLE` refusal; this is the incremental G5 boundary, not a claim that all
reference adapters are present.

### 7.11 Proxy Pools

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/proxies` | S | List | P2 |
| POST | `/api/v1/proxies` | S | `{label, protocol, host, port, username?, password?}` | P2 |
| PATCH | `/api/v1/proxies/{id}` | S | Update fields (`password` write-only) | P2 |
| DELETE | `/api/v1/proxies/{id}` | S | Remove | P2 |
| POST | `/api/v1/proxies/{id}/test` | S | Connectivity test → `{state, latency_ms, checked_at}` | P2 |
| POST | `/api/v1/proxies/test` | S | Test an unsaved `{protocol, host, port, ...}` candidate | P2 |

Proxy assignment to upstream endpoints rides on `settings.network.outbound_proxy_*` (global) in v1;
per-endpoint proxy binding is deferred (not in the reference either — reference assigns pools globally
with per-provider strategy overrides in `settings.providerStrategies`).

### 7.12 Usage & Quota Tracker

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/usage/summary` | S | Totals + group-by; `?from=&to=&group_by=provider|model|endpoint|gateway_key` | P1 |
| GET | `/api/v1/usage/timeseries` | S | Buckets; `?granularity=hour|day&from=&to=` | P1 |
| GET | `/api/v1/usage/records` | S | Paginated raw records; filters as summary | P1 |
| GET | `/api/v1/usage/records/{request_id}` | S | Single request detail (joins request log if captured) | P1 |
| GET | `/api/v1/quotas` | S | All quota windows: `{endpoint_id, provider_id, window, used, limit, resets_at, source}`; `source` ∈ `computed|reported` | P1 |
| GET | `/api/v1/quotas/{endpoint_id}` | S | Windows for one endpoint | P1 |
| PUT | `/api/v1/quotas/{endpoint_id}` | S | Set budget caps `{monthly_cost_usd?, monthly_tokens?}` — router stops picking exhausted endpoints | P2 |

Quota worker re-checks windows per provider cadence (5h/daily/weekly/monthly), refresh countdowns,
and records `resets_at`. Cost figures are estimates for display only (reference parity).

### 7.13 Logs

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/logs/requests` | S | Paginated; `?from=&to=&status=&endpoint_id=&model=&q=` | P1 |
| GET | `/api/v1/logs/requests/{request_id}` | S | Full captured req/resp (if capture enabled) | P1 |
| DELETE | `/api/v1/logs/requests` | S | Purge (respects retention config) | P1 |
| GET | `/api/v1/logs/console` | S | In-memory ring buffer (last N lines, mirrors `consoleLogBuffer`) | P1 |
| DELETE | `/api/v1/logs/console` | S | Clear buffer | P1 |

Body capture governed by `settings.logging` (§7.14): off by default; when on, truncated to
`capture_body_max_bytes`.

### 7.14 Settings

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/settings` | S | Typed settings object (secrets never returned) | P1 |
| PATCH | `/api/v1/settings` | S | Partial update, validated per key | P1 |

v1 settings surface (subset of reference `DEFAULT_SETTINGS`):

```json
{
  "security": { "require_login": true, "require_api_key": true },
  "routing":  { "combo_strategy": "fallback", "combo_sticky_limit": 1, "sticky_limit": 3 },
  "network":  { "outbound_proxy_enabled": false, "outbound_proxy_url": "", "outbound_no_proxy": "" },
  "token_saver": { "see §7.9" },
  "logging":  { "request_capture_enabled": false, "retention_days": 7, "capture_body_max_bytes": 65536, "observability_max_records": 1000 }
}
```

### 7.15 Data Plane (OpenAI/Anthropic/Gemini-compatible, gateway-key auth)

Model string resolution order (mirrors reference `getComboModels` + aliases):
`combo name → alias → provider/model → 404 MODEL_NOT_FOUND`.

| Method | Path | Wire format | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/models` | OpenAI | `{object:"list", data:[...]}` — routable models + combos | P1 |
| POST | `/api/v1/chat/completions` | OpenAI | Chat, stream + non-stream | P1 |
| POST | `/api/v1/responses` | OpenAI Responses | Responses API | P3 |
| POST | `/api/v1/messages` | Anthropic | Claude messages, stream + non-stream | P1 |
| POST | `/api/v1/messages/count_tokens` | Anthropic | Token counting | P3 |
| POST | `/api/v1/embeddings` | OpenAI | Embeddings | P1 |
| POST | `/api/v1/audio/*`, `/images/*`, `/videos/*`, `/search` | OpenAI-ish | See §7.10 | P2 |

Request pipeline (port of `sse/handlers/chat.js` + `open-sse`):

```
auth → schema validation → bypass detection (naming/warmup) → model resolve
  → required-capability detection → combo/adapter augmentation (§7.7, §7.8)
  → token savers (§7.9) → format translation (OpenAI↔Claude↔Gemini)
  → endpoint+key selection (priority, circuit state, quota) → upstream call
  (timeout/retry/proxy) → response translation → usage + quota + log recording
  → SSE passthrough / JSON response
```

## 8. Error Codes (management envelope)

| Code | HTTP | Meaning |
|---|---|---|
| `VALIDATION_ERROR` | 400 | Payload failed schema validation |
| `UNAUTHORIZED` | 401 | Missing/invalid session or gateway key |
| `FORBIDDEN` | 403 | Authenticated but not permitted |
| `NOT_FOUND` | 404 | Unknown resource (incl. unknown `provider_id`) |
| `METHOD_NOT_ALLOWED` | 405 | Verb is not registered for that path (router-level rejection) |
| `CONFLICT` | 409 | Uniqueness / referential conflict (e.g. deleting last key) |
| `RATE_LIMITED` | 429 | Gateway or login rate limit hit |
| `NO_PROVIDER_AVAILABLE` | 503 | Data plane: all endpoints/keys for the model are unhealthy or quota-exhausted |
| `UPSTREAM_ERROR` | 502 | Upstream returned unrecoverable error |
| `UPSTREAM_TIMEOUT` | 504 | Upstream timeout after retries |
| `INTERNAL_ERROR` | 500 | Unexpected; logged with `request_id` |

`PROVIDER_NOT_ROUTABLE` is a **data-plane** code, so it is returned in the OpenAI error envelope
(§4), not the management one: the caller is a CLI tool, not the panel. It is returned when a request
names a provider whose wire format the gateway does not translate and for which no connector is
registered. It exists so that case is a named, actionable error rather than a 502 that reads like an
upstream outage. Which providers are in that set is published by `GET /api/v1/providers` as a
`routability` field (§7.4), computed from the registry entry's format, so the panel can warn before a
user configures an endpoint that can never answer.
### 8.1 Wire shapes settled before the endpoint DTOs

These decisions were taken before the schema layer was written, because each one changes the shape a
client sends and rewriting it later would mean rewriting the DTOs and every caller.

| Question | Decision | Reason |
|---|---|---|
| How is a credential's auth type expressed? | The API accepts `api_key`, `oauth`, `no_auth`. The registry's own vocabulary (`apikey`, `none`, `cookie`) is **mapped at the boundary**, not leaked: `apikey`→`api_key`, `none`→`no_auth`, and `cookie` is rejected with `VALIDATION_ERROR` because a browser-session credential is not a supported auth type in v1 (§2.2 leaves its tooling out). | Two vocabularies exist for real reasons, but only one may cross the wire; leaking both would make every client branch. |
| Does bulk change the single-create shape? | No. `POST /endpoints` takes `{provider_id, label, auth_type, priority?, keys:[…]}`; `POST /endpoints/bulk` takes the same fields per element of an `endpoints` array. The single route is the array-of-one case, so one validator serves both. | Duplicate shapes drift; one shape with two entry points does not. |
| Bulk OAuth credential import shape | `POST /providers/{provider_id}/oauth/bulk` takes `{accounts:[{access_token, refresh_token?, expires_at?, scopes?, label?, account?}]}`. `access_token` and `refresh_token` are **required to be non-empty** when present, are sealed immediately, and are never returned; the response carries the endpoint id, label, and account identity only. `account` is `{name?, email?, machine_id?, workspace_id?}` and, with the provider's auth modes, decides whether a re-import updates an existing account or adds one. | The tokens are the only fields whose secrecy matters, so the response shape is defined by what it must *not* carry. |
| Are partial bulk failures allowed? | No: a batch is validated in full and applied in one transaction. The response reports every row by index (`{index, id?, error?}`) so a client can show which row was rejected, but nothing is written unless all rows pass. | A half-imported account list is harder to reason about than a refused batch. |
| Can a media service use the provider's chat credential placement? | No: each media kind carries its own `auth_type`/`auth_header`, and `auth_header: key` means a **query parameter**, not a header. | Measured in the reference: Gemini's embedding config uses a query-param key while its chat transport uses a header. Reusing the transport's placement authenticates incorrectly rather than failing loudly. |

## 9. Non-Functional Requirements (from AGENTS.md, binding)

1. Files ≤ 250 lines (generated exempt); headers per §1.2; `gofmt`, `golangci-lint`, `go vet`,
   `go test -race`, `gitleaks` — all wired via `scrypts/` hooks/gates (pre-commit, pre-push).
2. Layer flow `schema → domain → repository → service → handler → router`; no `net/http` in service.
3. No `any`/`interface{}` outside declared decode boundaries; env → typed validated `Config` at boot.
4. Every goroutine recovers panic; workers state retry policy + dead-letter behavior in code docs.
5. No unbounded queries; `usage_records`/`request_logs` always paginated; indexes per §6.
6. Pool limits set explicitly; Redis required for limiter/sticky/circuit state.
7. Cross-service imports forbidden; shared code goes to a shared module, never vendored.
8. `SYSTEM_MAP.md` updated in the same PR when topology/data flow changes; PR description states
   whether docs were reviewed.
9. Every outbound dial goes through the process-wide egress guard (`internal/netguard`,
   `EGRESS_ALLOWED_TARGETS`): the chat/embeddings/media data plane, the connectivity probe, the
   OAuth token client, and the proxy test share one guard and one guarded HTTP client. Loopback
   and private ranges are refused unless the operator names them in the allowlist — a self-hosted
   upstream (§7.10) needs its address listed — and a malformed entry fails the boot.

## 10. Delivery Phases

| Phase | Content | Exit criteria |
|---|---|---|
| **P0** | Go skeleton, config, migrations, health/version, auth, gateway keys, `scrypts` gates | login + key CRUD tested (happy/validation/auth per route) |
| **P1** | Provider registry port (YAML), upstream endpoints + multi-key, models catalog, combos (fallback/round_robin), vision adapter, chat data plane (OpenAI + Anthropic), embeddings, usage + quotas, logs, settings | CLI tool completes a request through combo fallback; `-race` clean |
| **P2** | OAuth providers + refresh worker, fusion strategy, combo test, media providers + media data plane, proxies, token-saver config, budget caps, custom/alias/disabled models | OAuth provider round-trip; TTS/embeddings passthrough verified |
| **P3** | Responses API, count_tokens, native token-saver engine (new spec: `002-TOKEN-SAVER`) | parity spot-checks vs reference |

*Changelog 2026-09-17 — §7.4 publishes `routability` (`native` / `connector`) and the per-kind `media` block; §8.1 records the five wire shapes settled before the endpoint DTOs (auth-type mapping, one create shape for single and bulk, the OAuth bulk import shape, all-or-nothing batch semantics, and per-kind credential placement). Rationale: four providers on the owner's list speak protocols P1 does not translate, so "configured but always failing" needed to become a value the panel reads, and the bulk routes needed a shape before their DTOs existed.*

## 11. Locked Decisions

1. Single version prefix `/api/v1`; data plane lives under it (clients set `base_url = http://host:8080/api/v1`).
2. Multi-key-per-endpoint is a **new** capability not present in the reference (explicit user requirement).
3. PostgreSQL + Redis replace SQLite; provider registry is embedded static config, not DB rows.
4. Token saver = config-only in v1; native engine deferred to its own spec.
5. `app-ui` never touches PostgreSQL/Redis; management API is its only surface.

## 12. Open Questions (resolve before P1 code)

1. Default listen port for `app-serv` (reference used 20128) — propose **8080**.
2. Session token format: opaque random nonce + HMAC-SHA256 signature in `pannel_session`; Redis stores the nonce digest with TTL for revocation. JWT is not used because immediate revocation is required.
3. Registry port scope: all 120+ providers in P1, or apikey-category first (~40) and OAuth set in P2?

---

*Changelog: 2026-09-11 — initial draft (feature mapping from reference `~/ai-gateway` @ 9Router 0.5.55); same day — added 002 OpenAPI companion cross-link + OAuth callback headless mode.*
*Changelog 2026-09-17 — P0 auth contract implemented: `/api/v1` session routes, HMAC-signed `pannel_session`, Redis revocation/lockout, and session-gated gateway-key CRUD.*
*Changelog 2026-09-16 — marked the `caveman` token-saver key DEPRECATED with removal scheduled for `/api/v2`, to match the owner decision; linked the panel contract at `docs/SPEC-UI/001-SPEC-UI.md`.*
*Changelog 2026-09-16 — §8 adds `METHOD_NOT_ALLOWED` (405): the router registers routes method-aware, so a wrong verb is rejected before any handler runs and needs a code that maps to 405 rather than borrowing `VALIDATION_ERROR` (which §8 binds to 400). §6 records that `gateway_keys.name` is unique.*
*Changelog 2026-09-17 — §7.4 and §7.5 add the two routes the owner requirements name and the spec lacked: custom provider nodes (OpenAI-compatible / Anthropic-compatible) in §7.4, and multi-account bulk onboarding (endpoint batch, key batch, OAuth credential import) in §7.5. Both are P1, because the reference ships the node feature at the same surface level and every provider is multi-account by requirement.*
*Changelog 2026-09-19 — §7.4 records the callback's two answers and the redirect-origin rule, because the panel (SPEC-UI §6.3) has to read the outcome the browser lands with. The callback is public by necessity and its origin never comes from the request: a configured `PUBLIC_BASE_URL` wins, the staged `redirect_uri` origin is only a fallback, and with neither the route answers JSON instead of redirecting.*
*Changelog 2026-09-19 — §7.7 records the fusion execution the reference fixes, now that the data plane runs it: a non-streamed tool-less panel, the judge receiving the client's request plus the directive, the 1-answer and 0-answer degradations, and that panel spend stays outside the request's usage row. Also: a combo whose leading reference no longer resolves now starts from the first reference that does, instead of failing the whole combo — the engine already skipped such members.*
*Changelog 2026-09-19 — §7.7 records round_robin execution: the engine asks `ComboService.Order` for the order through the `dataplane.ComboOrderer` seam, so the distribution rule stays in one place and the combo's own `sticky_limit` is what reaches the store. An order that cannot be produced falls back to priority order, because rotation is an optimisation, not a correctness input. Before this, `ComboService.Order` had no production caller and the engine walked the stored order for every strategy.*
*Changelog 2026-09-19 — §7.7's combo test route now fixes its answer: `{combo_id, combo, strategy, results:[{ref, role, ok, provider_id, model_id, endpoint_id, latency_ms, error_code, error}]}`, one entry per stored reference with a fusion combo's judge last (`role: "judge"`). Each probe is a one-token non-streaming chat sent through the normal pipeline, so an alias or a nested combo resolves the way a real request would, and a failed probe is a `results` entry rather than a route error — refusing the whole answer because one member is down would hide the members that are up. Only an unreadable combo is 404. Probes run sequentially, so one click spends one account at a time and the reported order is the stored one. Probe spend is not written to `usage_records`: the route carries no gateway key or request id, and its spend is an operator diagnostic rather than client traffic.*
*Changelog 2026-09-19 — §7.11 records the proxy pool's shapes and rules, now that the surface is implemented: the password is write-only (`has_password` in the response; an empty value on PATCH keeps the stored secret), and both test routes answer `{state: "ok"|"fail", latency_ms, checked_at}` plus an additive `message`. A failed probe is a 200 carrying that fail state rather than an error, because "does this candidate work" was the question. `POST /proxies/{id}/test` stores its finding so the list reports the same proof; `POST /proxies/test` stores nothing. A repoint or a credential change clears the stored status — a proof measured against the old address is not a proof of the new one — while saving an unchanged form does not. Every test runs under the new egress guard (`internal/netguard`, OWASP A01): the candidate's host is resolved and every answer validated before any dial and re-validated at connect time, loopback and private ranges are refused unless the operator names them in `EGRESS_ALLOWED_TARGETS`, and the fetched URL is the server's `PROXY_TEST_URL`, never a request field.*
*Changelog 2026-09-19 — §7.10 records the media surface now that both planes are implemented. Management: overrides are stored per provider **and** kind (`media_provider_settings`, PK `(provider_id, kind)`), and every resolved value reports its own source (`registry`|`override`) rather than one flag for the pair, because a provider may have its `base_url` overridden while its default model still comes from the registry. A PATCH that would leave no `base_url` from either source is refused (`VALIDATION_ERROR`) — the "no silent cloud fallback" rule above, enforced at save time rather than discovered as a dial to the wrong host. Data plane: the six routes keep the reference's wire shapes — `POST /audio/speech` answers bytes as `audio/{fmt}` unless `?response_format=json` asks for `{audio: base64, format}`; `POST /audio/transcriptions` forwards the upstream's answer verbatim (JSON as `application/json`, anything else as `text/plain`) because re-encoding would drop the fields the caller asked for with `response_format`, and it forwards `language`/`prompt`/`response_format`/`temperature` only to OpenAI-compatible providers, which is where the reference forwards them; `GET /audio/voices` serves the catalog the block declares in the registry (`voices:`), and a provider declaring none is refused by name; `POST /images/generations` normalizes to `{created, data:[{url|b64_json}]}`; `POST /videos/generations` is registered but refuses `PROVIDER_NOT_ROUTABLE`, because no provider declares `video` — reference parity, where `videoConfig` is recognized and nothing defines it; `POST /search` builds its payload from the block's declared `query_param`/`max_results_param` (GET reads them as query parameters, POST as a body), because search APIs disagree on `q`/`query` and `num`/`count`. Model strings are `provider/model`, split on the first slash so slash-carrying ids (`openrouter`'s `openai/gpt-4o-mini-tts`) resolve to their provider and keep the whole model; the reference's `provider/model/voice` shorthand is deliberately not carried, because it cannot be told apart from such an id. A provider declaring a media format the gateway has no adapter for is refused by name (`PROVIDER_NOT_ROUTABLE`) instead of being dialed with a wrong-shaped payload — an honest gap rather than an outage-shaped one. Media routes authenticate the gateway key before reading any body: a 25 MB multipart upload is never parsed, or spilled to disk, for a caller without a key.*
*Changelog 2026-09-19 — §9 makes the egress policy binding for every upstream dial, not only the proxy test. The chat, embeddings, and media transports, the connectivity probe, and the OAuth token client now share the process's `internal/netguard` guard, its `EGRESS_ALLOWED_TARGETS` allowlist, and the one HTTP client built on its dialer; a probe of a refused address reports the refusal (`the upstream address was refused: …`) instead of an unreachable host. A self-hosted upstream on loopback or a private range therefore needs its address allowlisted, and a malformed entry fails the boot rather than widening what the gateway may reach.*
*Changelog 2026-09-19 — §7.12/§7.13 record the media plane's accounting now that it is wired (register G6, D3 = usage + log + request_count). Every §7.10 media call and every `POST /embeddings` writes exactly one `usage_records` row and one `request_logs` row, both under the router's request id so the pair is reachable from either surface; a call that fails is recorded too (`status: error`, `error_code` set), because the row is what makes a failure visible in the panel rather than only in the gateway's own log. Tokens are 0 — a media answer reports no token usage — a search call's `cost_usd` is the registry's `cost_per_query` and its `model` column carries the provider id because the route names no model, and the log row's error text is `CODE: message` without the wrapped cause: a media URL may carry the credential as a query parameter (§8.1), so an upstream's message is quoted but our own error chain is not. Log bodies stay empty for media, because the reference records request detail for chat only. A recording failure never fails a call the client already got an answer to, the same rule the chat plane follows. §7.3's `request_count` and `last_used_at` now advance once per authenticated data-plane call — chat, models list, media, embeddings — written at the single authentication choke point, so one call counts exactly once, and the counter advances in the database (`request_count = request_count + 1`) rather than by read-modify-write, so concurrent calls do not lose counts.*
*Changelog 2026-09-19 — §7.12 records the budget cap's read-back (owner decision D5 = b, register G12): `GET /api/v1/quotas/{endpoint_id}` now answers `{endpoint_id, cap, data}`, where `cap` is the stored cap or `null` when none is set, so a client that just wrote a cap reads the same values back before any usage window exists — the case that previously answered `{"data":[]}`. The field is an explicit `null` rather than an omitted key, because "no cap" is a state the panel renders as an empty form and an absent field could not be told apart from a response that forgot it. The collection route `GET /api/v1/quotas` stays windows-only, since a cap belongs to one endpoint, and its body is unchanged.*
*Changelog 2026-09-19 — §9 records what a failed request leaves in the gateway's own log (register G9): the existing `request handled` line gains a `code` field carrying the machine error code beside the request id and status — the same envelope code §8 already binds to the response, so the operator reads "why" from the one line that already says "what". The field is absent on a served request, so its presence means the request failed; 404 and 405 from the router's own envelope carry their code too. Only the code is logged, never the error message: a data-plane failure's message can quote an upstream's text back, and that text can carry the credential (§8.1). The panic path keeps its id + panic value, which is operator-only text and never leaves the process.*

*Changelog 2026-09-19 — §7.12/§7.13 record the chat plane's accounting now that it is wired end to end (register G17/G18). Every chat call — `POST /chat/completions` and `POST /messages`, streamed or not — leaves one `request_logs` row under the router's request id, carrying the client's request body and the served answer for the capture rule to decide about, so capture stays one decision in `LogService.Record` and a call refused before the pipeline ran (an unknown model, an invalid body) is visible in the Logs screen too. A call that reached an upstream attempt also leaves one `usage_records` row, on success and failure alike: the engine now builds the call's identity before anything can fail, so a failed call's row carries the provider, endpoint, model, and combo it was attempted against with `status: error` and the error code — before this, every failure path returned a zero identity, the aggregate rejected the row, and the rejection was swallowed. A call refused before any attempt leaves no usage row: the aggregate requires a provider and a model, and the write is skipped deliberately rather than left to fail validation. Fusion combos follow the same rule — a failed panel reports the first member's identity, the failure the client receives, and a failed judge reports the judge's. The chat log row's error text is the code alone, never the `CODE: message` the media row stores: a chat upstream's rejection quotes the presented credential back in its message, and a stored row must not hold a secret.*
*Changelog 2026-09-19 — §7.12/§7.13 record the refusal row now that the media and embeddings planes write it (register G20). A call refused before any upstream attempt — an unknown provider, an untranslated media format, a provider with no base URL, no usable account, an unresolvable model, a combo on the embeddings route — leaves one `request_logs` row under the router's request id, with `status: error` and the error's code alone; it leaves no `usage_records` row, because nothing was spent and the aggregate requires a provider and a model, which is the same deliberate skip the chat plane makes for a request refused before the pipeline ran. The row names the provider and model resolution had reached: a media model string is split so `nope/model` names both columns, a search call that named only a provider is named by it, and a request whose provider never resolved carries no identity rather than a guessed one. A refusal the handler itself answers (an invalid body, a bad key) writes nothing, the same boundary the chat plane keeps.*
*Changelog 2026-09-19 — §7.10 records the first incremental G5 adapter: Deepgram STT now accepts the raw uploaded audio bytes, derives a safe audio MIME type from the multipart part or sanitized filename, sends `model`, `smart_format`, `punctuate`, and `language`/`detect_language` query parameters, uses `Authorization: Token`, and normalizes Deepgram's nested transcript to the OpenAI-compatible `{text}` answer. Provider aliases resolve before the adapter is selected (`dg/nova-2` and `deepgram/nova-2` are equivalent). The remaining non-OpenAI media formats stay explicit `PROVIDER_NOT_ROUTABLE` refusals until their own adapters are ported; AWS Polly is not invented because the reference has no builder for it.*
