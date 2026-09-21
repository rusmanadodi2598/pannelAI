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
| `upstream_keys` | `id, endpoint_id→upstream_endpoints, label, value_encrypted, key_hint, priority, status, last_used_at, last_error, consecutive_errors, rate_limited_until, created_at, updated_at` | **N per endpoint**; value encrypted at rest (AES-GCM, key from env) |
| `gateway_keys` | `id, name, value_hash, key_hint, status, last_used_at, request_count, created_at, revoked_at` | SHA-256 hash lookup, plaintext shown once |
| `combos` | `id, name, strategy, sticky_limit, judge_model, models(jsonb), created_at, updated_at` | `models`: `[{ref, priority}]` |
| `model_aliases` | `alias, target, created_at` | alias → `provider/model` or combo name |
| `models_custom` | `id, provider_id, model_id, display_name, capabilities(jsonb), created_at` | user-added models |
| `models_disabled` | `provider_id, model_id, disabled_at` | hidden from catalog/routing |
| `usage_records` | `id, request_id, ts, endpoint_id, provider_id, gateway_key_id, model, combo, tokens_in, tokens_out, tokens_cache_read, tokens_cache_write, cost_usd, latency_ms, status, error_code` | partitioned by month; index `(ts)`, `(provider_id, ts)`, `(endpoint_id, ts)` |
| `quota_windows` | `endpoint_id, window, used_units, limit_units, resets_at, source` | window ∈ `5h|daily|weekly|monthly`; counters cached in Redis, flushed to PG |
| `request_logs` | `request_id, ts, gateway_key_id, endpoint_id, model, status, latency_ms, request_body, response_body, error` | bodies stored only when capture enabled; retention job deletes > `retention_days` |
| `proxies` | `id, label, protocol, host, port, username, password_encrypted, enabled, status(jsonb), created_at, updated_at` | protocol ∈ `http|https|socks5` |
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
| PATCH | `/api/v1/gateway-keys/{id}` | S | `{name?, status?}` | P0 |
| DELETE | `/api/v1/gateway-keys/{id}` | S | Revoke (soft) | P0 |

### 7.4 Providers (registry + OAuth)

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/providers` | S | Registry list: `{id, name, category, auth_type, endpoint_count, status_summary}`; filter `?category=apikey|oauth|free|media|local` | P1 |
| GET | `/api/v1/providers/{provider_id}` | S | Registry detail incl. transport defaults + model catalog summary | P1 |
| GET | `/api/v1/providers/{provider_id}/models` | S | Full model list; `?suggested=true` returns suggested set | P1 |
| POST | `/api/v1/providers/{provider_id}/oauth/start` | S | `{redirect_uri?}` → `{authorize_url, state}` (OAuth providers only) | P2 |
| GET | `/api/v1/providers/{provider_id}/oauth/callback` | P | Completes flow (validates `state` replay-guard), creates upstream endpoint + tokens; `302` redirect for browsers, `200` JSON body for headless callers (`Accept: application/json`) | P2 |
| GET | `/api/v1/providers/{provider_id}/oauth/status` | S | Token expiry / refresh state per endpoint | P2 |
| POST | `/api/v1/providers/{provider_id}/oauth/refresh` | S | Force token refresh (worker also auto-refreshes at `refresh_lead`) | P2 |

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
| `CONFLICT` | 409 | Uniqueness / referential conflict (e.g. deleting last key) |
| `RATE_LIMITED` | 429 | Gateway or login rate limit hit |
| `NO_PROVIDER_AVAILABLE` | 503 | Data plane: all endpoints/keys for the model are unhealthy or quota-exhausted |
| `UPSTREAM_ERROR` | 502 | Upstream returned unrecoverable error |
| `UPSTREAM_TIMEOUT` | 504 | Upstream timeout after retries |
| `INTERNAL_ERROR` | 500 | Unexpected; logged with `request_id` |

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

## 10. Delivery Phases

| Phase | Content | Exit criteria |
|---|---|---|
| **P0** | Go skeleton, config, migrations, health/version, auth, gateway keys, `scrypts` gates | login + key CRUD tested (happy/validation/auth per route) |
| **P1** | Provider registry port (YAML), upstream endpoints + multi-key, models catalog, combos (fallback/round_robin), vision adapter, chat data plane (OpenAI + Anthropic), embeddings, usage + quotas, logs, settings | CLI tool completes a request through combo fallback; `-race` clean |
| **P2** | OAuth providers + refresh worker, fusion strategy, combo test, media providers + media data plane, proxies, token-saver config, budget caps, custom/alias/disabled models | OAuth provider round-trip; TTS/embeddings passthrough verified |
| **P3** | Responses API, count_tokens, native token-saver engine (new spec: `002-TOKEN-SAVER`) | parity spot-checks vs reference |

## 11. Locked Decisions

1. Single version prefix `/api/v1`; data plane lives under it (clients set `base_url = http://host:8080/api/v1`).
2. Multi-key-per-endpoint is a **new** capability not present in the reference (explicit user requirement).
3. PostgreSQL + Redis replace SQLite; provider registry is embedded static config, not DB rows.
4. Token saver = config-only in v1; native engine deferred to its own spec.
5. `app-ui` never touches PostgreSQL/Redis; management API is its only surface.

## 12. Open Questions (resolve before P1 code)

1. Default listen port for `app-serv` (reference used 20128) — propose **8080**.
2. Session token format: opaque random + Redis (proposed) vs JWT — opaque preferred for revocation.
3. Registry port scope: all 120+ providers in P1, or apikey-category first (~40) and OAuth set in P2?

---

*Changelog: 2026-09-11 — initial draft (feature mapping from reference `~/ai-gateway` @ 9Router 0.5.55); same day — added 002 OpenAPI companion cross-link + OAuth callback headless mode. 2026-09-16: marked the `caveman` token-saver key DEPRECATED with removal scheduled for `/api/v2`, to match the owner decision; linked the panel contract at `docs/SPEC-UI/001-SPEC-UI.md`.*
