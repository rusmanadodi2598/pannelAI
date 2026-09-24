# SPEC-API-001: pannelAI Gateway API v1

| | |
|---|---|
| **Spec ID** | 001-SPEC-API |
| **Status** | Closed |
| **Date** | 2026-09-11 |
| **Organization** | KENTANG TECH |
| **Author** | Dodi Rusmana <rusmanadodi@kentangtech.com> |
| **GitHub** | https://github.com/rusmanadodi2598 |
| **Source Rewrite (Reference)** | `/home/rusmanadodi/ai-gateway` (9Router, Next.js monolith) |
| **Target** | `pannelAI/` → `app-serv/` (Go API) + `app-ui/` (Svelte panel) |
| **Governance** | `AGENTS.md` (root): mandatory for all implementation work |
| **Companion spec** | [`docs/SPEC-UI/001-SPEC-UI.md`](../SPEC-UI/001-SPEC-UI.md) (panel contract; wire shapes remain normative here) |
| **OpenAPI source** | [`docs/CONTRACT/001-CONTRACT-API-V1.yaml`](../CONTRACT/001-CONTRACT-API-V1.yaml) (schema-first source); [`docs/CONTRACT/001-CONTRACT-API-V1.md`](../CONTRACT/001-CONTRACT-API-V1.md) (manual endpoint reference) |

> **SPEC FIRST.** This document is the contract of record for API v1. Implementation starts only against
> what is written here. Any deviation requires editing this spec in the same PR. The machine-readable
> OpenAPI 3.1 source: [`../CONTRACT/001-CONTRACT-API-V1.yaml`](../CONTRACT/001-CONTRACT-API-V1.yaml), with the manual endpoint reference at [`../CONTRACT/001-CONTRACT-API-V1.md`](../CONTRACT/001-CONTRACT-API-V1.md). The generated JSON is served at `/api/v1/openapi.json`.

---

## 1. Purpose

Rewrite the 9Router AI gateway as a clean two-app system:

- **`app-serv`**: Go 1.26 API server. Management plane + OpenAI-compatible data plane. PostgreSQL + Redis.
- **`app-ui`**: Svelte panel (Bun toolchain). Consumes `app-serv` management APIs only. No direct DB access.

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

### 2.2 NOT PORTED (left out on purpose: do not spec, do not build)

Cloud sync, tunnels (Cloudflare/Tailscale), MITM router, pxpipe, CLI-tools config writers
(`src/app/api/cli-tools/*`), the translator playground page, headroom process manager
(start/stop/restart; v1 only calls an external Headroom URL), proxy-pool cloud deployers
(Cloudflare/Deno/Vercel), SAML/OIDC/SSO, MCP, shutdown/update endpoints, i18n framework.
Anything not listed in §2.1 is assumed **not wanted**; it may be added later via a new spec only.

The 2026-09-20 endpoint audit ([`../DRAFT/003-ENDPOINT-READINESS.md`](../DRAFT/003-ENDPOINT-READINESS.md))
pulled four items back into scope by owner decision: **skills**, the **chat playground**, **API
docs**, and a **changelog surface**. They are not reference ports; §7.16 to §7.18 specify them from
the reference's surface behavior, and phase P4 builds them. The translator playground stays excluded.

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
| Management error envelope | `{"error":{"code":"NOT_FOUND","message":"..."}}`: `code` machine-parseable, `message` English (AGENTS §1.3) |
| Data plane error envelope | OpenAI shape for client compatibility: `{"error":{"message":"...","type":"invalid_request_error","code":"..."}}` |
| Pagination | `?page=1&per_page=25` (max 100) on list endpoints → `{"data":[...],"meta":{"page":1,"per_page":25,"total":123}}` |
| Streaming | SSE passthrough (`text/event-stream`), heartbeat disabled by default, `X-Accel-Buffering: no`; usage chunk emitted when `stream_options.include_usage=true`. The status line and SSE headers are committed on the first frame, so a relay failure with no frame yet answers an ordinary HTTP error (the §4 envelope and its status) rather than a success status carrying an error body. After the first frame the status is fixed and a failure ends the stream without `[DONE]`, because that marker is the client's only signal that the answer is complete (register F3 of draft 009) |
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
| **Upstream Key** | One API key credential under an upstream endpoint (multi-key per endpoint: new capability) | `connection.apiKey` (was 1:1) |
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

Every table in the schema belongs to the role that owns `gateway_keys`: the application role the gateway
connects as. A migration runs as whichever role boots the process, so a superuser boot would otherwise
create tables the application role cannot read (measured live: `media_provider_settings` and `proxies`
answered `permission denied` while every other table worked). Migration `000011` re-owns those two to the
anchor role, and a refusal it cannot carry out is a warning at boot naming the statement to run rather
than a boot failure.

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
| POST | `/api/v1/gateway-keys` | S | `{name}` → returns **full key once**, in `plaintext_key` (absent on every other response, which carries `key_hint` only) | P0 |
| GET | `/api/v1/gateway-keys/{id}` | S | Detail (hint only) | P0 |
| PATCH | `/api/v1/gateway-keys/{id}` | S | `{name?, status?}`; `status` accepts only `active` or `disabled` (`revoked` is reached only through DELETE) | P0 |
| DELETE | `/api/v1/gateway-keys/{id}` | S | Revoke (soft) | P0 |

**Design decisions (owner, 2026-09-24).** Three reference behaviours are deliberately not ported,
recorded here so their absence reads as a decision rather than a gap
(`docs/PORT/001-PORT-ENDPOINT-KEYS.md` F3/F4):

- **Keys are opaque and machine-independent.** The reference key embeds a `machineId` and a CRC over it;
  ours is `GATEWAY_KEY_PREFIX` plus 48 random characters, and it validates at any gateway sharing the
  database. Binding a key to the host that minted it would break the multi-host deployment this gateway
  targets.
- **No auto-provision of a first key.** The reference creates a `Default Key` itself when the list is
  empty; the panel shows an empty state and asks the operator to create one, because minting a credential
  without operator intent is the wrong default.
- **Remote exposure (Cloudflare Tunnel, Tailscale Funnel) is out of scope.** The reference's API Endpoint
  card, its security gate, and its seven `/api/tunnel/*` routes have no equivalent in `app-serv` or
  `app-ui`; the panel shows the read-only base address only. An operator who needs remote exposure runs
  the tunnel in front of the gateway.

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

An **upstream endpoint** holds **1..N API keys**. Routing walks the provider's endpoints in the order
that provider's **credential rotation policy** fixes (§7.14): `fill-first` (the default) walks
priority order and starts every request from the first usable endpoint, `round-robin` advances the
shared cursor and keeps one endpoint for `sticky_limit` consecutive requests. Inside the chosen
endpoint it picks a healthy key, least-recently-used first under `round-robin` and in priority order
under `fill-first`, skipping parked keys and breaking ties by priority then id. The policy is a
per-provider override over a global default (`routing.fallback_strategy` plus
`routing.provider_strategies`), and a policy that cannot be read degrades to `fill-first`, because
rotation is an optimisation and not a correctness input. Within one request a
failed credential is followed by the provider's next healthy credential before the next model is
tried (credential-first failover, §7.7). This generalizes the reference 1-connection-1-key model.

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/endpoints` | S | List; `?provider_id=&status=`; sorted by `priority` | P1 |
| POST | `/api/v1/endpoints` | S | `{provider_id, label, auth_type, priority?, keys: [{label?, value}]}` (≥1 key for `auth_type=api_key`) | P1 |
| GET | `/api/v1/endpoints/{id}` | S | Detail incl. keys summary + test status | P1 |
| PATCH | `/api/v1/endpoints/{id}` | S | `{label?, priority?, status?}`: priority change reorders siblings transactionally | P1 |
| DELETE | `/api/v1/endpoints/{id}` | S | Delete endpoint + its keys | P1 |
| POST | `/api/v1/endpoints/{id}/test` | S | Connectivity test; `{key_id?}` targets one key else first active; updates `test_status` | P1 |
| POST | `/api/v1/endpoints/{id}/keys` | S | Add key `{label?, value}` | P1 |
| GET | `/api/v1/endpoints/{id}/keys` | S | List keys (hints only) with health fields | P1 |
| PATCH | `/api/v1/endpoints/{id}/keys/{key_id}` | S | `{label?, value?, priority?, status?}`: value is write-only | P1 |
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

Key health model (parking per key, class-aware): the first upstream failure parks the key with a
window sized by its class: 401/402/403/404 ⇒ `rate_limited_until = now + 2m`; 429 ⇒ exponential `2s × 2^(n-1)` capped at 5m
(`n` = consecutive failures, reset by a successful call); 5xx/network/timeout ⇒ 30s; any other 4xx
records nothing because the request, not the credential, is the cause. `consecutive_errors` counts
every parked failure and a successful call resets all of it. The health lives on the key row; only
the rotation cursor is Redis-backed.

### 7.6 Models (catalog, custom, alias, disabled)

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/models/catalog` | S | Merged catalog across providers; `?provider_id=&capability=vision|tools&q=&active=true|false`; excludes disabled. `active=true` answers only rows whose provider holds ≥1 active upstream endpoint (the router's own candidate population — `selection.go` selects status `active` under the canonical id); `active=false` and absent narrow nothing; any other spelling is `VALIDATION_ERROR` (draft 025) | P1 |
| GET | `/api/v1/models/custom` | S | List custom models; `?provider_id=` | P2 |
| POST | `/api/v1/models/custom` | S | `{provider_id, model_id, display_name, capabilities?}` | P2 |
| DELETE | `/api/v1/models/custom/{id}` | S | Remove custom model | P2 |
| GET / PUT | `/api/v1/models/aliases` | S | `[{alias, target}]`; PUT replaces full set (validated: target exists) | P2 |
| GET / PUT | `/api/v1/models/disabled` | S | `{provider_id, model_id}` list; PUT replaces set | P2 |

### 7.7 Combos

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/combos` | S | List | P1 |
| POST | `/api/v1/combos` | S | `{name, strategy, models:[{ref, priority}], sticky_limit?, judge_model?}`: `ref` = `provider/model`, combo name, or alias (one deref level); strategy ∈ `fallback|round_robin|fusion`. A `provider/model` ref is accepted in every form the router resolves (provider id, registry alias, node prefix) and must name a model the chat plane serves — a provider with no chat translator or a media model is `VALIDATION_ERROR` naming the reason | P1 |
| GET | `/api/v1/combos/{id}` | S | Detail | P1 |
| PATCH | `/api/v1/combos/{id}` | S | Update any field | P1 |
| DELETE | `/api/v1/combos/{id}` | S | Delete (references in aliases must be cleaned first → `CONFLICT`) | P1 |
| POST | `/api/v1/combos/{id}/test` | S | Sends a 1-token ping through the combo chain, reports per-model result | P2 |

Strategy semantics (ported from `combo.js`):
- `fallback`: within one model the router first spends that provider's healthy credentials (next
  endpoint, then next key, in the order the provider's rotation policy fixes, §7.14) before moving to the next model in priority order. A request-scoped 4xx
  (any 4xx except 401/402/403/404/429) stops the chain and is returned to the client, because the
  same body would be rejected identically everywhere. When every model fails, the client receives
  the first failure's status with the last failure's message.
- `round_robin`: distribute; `sticky_limit` = consecutive requests kept on one model before rotating.
- `fusion`: fan out to N models, `judge_model` synthesizes the final answer.

The fusion execution the reference fixes, and the gateway implements:
- Panel calls are **forced non-streaming and carry no tools** (the judge needs complete prose, and a member
  that called a tool would return none). The panel answers are read back in the client's own wire format.
- The judge receives the **client's original request plus one appended user turn** carrying the directive,
  so the client's stream flag and tool declarations survive to the served call. Sources are anonymized
  (`[Source N]`) and the judge is told not to mention the panel.
- Degradation: **0 panel answers** is an upstream error (the panel's own failure is reported, so a
  validation failure stays a validation failure); **exactly 1 answer** is served directly, with no judge
  call, and a client that asked for SSE is re-issued the surviving member with its own request, because a
  streamed client must receive a stream rather than the panel's prose-only answer; a member that no longer
  resolves costs its own slot, not the request.
- A member's tokens are spent but are **not folded into the served request's usage row**, which records the
  judge's accounting: one row per request stays the accounting unit.

`round_robin` execution: the engine asks `ComboService.Order` through the `dataplane.ComboOrderer` seam,
for the order, which reaches the rotation store with the **combo's own `sticky_limit`** (not the data plane
default); it serves the returned leader and fails over through the rest of the rotated order. Rotation is an
optimisation: an order that cannot be produced, or one of a different length, serves the stored priority
order instead of failing the request.

### 7.8 Vision Adapter

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/vision-adapter` | S | `{enabled, round_robin, models, updated_at}` | P1 |
| PUT | `/api/v1/vision-adapter` | S | Replace config; `models` entries validated against catalog (`vision` capability, and the entry must name a model the chat plane serves) | P1 |

Behavior (ported from `capacityAdapter.vision`): when a request carries image content and the resolved
model lacks vision, the router prepends adapter models (respecting `round_robin`) and strips the
adapter model from the response identity. v1 ships **vision only**; the adapter framework is
capability-generic internally (pdf/audio/video adapters from the reference are **not** ported).

### 7.9 Token Saver

The configuration surface is here; what the engine does with it is
[`002-TOKEN-SAVER.md`](./002-TOKEN-SAVER.md) (SPEC-API-002), the P3 spec that ships the native engine.

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/token-saver` | S | Full config (below) | P2 |
| PUT | `/api/v1/token-saver` | S | Replace config (validated) | P2 |

```json
{
  "rtk":      { "enabled": false, "filters": [] },
  "headroom": { "enabled": false, "url": "http://localhost:8787", "compress_user_messages": false },
  "caveman":  { "enabled": false, "level": "full" },
  "ponytail": { "enabled": false, "level": "full" }
}
```

- `rtk` is the native engine's compressor group: `enabled` turns the pass on, and `filters` is the
  allowlist of the twelve canonical filter names, where an empty list means every filter is eligible
  (SPEC-API-002 §4). It carries no level; the allowlist is its strength.
- `headroom` calls the external `/v1/compress` endpoint with a 5s timeout; **fails open** on error
  (SPEC-API-002 §7.4, §7.5).
- The example above is also the delivered default: every saver ships **off** (owner decision,
  2026-09-19), so the request path is byte-neutral until an operator enables a group.
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
from the reference README is preserved in adapter docs); no silent cloud fallback: missing `base_url`
⇒ `VALIDATION_ERROR`.

`POST /audio/transcriptions` keeps the OpenAI-compatible multipart passthrough for OpenAI-shaped providers.
The first provider-specific adapter is Deepgram STT: the gateway sends the uploaded audio bytes with a
sanitized audio MIME type, `model`, `smart_format=true`, `punctuate=true`, and either `language` or
`detect_language=true`, authenticates as `Authorization: Token`, and normalizes
`results.channels[0].alternatives[0].transcript` to `{text}`. `POST /audio/speech` also has the first
provider-specific TTS adapter, NVIDIA NIM: `{input:{text}, voice, model}`, `Authorization: Bearer`, and
WAV output, with provider voice default `default`. Cartesia TTS is also adapted: `{model_id, transcript,
voice?, output_format}`, `X-API-Key`, `Cartesia-Version: 2024-06-10`, and fixed MP3 output.

Seven further TTS formats are adapted from the same reference handlers. ElevenLabs sends `{text,
model_id, voice_settings}` with `xi-api-key` and the voice id as the target's last path segment. MiniMax
and MiniMax CN send `{model, text, stream:false, language_boost, output_format:"hex", voice_setting,
audio_setting}` with `Authorization: Bearer` and decode the hex answer. Inworld sends `{text, voiceId,
modelId, audioConfig}`, authenticates with `Authorization: Basic`, and decodes a base64 answer. PlayHT
sends `{text, voice, voice_engine, output_format, speed}` and splits one `userId:apiKey` credential into
`X-USER-ID` and `Authorization: Bearer`. Coqui and Tortoise are self-hosted and send `{text, voice?}`
with no credential, returning WAV bytes. Gemini TTS puts the model in the path (`…/models/{model}
:generateContent`), the credential in the `key` query parameter, and wraps the base64 PCM answer in a
RIFF/WAVE container.

`POST /audio/transcriptions` has a second provider-specific adapter, Gemini STT, on the same
`…/models/{model}:generateContent` surface: the credential rides in the `key` query, the uploaded audio
travels inline as base64 `inline_data` with its `mime_type`, and the prompt is the caller's own `prompt`
field or the reference's transcript instruction, with `Language: <lang>.` appended when the caller set a
language. The answer joins the first candidate's text parts into `{text}`; an answer with no candidate is
an empty transcript rather than a refusal, because silence transcribes to nothing.

A provider format without an adapter remains an explicit `PROVIDER_NOT_ROUTABLE` refusal naming the
format; this is the incremental G5 boundary, not a claim that all reference adapters are present. Five
formats stay on that boundary, registered as G21 in the P2 gap register: AssemblyAI STT (upload →
submit → poll), AWS Polly (SigV4 signing, which the reference does not implement), Edge TTS and Google
TTS (a token scraped from a vendor HTML page, whose registry `base_url` is a marker rather than a URL),
and Local Device TTS (spawns host binaries).

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
per-endpoint proxy binding is deferred (not in the reference either: reference assigns pools globally
with per-provider strategy overrides in `settings.providerStrategies`).

The settings are read per request, so enabling a proxy takes effect on the next outbound call rather than
the next boot. `outbound_proxy_url` is the proxy for every outbound call except the hosts named in
`outbound_no_proxy`: a comma-separated list of hosts or domain suffixes, with `*` exempting everything.
A proxied call still has its destination validated against the egress allowlist (§9), because the dialer's
guard sees the proxy's address rather than the destination's; a settings read or a proxy URL that fails
refuses the call rather than dialing direct, since quietly bypassing a proxy the operator enabled is the
failure this setting exists to prevent.

### 7.12 Usage & Quota Tracker

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/usage/summary` | S | Totals + group-by; `?from=&to=&group_by=provider|model|endpoint|gateway_key` | P1 |
| GET | `/api/v1/usage/timeseries` | S | Buckets; `?granularity=hour|day&from=&to=` | P1 |
| GET | `/api/v1/usage/records` | S | Paginated raw records; filters as summary | P1 |
| GET | `/api/v1/usage/records/{request_id}` | S | Single request detail (joins request log if captured) | P1 |
| GET | `/api/v1/usage/live` | S | Server-sent events: the requests in flight now, the requests that just finished, and the provider the gateway last reported an error for | P1 |
| GET | `/api/v1/quotas` | S | All quota windows: `{endpoint_id, provider_id, window, used, limit, resets_at, source}`; `source` ∈ `computed|reported` | P1 |
| GET | `/api/v1/quotas/{endpoint_id}` | S | Windows for one endpoint | P1 |
| PUT | `/api/v1/quotas/{endpoint_id}` | S | Set budget caps `{monthly_cost_usd?, monthly_tokens?}`: router stops picking exhausted endpoints | P2 |

Quota worker re-checks windows per provider cadence (5h/daily/weekly/monthly), refresh countdowns,
and records `resets_at`. Cost figures are estimates for display only (reference parity).

**The live stream (draft 013 F4, 2026-09-22).** `GET /api/v1/usage/live` is the one Usage read that is
not a period window: an in-flight request exists only between two reads, so no poll can state one, and
the route carries the three facts a window cannot. Each frame is **full state**, not a delta, for
`active` (the requests being routed right now, oldest first), `recent` (the requests that just finished,
newest first), and `error_provider` (the provider the gateway last reported an error for, or an empty
string). Both lists are always arrays and never `null`: an empty in-flight set is a fact, and `null`
would be a second spelling of it. The frame is sent when that state changes, and an idle connection
receives an SSE comment (`: ping`) rather than a frame, so a client counting frames is never told a
change happened. The status line and the SSE headers are committed with the first frame, which is the
§4 rule every stream in this API follows: a failure before that point is an ordinary HTTP error in the
management envelope, and the session gate answers before any of it. Both reads behind the frame are
bounded: `recent` is a fixed window (5 minutes) and a fixed row limit (20), and `active` is bounded by
the marker set it reads, so the frame has a fixed maximum size however busy the gateway is. Nothing on
this path aggregates and the frame has no field for a total, so the stream is structurally unable to
restate what `summary` and `timeseries` returned. The in-flight set is a Redis sorted set
(`pannelai:usage:active`) scored by each request's start instant, written by the chat, media, and
embeddings planes at their outbound seam and removed when the call returns; a marker older than 60
seconds is treated as stale and pruned, so a gateway that dies mid-request cannot leave a node lit
forever. The 60 second window matches the panel's own guard, so both halves of that rule use one
figure.

**Aggregate latency is a sum (draft 010 F3, 2026-09-22).** `latency_ms` on a `UsageTotals` block,
which is the summary's own totals, every `groups[]` row, and every `buckets[]` point, is the **sum**
of the matched requests' own durations, so it grows with the request count and is not comparable
between two windows of different traffic. It is deliberately not a mean: the field is the sum the
projection already computes, and changing it would silently restate every stored comparison. The
per-request picture is `latency_p50_ms` and `latency_p95_ms`, computed over the same rows by the
database; `latency_ms` on a single `UsageRecordResponse` is that one request's own duration. The
contract states all three semantics on the fields themselves, so a consumer that only reads the
served OpenAPI document learns them without this section.

**Free-text `q` scope (draft 010 F8, 2026-09-22, owner decision D4 = expand).** `q` is a
case-insensitive substring match over the row's identity and failure fields as well as its model,
because the panel's placeholder promises a request id and an error code and an operator searching
the id visible on screen must not read an empty table. Usage reads (`summary`, `timeseries`,
`records`) match `request_id`, `id`, `error_code`, and `model`; `logs/requests` matches
`request_id`, `error`, and `model`. The value is still one bind parameter bounded to 200 characters,
so the pattern is never built from caller text (OWASP A05), and a value matching nothing answers an
empty `200` rather than an error.

**Ingest and enforcement (register G22, 2026-09-20).** A served request advances the
Redis window counters for the endpoint it resolved to: chat and embeddings advance by the
tokens the upstream billed, media and the models list advance by one per call (no token
usage), and a call refused before an upstream attempt advances nothing. Counters advance
in the accounting sites (`service/chat_record.go`, `service/dataplane_record.go`) through the
`QuotaRecorder` seam, which is the one path a nil store leaves empty rather than failing
the call. The flush worker drains counters into `quota_windows.used_units` by endpoint and
window, overwriting not adding so a retried batch is not double-counted.

The budget cap is the enforcement half: the selector consults `QuotaCap.Exhausted` over
`MonthlyUsage` before returning an endpoint, and skips any whose month-to-date spend has
reached its stored cap. A missing cap is never exhausted, "uncapped" and "used up" are
distinct, and a failed cap read fails open, so a control-plane outage cannot lock the data
plane out. The quota re-check worker remains deferred to P2 (§10); a cap is enforced on the
next request as soon as one is written, with no separate re-check needed until a provider
reports its own windows.

**Cap write bounds (draft 005 F2/F5, 2026-09-20).** `PUT /api/v1/quotas/{endpoint_id}`
refuses an endpoint id the gateway does not have configured with `NOT_FOUND`, because a cap
on an endpoint no selector reads is a typo the caller should see rather than a row nothing
uses; the existence read is the same `upstream_endpoints` row §7.5 serves. Both cap amounts
are validated against one rule set shared by the wire boundary and the domain constructor,
so the two cannot return different errors for one body: non-negative, at most 1,000,000,000
USD for the cost and at most 1,000,000,000,000 tokens for the token cap (a value past either
is a typo, not a budget), and a zero cost cap with no token cap beside it is refused.

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
  "routing":  { "combo_strategy": "fallback", "combo_sticky_limit": 1, "sticky_limit": 3, "fallback_strategy": "fill-first", "provider_strategies": {} },
  "network":  { "outbound_proxy_enabled": false, "outbound_proxy_url": "", "outbound_no_proxy": "" },
  "token_saver": { "see §7.9" },
  "logging":  { "request_capture_enabled": false, "retention_days": 7, "capture_body_max_bytes": 65536, "observability_max_records": 1000 }
}
```

`routing.fallback_strategy` (`fill-first` | `round-robin`, default `fill-first`) is the credential
rotation default for every provider, and `routing.provider_strategies` overrides it per provider id,
the reference's own `providerStrategies` shape: `{"<provider_id>": {"fallback_strategy": "...",
"sticky_limit": N}}`. An absent entry inherits the global default; an absent `sticky_limit` inherits
`routing.sticky_limit` (default 3). The policy governs the credential walk of any request through
that provider (§7.5), not only combo members; the combo's own rotation stays `combo_strategy` /
`combo_sticky_limit`. `provider_strategies` is written whole, like the reference's PATCH, so a
read-modify-write is what changes one provider.

### 7.15 Data Plane (OpenAI/Anthropic/Gemini-compatible, gateway-key auth)

Model string resolution order (mirrors reference `getComboModels` + aliases):
`combo name → alias → provider/model → 404 MODEL_NOT_FOUND`.

| Method | Path | Wire format | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/models` | OpenAI | `{object:"list", data:[...]}`: routable models + combos | P1 |
| POST | `/api/v1/chat/completions` | OpenAI | Chat, stream + non-stream | P1 |
| POST | `/api/v1/responses` | OpenAI Responses | Responses API | P3 |
| POST | `/api/v1/messages` | Anthropic | Claude messages, stream + non-stream | P1 |
| POST | `/api/v1/messages/count_tokens` | Anthropic | Token counting | P3 |
| POST | `/api/v1/embeddings` | OpenAI | Embeddings | P1 |
| POST | `/api/v1/systemone` | Native | System One (Jev) decision models | P2 |
| POST | `/api/v1/audio/*`, `/images/*`, `/videos/*`, `/search` | OpenAI-ish | See §7.10 | P2 |

Request pipeline (port of `sse/handlers/chat.js` + `open-sse`):

```
auth → schema validation → bypass detection (naming/warmup) → model resolve
  → required-capability detection → combo/adapter augmentation (§7.7, §7.8)
  → token savers (§7.9) → format translation (OpenAI↔Claude↔Gemini)
  → endpoint+key selection (cursor, parked keys, quota; per-request credential
    failover within the provider, §7.7) → upstream call
  (timeout/retry/proxy) → response translation → usage + quota + log recording
  → SSE passthrough / JSON response
```

Authentication runs first, before the body is read or validated: an unauthenticated caller is refused
with `401 UNAUTHORIZED` whatever its body looks like, so a malformed payload cannot be used to probe the
request schema, and an oversized body is never read for a caller who has presented no credential
(register F2 of draft 009).

**The panel playground is a §7.15 client, not a route group.** The reference serves its basic-chat
page from the same origin; pannelAI's panel is a separate app, so its playground calls the routes
above with `Authorization: Bearer <gateway key>` it creates through §7.3, exactly like a CLI tool.
No playground-specific endpoint exists, and the page's contract lives in SPEC-UI.

### 7.16 Skills (agent skill catalog)

The reference's `/dashboard/skills` page is a static catalog of **agent skill documents**: one
`SKILL.md` per gateway capability, hosted in the product's own repository, whose raw URL the user
copies into any AI agent so the agent learns the capability without the operator explaining it.
pannelAI ports the same shape: the gateway serves the catalog, the documents live in the owner's
skills repository, and the panel renders what the route returns. The catalog is embedded static
data (Locked Decision 3's registry rule), carries no database table, and lists one entry per
capability endpoint this gateway actually serves, the entry skill first.

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/skills` | S | `{data: [{id, name, description, endpoint, entry, raw_url, blob_url}]}`; `entry` marks the index skill | P4 |

`endpoint` is the §7.15 path the skill teaches (`/chat/completions`, `/images/generations`,
`/audio/speech`, and the rest); `raw_url` and `blob_url` derive from the repository constants
beside the catalog, the same derivation `src/shared/constants/skills.js` performs. The documents
themselves are owner-authored content in that repository: the route serves metadata and links,
never a `SKILL.md` body, so no capability doc can drift from what the gateway serves.

### 7.17 API Docs (machine-readable contract)

The reference keeps its docs in an external gitbook; pannelAI serves the contract itself, because
a self-hosted gateway's panel cannot assume internet access, and the egress guard (§9) exists to
notice when it tries.

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/openapi.json` | S | The OpenAPI document of both planes; `application/json`, no error envelope | P4 |

The document is embedded at build from the same source of truth as this spec and names every
registered route of §7; a route missing from it is a build failure, not a doc lag. The response is
the document itself, so a reader can diff the served contract against the spec that produced it.

### 7.18 Changelog (served release notes)

The reference's `ChangelogModal` fetches a GitHub changelog URL from the browser. pannelAI serves
its own history instead: the binary carries the release notes it was built from, the panel reads
them over the management API it already has, and nothing external is fetched.

| Method | Path | Auth | Description | Phase |
|---|---|---|---|---|
| GET | `/api/v1/changelog` | S | `{data: [{version, date, title, notes}]}`, newest first | P4 |

Entries are embedded at build from the repository's changelog file. The route is read-only and has
no database table, because the changelog describes the binary and the binary carries it.

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
| `UPSTREAM_REJECTED` | 400 | Data plane: the upstream rejected the request itself (a 4xx other than 401/402/403/404/429); the failover chain stops |
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
   `go test -race`, `gitleaks`: all wired via `scrypts/` hooks/gates (pre-commit, pre-push).
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
   and private ranges are refused unless the operator names them in the allowlist: a self-hosted
   upstream (§7.10) needs its address listed, and a malformed entry fails the boot.

## 10. Delivery Phases

| Phase | Content | Exit criteria |
|---|---|---|
| **P0** | Go skeleton, config, migrations, health/version, auth, gateway keys, `scrypts` gates | login + key CRUD tested (happy/validation/auth per route) |
| **P1** | Provider registry port (YAML), upstream endpoints + multi-key, models catalog, combos (fallback/round_robin), vision adapter, chat data plane (OpenAI + Anthropic), embeddings, usage + quotas, logs, settings | CLI tool completes a request through combo fallback; `-race` clean |
| **P2** | OAuth providers + refresh worker, fusion strategy, combo test, media providers + media data plane, proxies, token-saver config, budget caps, custom/alias/disabled models | OAuth provider round-trip; TTS/embeddings passthrough verified |
| **P3** | Responses API, count_tokens, native token-saver engine (new spec: `002-TOKEN-SAVER`) | parity spot-checks vs reference (002 §9 records their status) |
| **P4** | Skills catalog, openapi.json, changelog routes (§7.16 to §7.18); panel playground (SPEC-UI page over §7.15, no new route) | the three routes tested (happy/validation/auth per route); the served openapi.json names every registered route |

*Changelog 2026-09-17: §7.4 publishes `routability` (`native` / `connector`) and the per-kind `media` block; §8.1 records the five wire shapes settled before the endpoint DTOs (auth-type mapping, one create shape for single and bulk, the OAuth bulk import shape, all-or-nothing batch semantics, and per-kind credential placement). Rationale: four providers on the owner's list speak protocols P1 does not translate, so "configured but always failing" needed to become a value the panel reads, and the bulk routes needed a shape before their DTOs existed.*

## 11. Locked Decisions

1. Single version prefix `/api/v1`; data plane lives under it (clients set `base_url = http://host:8080/api/v1`).
2. Multi-key-per-endpoint is a **new** capability not present in the reference (explicit user requirement).
3. PostgreSQL + Redis replace SQLite; provider registry is embedded static config, not DB rows.
4. Token saver = config surface in §7.9; the native engine ships in P3 under
   [`002-TOKEN-SAVER.md`](./002-TOKEN-SAVER.md) (this decision is amended by that spec, as §10's P3 row
   always planned).
5. `app-ui` never touches PostgreSQL/Redis; management API is its only surface.

## 12. Open Questions (resolve before P1 code)

1. Default listen port for `app-serv` (reference used 20128): propose **8080**.
2. Session token format: opaque random nonce + HMAC-SHA256 signature in `pannel_session`; Redis stores the nonce digest with TTL for revocation. JWT is not used because immediate revocation is required.
3. Registry port scope: all 120+ providers in P1, or apikey-category first (~40) and OAuth set in P2?

---

*Changelog 2026-09-11: initial draft (feature mapping from reference `~/ai-gateway` @ 9Router 0.5.55); same day: added 002 OpenAPI companion cross-link + OAuth callback headless mode.*
*Changelog 2026-09-17: P0 auth contract implemented: `/api/v1` session routes, HMAC-signed `pannel_session`, Redis revocation/lockout, and session-gated gateway-key CRUD.*
*Changelog 2026-09-16: marked the `caveman` token-saver key DEPRECATED with removal scheduled for `/api/v2`, to match the owner decision; linked the panel contract at `docs/SPEC-UI/001-SPEC-UI.md`.*
*Changelog 2026-09-16: §8 adds `METHOD_NOT_ALLOWED` (405): the router registers routes method-aware, so a wrong verb is rejected before any handler runs and needs a code that maps to 405 rather than borrowing `VALIDATION_ERROR` (which §8 binds to 400). §6 records that `gateway_keys.name` is unique.*
*Changelog 2026-09-17: §7.4 and §7.5 add the two routes the owner requirements name and the spec lacked: custom provider nodes (OpenAI-compatible / Anthropic-compatible) in §7.4, and multi-account bulk onboarding (endpoint batch, key batch, OAuth credential import) in §7.5. Both are P1, because the reference ships the node feature at the same surface level and every provider is multi-account by requirement.*
*Changelog 2026-09-19: §7.4 records the callback's two answers and the redirect-origin rule, because the panel (SPEC-UI §6.3) has to read the outcome the browser lands with. The callback is public by necessity and its origin never comes from the request: a configured `PUBLIC_BASE_URL` wins, the staged `redirect_uri` origin is only a fallback, and with neither the route answers JSON instead of redirecting.*
*Changelog 2026-09-19: §7.7 records the fusion execution the reference fixes, now that the data plane runs it: a non-streamed tool-less panel, the judge receiving the client's request plus the directive, the 1-answer and 0-answer degradations, and that panel spend stays outside the request's usage row. Also: a combo whose leading reference no longer resolves now starts from the first reference that does, instead of failing the whole combo: the engine already skipped such members.*
*Changelog 2026-09-19: §7.7 records round_robin execution: the engine asks `ComboService.Order` for the order through the `dataplane.ComboOrderer` seam, so the distribution rule stays in one place and the combo's own `sticky_limit` is what reaches the store. An order that cannot be produced falls back to priority order, because rotation is an optimisation, not a correctness input. Before this, `ComboService.Order` had no production caller and the engine walked the stored order for every strategy.*
*Changelog 2026-09-19: §7.7's combo test route now fixes its answer: `{combo_id, combo, strategy, results:[{ref, role, ok, provider_id, model_id, endpoint_id, latency_ms, error_code, error}]}`, one entry per stored reference with a fusion combo's judge last (`role: "judge"`). Each probe is a one-token non-streaming chat sent through the normal pipeline, so an alias or a nested combo resolves the way a real request would, and a failed probe is a `results` entry rather than a route error: refusing the whole answer because one member is down would hide the members that are up. Only an unreadable combo is 404. Probes run sequentially, so one click spends one account at a time and the reported order is the stored one. Probe spend is not written to `usage_records`: the route carries no gateway key or request id, and its spend is an operator diagnostic rather than client traffic.*
*Changelog 2026-09-19: §7.11 records the proxy pool's shapes and rules, now that the surface is implemented: the password is write-only (`has_password` in the response; an empty value on PATCH keeps the stored secret), and both test routes answer `{state: "ok"|"fail", latency_ms, checked_at}` plus an additive `message`. A failed probe is a 200 carrying that fail state rather than an error, because "does this candidate work" was the question. `POST /proxies/{id}/test` stores its finding so the list reports the same proof; `POST /proxies/test` stores nothing. A repoint or a credential change clears the stored status: a proof measured against the old address is not a proof of the new one: while saving an unchanged form does not. Every test runs under the new egress guard (`internal/netguard`, OWASP A01): the candidate's host is resolved and every answer validated before any dial and re-validated at connect time, loopback and private ranges are refused unless the operator names them in `EGRESS_ALLOWED_TARGETS`, and the fetched URL is the server's `PROXY_TEST_URL`, never a request field.*
*Changelog 2026-09-19: §7.10 records the media surface now that both planes are implemented. Management: overrides are stored per provider **and** kind (`media_provider_settings`, PK `(provider_id, kind)`), and every resolved value reports its own source (`registry`|`override`) rather than one flag for the pair, because a provider may have its `base_url` overridden while its default model still comes from the registry. A PATCH that would leave no `base_url` from either source is refused (`VALIDATION_ERROR`): the "no silent cloud fallback" rule above, enforced at save time rather than discovered as a dial to the wrong host. Data plane: the six routes keep the reference's wire shapes: `POST /audio/speech` answers bytes as `audio/{fmt}` unless `?response_format=json` asks for `{audio: base64, format}`; `POST /audio/transcriptions` forwards the upstream's answer verbatim (JSON as `application/json`, anything else as `text/plain`) because re-encoding would drop the fields the caller asked for with `response_format`, and it forwards `language`/`prompt`/`response_format`/`temperature` only to OpenAI-compatible providers, which is where the reference forwards them; `GET /audio/voices` serves the catalog the block declares in the registry (`voices:`), and a provider declaring none is refused by name; `POST /images/generations` normalizes to `{created, data:[{url|b64_json}]}`; `POST /videos/generations` is registered but refuses `PROVIDER_NOT_ROUTABLE`, because no provider declares `video`: reference parity, where `videoConfig` is recognized and nothing defines it; `POST /search` builds its payload from the block's declared `query_param`/`max_results_param` (GET reads them as query parameters, POST as a body), because search APIs disagree on `q`/`query` and `num`/`count`. Model strings are `provider/model`, split on the first slash so slash-carrying ids (`openrouter`'s `openai/gpt-4o-mini-tts`) resolve to their provider and keep the whole model; the reference's `provider/model/voice` shorthand is deliberately not carried, because it cannot be told apart from such an id. A provider declaring a media format the gateway has no adapter for is refused by name (`PROVIDER_NOT_ROUTABLE`) instead of being dialed with a wrong-shaped payload: an honest gap rather than an outage-shaped one. Media routes authenticate the gateway key before reading any body: a 25 MB multipart upload is never parsed, or spilled to disk, for a caller without a key.*
*Changelog 2026-09-19: §9 makes the egress policy binding for every upstream dial, not only the proxy test. The chat, embeddings, and media transports, the connectivity probe, and the OAuth token client now share the process's `internal/netguard` guard, its `EGRESS_ALLOWED_TARGETS` allowlist, and the one HTTP client built on its dialer; a probe of a refused address reports the refusal (`the upstream address was refused: …`) instead of an unreachable host. A self-hosted upstream on loopback or a private range therefore needs its address allowlisted, and a malformed entry fails the boot rather than widening what the gateway may reach.*
*Changelog 2026-09-19: §7.12/§7.13 record the media plane's accounting now that it is wired (register G6, D3 = usage + log + request_count). Every §7.10 media call and every `POST /embeddings` writes exactly one `usage_records` row and one `request_logs` row, both under the router's request id so the pair is reachable from either surface; a call that fails is recorded too (`status: error`, `error_code` set), because the row is what makes a failure visible in the panel rather than only in the gateway's own log. Tokens are 0: a media answer reports no token usage: a search call's `cost_usd` is the registry's `cost_per_query` and its `model` column carries the provider id because the route names no model, and the log row's error text is `CODE: message` without the wrapped cause: a media URL may carry the credential as a query parameter (§8.1), so an upstream's message is quoted but our own error chain is not. Log bodies stay empty for media, because the reference records request detail for chat only. A recording failure never fails a call the client already got an answer to, the same rule the chat plane follows. §7.3's `request_count` and `last_used_at` now advance once per authenticated data-plane call: chat, models list, media, embeddings: written at the single authentication choke point, so one call counts exactly once, and the counter advances in the database (`request_count = request_count + 1`) rather than by read-modify-write, so concurrent calls do not lose counts.*
*Changelog 2026-09-19: §7.12 records the budget cap's read-back (owner decision D5 = b, register G12): `GET /api/v1/quotas/{endpoint_id}` now answers `{endpoint_id, cap, data}`, where `cap` is the stored cap or `null` when none is set, so a client that just wrote a cap reads the same values back before any usage window exists: the case that previously answered `{"data":[]}`. The field is an explicit `null` rather than an omitted key, because "no cap" is a state the panel renders as an empty form and an absent field could not be told apart from a response that forgot it. The collection route `GET /api/v1/quotas` stays windows-only, since a cap belongs to one endpoint, and its body is unchanged.*
*Changelog 2026-09-19: §9 records what a failed request leaves in the gateway's own log (register G9): the existing `request handled` line gains a `code` field carrying the machine error code beside the request id and status: the same envelope code §8 already binds to the response, so the operator reads "why" from the one line that already says "what". The field is absent on a served request, so its presence means the request failed; 404 and 405 from the router's own envelope carry their code too. Only the code is logged, never the error message: a data-plane failure's message can quote an upstream's text back, and that text can carry the credential (§8.1). The panic path keeps its id + panic value, which is operator-only text and never leaves the process.*

*Changelog 2026-09-19: §7.12/§7.13 record the chat plane's accounting now that it is wired end to end (register G17/G18). Every chat call: `POST /chat/completions` and `POST /messages`, streamed or not: leaves one `request_logs` row under the router's request id, carrying the client's request body and the served answer for the capture rule to decide about, so capture stays one decision in `LogService.Record` and a call refused before the pipeline ran (an unknown model, an invalid body) is visible in the Logs screen too. A call that reached an upstream attempt also leaves one `usage_records` row, on success and failure alike: the engine now builds the call's identity before anything can fail, so a failed call's row carries the provider, endpoint, model, and combo it was attempted against with `status: error` and the error code: before this, every failure path returned a zero identity, the aggregate rejected the row, and the rejection was swallowed. A call refused before any attempt leaves no usage row: the aggregate requires a provider and a model, and the write is skipped deliberately rather than left to fail validation. Fusion combos follow the same rule: a failed panel reports the first member's identity, the failure the client receives, and a failed judge reports the judge's. The chat log row's error text is the code alone, never the `CODE: message` the media row stores: a chat upstream's rejection quotes the presented credential back in its message, and a stored row must not hold a secret.*
*Changelog 2026-09-19: §7.12/§7.13 record the refusal row now that the media and embeddings planes write it (register G20). A call refused before any upstream attempt: an unknown provider, an untranslated media format, a provider with no base URL, no usable account, an unresolvable model, a combo on the embeddings route: leaves one `request_logs` row under the router's request id, with `status: error` and the error's code alone; it leaves no `usage_records` row, because nothing was spent and the aggregate requires a provider and a model, which is the same deliberate skip the chat plane makes for a request refused before the pipeline ran. The row names the provider and model resolution had reached: a media model string is split so `nope/model` names both columns, a search call that named only a provider is named by it, and a request whose provider never resolved carries no identity rather than a guessed one. A refusal the handler itself answers (an invalid body, a bad key) writes nothing, the same boundary the chat plane keeps.*
*Changelog 2026-09-19: §7.10 records the first incremental G5 adapter: Deepgram STT now accepts the raw uploaded audio bytes, derives a safe audio MIME type from the multipart part or sanitized filename, sends `model`, `smart_format`, `punctuate`, and `language`/`detect_language` query parameters, uses `Authorization: Token`, and normalizes Deepgram's nested transcript to the OpenAI-compatible `{text}` answer. Provider aliases resolve before the adapter is selected (`dg/nova-2` and `deepgram/nova-2` are equivalent). The remaining non-OpenAI media formats stay explicit `PROVIDER_NOT_ROUTABLE` refusals until their own adapters are ported; AWS Polly is not invented because the reference has no builder for it.*
*Changelog 2026-09-19: §7.10 records G5's second incremental adapter: NVIDIA NIM TTS now sends `{input:{text}, voice, model}` with `Authorization: Bearer`, defaults the provider voice to `default`, and labels its raw answer as WAV (including `format: "wav"` in the base64 JSON form). The remaining non-OpenAI media formats stay explicit `PROVIDER_NOT_ROUTABLE` refusals until their own builders are ported.*
*Changelog 2026-09-19: §7.10 records G5's third incremental adapter: Cartesia TTS now sends `{model_id, transcript, voice?, output_format}` with `X-API-Key` and `Cartesia-Version: 2024-06-10`; omitted voices stay omitted, the requested output is fixed MP3 (`128000` bit rate, `44100` sample rate), and the route labels both raw and base64 answers `mp3`. The remaining non-OpenAI media formats stay explicit `PROVIDER_NOT_ROUTABLE` refusals until their own builders are ported.*

*Changelog 2026-09-19: §7.10 records G5's fourth slice, seven TTS adapters in one pass (register G5): ElevenLabs, MiniMax and MiniMax CN, Inworld, PlayHT, Coqui, Tortoise, and Gemini TTS. Each is the reference's own wire shape rather than a generic guess, and the differences are the point: the credential is a declared header for some (`xi-api-key`), a raw-base64 `Basic` value for Inworld, one `userId:apiKey` string split across two headers for PlayHT, and a `key` query parameter for Gemini; the model is a path segment for ElevenLabs (the voice) and Gemini (the model, before `:generateContent`); and the answer arrives as raw bytes, hex, base64, or base64 PCM that needs a RIFF/WAVE header. A provider format without an adapter is still an explicit `PROVIDER_NOT_ROUTABLE` refusal naming the format.*

*Changelog 2026-09-19: §7.10 and the P2 gap register record G21, the five formats deliberately left unadapted: AssemblyAI STT (upload → submit → poll up to 120s), AWS Polly (SigV4 signing, which the reference does not implement at all), Edge TTS and Google TTS (a token scraped out of a vendor HTML page and cached, with a registry `base_url` that is a marker rather than a URL), and Local Device TTS (spawns host binaries: macOS `say`, Windows SAPI, ffmpeg). They are not one builder away from the current pipeline: each needs a second request, a signing scheme, or a host process: so they stay refused by name until a seam for multi-step adapters is designed. Gemini STT remains ordinary G5 work: it is a single `generateContent` with inline audio.*

*Changelog 2026-09-19: §7.11 records that the proxy settings are now honored on the dial path (register G4, owner decision D2 = a): `settings.network.outbound_proxy_*` is read per request and decides the route for every outbound call the shared client makes, so an operator's proxy takes effect on the next call rather than the next boot, and `outbound_no_proxy` exempts hosts by exact name or domain suffix (`*` for all). The security half is stated in the section because it is the reason the check lives where it does: a proxied request never dials its destination, so the destination is validated against the egress allowlist before the route is returned: otherwise enabling a proxy would switch the A01 policy off for every call: and a settings read or proxy URL that fails refuses the call rather than dialing direct. Per-endpoint binding stays deferred.*

*Changelog 2026-09-19: §6 records the schema's ownership rule (register G19): every table belongs to the role that owns `gateway_keys`. A migration runs as whichever role boots the process, so a superuser boot created `media_provider_settings` and `proxies` owned by that superuser while the application role could not read them: measured live as `permission denied for table media_provider_settings` on `PATCH /media-providers/{id}` and on every §7.11 route, while all other tables answered. Migration `000011` re-owns the two to the anchor role, and because a role that does not own a table cannot re-own it, a refusal is a boot warning naming the statement to run instead of a boot that never comes up.*

*Changelog 2026-09-19: §7.10 records G5's last adapter, Gemini STT, which closes the G5 register item: the transcription route now speaks Gemini's `generateContent` surface. The model is a path segment before `:generateContent` and the credential stays in the `key` query parameter, both as in the TTS adapter; the audio travels inline as base64 `inline_data` with the `mime_type` the upload's own resolver produced, so a non-audio content type is never copied through; and the prompt is the caller's `prompt` when it wrote one, the reference's transcript instruction otherwise, with `Language: <lang>.` appended when a language was set. The answer joins the first candidate's text parts into `{text}`, and an answer with no candidate is an empty transcript rather than a refusal: silence transcribes to nothing. The five remaining formats stay `PROVIDER_NOT_ROUTABLE` refusals by name under G21.*

*Changelog 2026-09-19: §7.15's `count_tokens` route is implemented, the first P3 slice. `POST /api/v1/messages/count_tokens` answers `{"input_tokens": N}` from the reference's estimate: the characters of every message's text content are summed (a bare-string body and an array of text blocks measure the same, and non-text blocks count nothing) and divided by four, rounded up. The route dials no upstream and resolves no model, so it writes no `usage_records` and no `request_logs` row (the same boundary the models list keeps) while the presenting key's `request_count` still advances once through the §4 choke point. It enforces the §4 gateway-key rule (either credential header) and refuses a body that fails the Anthropic contract (`model` required, message roles limited to `user`/`assistant`, tools validated) with `VALIDATION_ERROR`, while tolerating the members the gateway does not model (`betas`, `metadata`) because the caller is a CLI tool. The estimate deliberately counts message text only (`system` and `tools` are not measured) because §10's exit criterion for P3 is a parity spot-check against the reference, and the reference counts text parts only; a tokenizer would report a different number than the client would have received from the reference.*

*Changelog 2026-09-19: §7.15 records the Responses API as a routable upstream target, the second P3 slice, so the set §8 refuses as `PROVIDER_NOT_ROUTABLE` shrinks by one format. `targetFormat` now maps `registry.FormatOpenAIResponses` onto `TargetResponses`, so a custom node declaring `api_type: responses` resolves and is translated instead of being refused by name; Gemini stays deliberately absent, because the registry reports every provider declaring it as `connector` and a format with no reachable route must not decode as translatable. The request direction is one mapping per pair rather than one per combination: an OpenAI chat request becomes a Responses request directly, and an Anthropic messages request reaches the same vocabulary through the OpenAI chat request, which is the pivot the reference's registry uses. The rules the API enforces are the reference's: the first `system` or `developer` turn becomes `instructions` and later ones are dropped (the API has one instruction field, and joining turns would tell the model something the client never asked for); a `user` or `assistant` turn with content becomes one `message` item whose text parts are `input_text` for a non-assistant role and `output_text` for an assistant one, since the API splits text by who wrote it; a tool result becomes a `function_call_output` item; an assistant turn's calls become `function_call` items emitted after its message item; `store` is always `false`; `max_tokens` is omitted when the client set none rather than sent as zero, which some providers read as "produce nothing"; tool declarations are flattened out of OpenAI's `function` wrapper and gain an empty object schema when they carry none; and a `call_id` is clamped to 64 characters while a nameless call is skipped, because the API rejects both outright. The answer direction reads one decoded form for both client wires: text is the `output_text` parts joined with newlines, reasoning is a `reasoning` item's summary with a `content` fallback for providers that report it there, a nameless call is dropped because the client could not dispatch it, and the finish reason is `tool_calls` when a call is present, `length` when the answer is `incomplete` for `max_output_tokens`, and `stop` otherwise. Accounting follows the reference's note that `input_tokens` already includes the cached tokens, so the cached split is reported as a detail rather than added to the prompt count; the Anthropic envelope then subtracts it, because Anthropic's input count excludes what OpenAI's includes. The stream is a named-event stream rather than a chunk stream, so each event is mapped onto the OpenAI chunk vocabulary the two existing client stream states already consume, which is what keeps this slice from being two more translators: text and reasoning deltas become `content` and `reasoning_content`; `response.output_item.added` opens a call (with a deterministic `call_pannelai_<n>` id when the provider omits one) and `response.output_item.done` advances the index, so a second call does not overwrite the first; `response.completed` and `response.done` close the answer with its accounting; `response.failed` and `error` surface as content carrying `[Error] <message>`, which is the reference's behaviour and the only channel a client mid-stream has; and every other event produces no frame at all, because an empty frame makes a client that counts frames mis-count the answer. The `POST /api/v1/responses` client route itself is the next P3 slice.*


*Changelog 2026-09-19: §7.15's `POST /api/v1/responses` route is served, the third P3 slice, so a coding agent on the Responses wire reaches every provider the registry offers. The request direction pivots through the OpenAI chat request: `instructions` becomes the leading system turn; the `input` union (a bare string or an item array) becomes the message list; an item's `type` decides its role with the `role` fallback the CLI tools rely on; a `function_call` item becomes an assistant turn's tool call and a `function_call_output` item becomes a tool result whose JSON string is unwrapped, so the model reads the payload rather than a quoted copy of it; `image_url` is read as the bare string this wire carries; `max_output_tokens` becomes `max_tokens` and is omitted when the client set none, because a zero ceiling reads as "produce nothing"; a tool declaration still carrying OpenAI's `function` wrapper is flattened, a nameless one is skipped, and an object schema without `properties` gains one; and the fields only the Responses API knows are dropped rather than leaked into a chat body. A streamed answer always asks the upstream for usage (`stream_options.include_usage`), because this wire reports accounting in its closing event rather than behind a client flag, and the reference omits the request and so reports none. The stream is the API's named-event lifecycle: `response.created` and `response.in_progress` open it once; an item opens before its content and closes after it, through `response.output_item.added`, the part-added event, the deltas, and the part-done and item-done closers; `response.completed` closes it and `data: [DONE]` ends it; and `sequence_number` increases by one across the whole stream. Reasoning, message, and function-call items open lazily on the first delta that needs them, each at its own output index, so a reasoning item and the message that follows it do not share one as they do in the reference, which keys items by the OpenAI choice index. Inline `<think>` markup is routed to the reasoning item and stripped from the answer either way, and text that follows a tool call opens a fresh message item rather than feeding deltas to one the client already saw close. Item ids are `rs_<response>_<index>`, `msg_<response>_<index>`, and `fc_<call_id>`, with a deterministic `call_pannelai_<index>` call id when the provider omits one and `{}` for a call that never reported arguments. The closing event reports the assembled items and the accounting, which the official API does and the reference omits, so the streamed and non-streamed answers describe the same object. Accounting for an Anthropic upstream now merges rather than replaces, because that provider reports the prompt side once and the output side cumulatively, and replacing the block reported a prompt of zero for every Claude call, which the reference avoids by accumulating.*

*Changelog 2026-09-19: §7.9's native token-saver engine is built and wired, the fourth P3 slice, governed by the new [`002-TOKEN-SAVER.md`](./002-TOKEN-SAVER.md) it cites. Three savers run in the reference's order (RTK, then Headroom, then Ponytail) inside the relay, on the already translated upstream body before the transport call, so a rewrite cannot be undone by a later format conversion and only the provider ever sees one; the pipeline covers streamed and non-streamed calls alike. `rtk` on the wire trades its `level` for `filters`, the allowlist of the twelve canonical filter names (an empty list means every filter is eligible), because the allowlist is the compressor's strength; `ponytail.level` stays. Every saver ships off by default (owner decision, 2026-09-19), so the request path is byte-neutral until an operator enables a group, and Locked Decision 4 is amended accordingly. The document is read per request, so a settings change takes effect on the next call rather than the next boot. `X-Token-Saver: off` (§4) now reaches the pipeline as a per-request bypass: only the exact case-insensitive value `off` opts out, and every other value keeps the stored configuration active. The failure direction is fail-open everywhere: a settings read error, a proxy failure, a malformed transform, or a filter panic returns the latest body and the call proceeds, because a saver is an optimization and no request may fail or grow because of one. Headroom's external call shares the process's guarded HTTP client (§9), so a self-hosted compression proxy needs its address in `EGRESS_ALLOWED_TARGETS` exactly like a self-hosted provider, and its endpoint is redacted in errors. The savers write no accounting of their own: the relayed call keeps the one usage row and one log row §7.12/§7.13 already give it.*

*Changelog 2026-09-20: the endpoint audit ([`../DRAFT/003-ENDPOINT-READINESS.md`](../DRAFT/003-ENDPOINT-READINESS.md)) lands. §7.6 adds the `GET /api/v1/models/custom` list route the router already served and tested but the table lacked, closing the one drift the audit found between the registered routes (90) and the contract (89). §2.2 records the owner decision pulling four items back into scope: skills, the chat playground, API docs, and a changelog surface. §7.16 to §7.18 specify them from the reference's surface behavior rather than invention: the reference's skills page is a static catalog of agent skill documents (`src/shared/constants/skills.js`), so §7.16 is one read-only catalog route over embedded data; the reference serves docs from an external gitbook and fetches its changelog from a GitHub URL in the browser (`ChangelogModal.js`), so §7.17 and §7.18 serve both from the binary itself, because a self-hosted panel cannot assume internet access and §9 exists to notice when it tries; and the playground needs no endpoint at all, so §7.15 records it as a data plane client carrying a gateway key. §10 adds P4 to carry the build. The translator playground stays excluded.*

*Changelog 2026-09-20: P4 is built and 001 closes. The three static routes are live: GET /api/v1/skills serves the embedded agent-skill catalog (§7.16), GET /api/v1/openapi.json serves the machine-readable contract from the binary (§7.17), and GET /api/v1/changelog serves the delivery phases' release notes (§7.18). The contract's completeness holds by construction: the mux records each pattern as it registers (routeRecorder), and the router test walks that list against the served document in both directions, so a route added without updating the document fails the build, and so does a stale entry. The routes are tested through the real mux in the session-gated management pattern: 401 without a session, 200 with one, 405 for an unregistered verb. Gates: go build, go test -race ./..., go vet, and staticcheck clean, every touched file under the §1.1 warning threshold.*

*Changelog 2026-09-20: the OpenCode Free discovery fix lands (ported from the reference's PR #4073, OpenCode half). §7.15's list is "routable models + combos", and opencode proved the property could hold in name only: the port had dropped the reference's declared model list, so the provider's free models answered any request through passthrough yet appeared in no models list, and resolved onto the provider's chat-completions wire instead of the per-model formats the reference declares. The registry entry regains those three models (muse-spark-1.2-contributor-free and muse-spark-1.3-contributor-free on openai-responses, union-alpha on claude), which is what makes the list tell the truth again: every declared model is listed, translated to its own wire, and passthrough stays, so an id beyond the list remains answerable rather than 404. The reference's other two mechanisms for this list are deliberately absent: the noAuth virtual connection exists there because the list is connection-gated, which this port's registry-driven list never was, and modelsFetcher (a live upstream /models query) stays unported, since dynamic discovery is the compatible-provider behavior PR #4073's third part fixes, not a property this list has. The PR's first part, per-key model access and token quotas, is a schema, enforcement, and dashboard feature that touches §7.3 and §8; it needs its own spec line before any code, so it is not in this change.*

*Changelog 2026-09-20: §7.12 closes the quota vertical from read-only to enforced (register G22). §7.12's pipeline now has its full write side: every served data-plane call advances the Redis window counters for the endpoint it resolved to (chat and embeddings by the tokens the upstream billed, media and the models list by one call, and a refusal before an upstream attempt by nothing) through the `QuotaRecorder` seam that the accounting recorders (`service/chat_record.go`, `service/dataplane_record.go`) share, so a nil store leaves the screen empty without failing the call rather than the reverse. The read side is enforced too: the selector consults `QuotaService.Exhausted` over `MonthlyUsage` before returning an endpoint and skips any whose month-to-date spend reached its stored cap (the budget gate in `SelectorDeps.Gate`), a missing cap is never treated as exhausted, and a failed cap read fails open so a quota table outage cannot lock the data plane out. The quota re-check worker stated in §6/§7.6 stays deferred to P2; a cap takes effect on the next request as soon as it is written, because enforcement is a selection read against the durable `quota_caps` row, not a separate re-check. The `QuotaWindow.NextReset` and `domain.AccountingKinds` rules are pinned by tests. G22 is registered in the P2 gap register (`docs/DRAFT/001-P2-GAPS.md` §2) as "quota counter, advanced per served request".*
*Changelog 2026-09-22: §7.12/§7.13 close the status filter's closed set (draft `010-USAGE-ENDPOINT-READINESS.md` F2 + F9). The `status` query parameter on `summary`, `timeseries`, `records`, and `logs/requests` is now an enum on the wire (`success`|`error`, contract YAML and served OpenAPI regenerated) and a domain value object in code (`domain.UsageStatus` on `UsageFilter`/`UsageFilterInput`, `domain.RequestLogStatus` on `LogFilter`/`LogFilterInput`, checked at `DecodeUsageFilter` and again at each filter's `Validate`): before, a value outside the set was accepted and the read predicate's "empty means unfiltered" rule turned it into a silently empty 200 instead of a 400, so an operator typing `SUCCESS` read "no failed requests" where the filter was simply unknown. The panel already validated this field as a Zod enum, so the gateway now enforces the same set the panel promised. Both members still filter: an integration test against PostgreSQL seeds three failed and two successful rows and proves `success` reads two, `error` reads three, and the unfiltered read counts five, with the summary's `error_count` agreeing.*

*Changelog 2026-09-20: the quota cap write path closes two audit findings (draft `005-QUOTA-TRACKER-CLOSURE.md`). The `NOT_FOUND` refusal for an endpoint id the gateway does not hold now ships in `QuotaService.SetCap` ahead of any cap validation, with the endpoint existence read carried by a new `EndpointFinder` seam rather than a direct cross-package reach (AGENTS.md §1.5), and a lookup or store failure is wrapped as `INTERNAL_ERROR`, never as `NOT_FOUND`. The cap amount rule set is now single-sourced in `domain.ValidateQuotaCapValues` and shared by the wire validator, so a body and its constructor cannot disagree about what a legal cap is, with explicit upper ceilings that reject a typo (1,000,000,000 USD, 1,000,000,000,000 tokens) before it reaches the driver. The quota flush worker's `batchIdentity` is now keyed by `endpoint_id/window` per endpoint, so one failing batch cannot be dead-lettered against another batch's failures, and the worker file is split into lifecycle, policy, and drain concerns, each under the §1.1 threshold. Handler tests are table-driven (TDD.md §2.5) covering the happy write, the read-back, the unknown-endpoint refusal, and every cap-boundary, and the three routes are session-gated by the whole-table sweep (`router_session_sweep_test.go`).*
*Changelog 2026-09-22: §7.12's Usage surface gains its live stream (draft `013-USAGE-NODE-MOTION-READINESS.md` F4): `GET /api/v1/usage/live` answers `text/event-stream` with full-state frames of `{active, recent, error_provider}`, session-gated like the four reads beside it. The route existed only as a plan in draft 010 §10.3 and the panel built against it anyway (drafts 012 and 013), so the panel's live half was verified against a contract double until now. The three facts have three sources, and each is bounded: `active` is a Redis sorted set scored by start instant, written at each plane's outbound seam (chat through the relay leg, media, and embeddings) and removed when the call returns, with a 60 second staleness prune; `recent` is a 5 minute window and a 20 row limit over the durable rows; `error_provider` is derived from that same recent read inside a 10 second window, so the node the panel marks in error and the failure it lists cannot disagree about which request they came from. An idle connection gets an SSE comment rather than a frame, so a client counting frames is never told a change happened. A failed read ends the stream rather than answering an empty frame, because "nothing is running" is a claim a failed read cannot support (R-36). The frame carries no aggregate field, so the stream cannot restate `summary` or `timeseries`.*
*Changelog 2026-09-22: §4's pagination rule and §7.12/§7.13's filter contract are now stated by the machine-readable contract and enforced by one boundary (draft `010-USAGE-ENDPOINT-READINESS.md` F6 + F7, owner decision D3 = refuse). `per_page` above 100 is a `VALIDATION_ERROR` rather than a silent clamp to 100, matching the refusal the boundary already gave values below 1, so the three layers agree: the panel's Zod `min(1).max(100)`, the gateway's `DecodePage`, and the contract, which now declares `page` as `integer, minimum 1, default 1` and `per_page` as `integer, minimum 1, maximum 100, default 25` on `providers`, `endpoints`, `usage/records`, and `logs/requests` (two declarations that read `type: string` are corrected to `integer`, which is what the decoder parses). The usage window parameters `from`/`to` carry `format: date-time` and `group_by`/`granularity` carry their closed-set enums (`provider|model|endpoint|gateway_key`, `hour|day`) on every read that accepts them, so a generated client learns the rules the boundary already enforced. The served OpenAPI document is regenerated from the YAML by `tools/openapi-gen`, never hand-edited, and the parity is pinned by three new contract tests over the served document: every `from`/`to` is a date-time, the enum parameters match the domain sets, and every `page`/`per_page` declaration carries the documented bounds.*

*Changelog 2026-09-22: §7.12 states two semantics the wire was already sending but never named (draft `010-USAGE-ENDPOINT-READINESS.md` F3 + F8, owner decisions D1 = document and D4 = expand). The aggregate `latency_ms` on a `UsageTotals` block (the summary's own totals, every `groups[]` row, every `buckets[]` point) is the sum of the matched requests' own durations, so it grows with the request count and two windows of different traffic are not comparable through it; the per-request picture is `latency_p50_ms`/`latency_p95_ms`, and `latency_ms` on a single `UsageRecordResponse` is that one request's duration. The contract now carries all three statements on the fields themselves, so a consumer that reads only the served OpenAPI document learns them, and a PostgreSQL integration test seeds 100/200/300 ms rows so the sum (600) and the mean (200) are different numbers: the existing arithmetic test seeded three equal rows, where both readings are 300 and neither is pinned. The free-text `q` filter now matches what the panel's placeholder already promised: `summary`, `timeseries`, and `records` search a case-insensitive substring of `request_id`, `id`, `error_code`, and `model`, while `logs/requests` searches `request_id`, `error`, and `model` (a log row carries error text where a usage row carries an error code, so the two scopes differ and are stated separately). An operator searching the request id visible on screen no longer reads an empty table, the value is still one bind parameter bounded to 200 characters, and a value matching nothing still answers an empty `200`. The `coalesce` the first draft wrapped around the nullable error columns was removed after a mutation check showed it could not fail: `false OR NULL` excludes a row exactly as `false` does and `TRUE OR NULL` is `TRUE`, so the OR can only add a match.*

*Changelog 2026-09-23: §7.7's combo write path, §7.8's adapter, and §7.6's two filters now agree with the router about what a model reference names (draft `024-COMBO-VISION-READINESS.md` F1–F4). The data plane resolves three spellings of the first segment — provider id, registry alias, node prefix — while every write path read only the id form, measured live: `cc/claude-…` and `oczen/big-pickle` were refused with "does not resolve" by the same gateway that routes them. One canonicalizer now backs `ModelExists`, the combo ref and judge checks, the alias-target check, the vision adapter, and both `provider_id` filters, so a name the router routes is a name the API accepts, and `GET /models/custom?provider_id=` — documented since P2 and ignored by its handler — narrows by every spelling with the two-way match a prefix-stored row needs. A combo member that is itself a combo now resolves at runtime: `resolveMember` skipped the combo path, so a nested member saved (the write path accepts one deref level, including a combo name) and then answered `MODEL_NOT_FOUND` as a leading member while working as a later one; resolution follows one link with a depth bound, so a stored cycle terminates instead of recursing, and the answered identity stays the combo the client addressed rather than the inner one. Finally, a combo member, judge, or adapter model must be **servable by the chat plane**: a provider whose wire format has no translator and a media model both resolved and then failed the first request (measured: `PROVIDER_NOT_ROUTABLE`, and a chat selector that never reaches an `image` row), so both are now refused at write time with the reason named, which is the same property §7.15's list already held — listed and answerable are one property. What is deliberately unchanged: the reference validates no combo reference at all (its picker offers only active connections), and this port keeps its documented write-time validation; "filter by active provider" is the panel's picker concern and stays in `app-ui`.*

*Changelog 2026-09-24: §7.6's catalog read gains the `active` filter (draft `025-CATALOG-ACTIVE-FILTER.md`), which moves the reference's picker rule to the server. The reference offers only active providers because its picker filters `activeProviders` in the client; this port's catalog listed every model the registry declares, measured live as 587 rows across 67 providers while exactly one provider held an active endpoint — so 586 rows were offered that answer `NO_PROVIDER_AVAILABLE` on the first request (`opencode/muse-spark-1.2-contributor-free` and `mmf/gpt-5` both measured as 503 with no endpoint configured). `?active=true` now narrows the answer to rows whose provider holds at least one endpoint in status `active`, which is the router's own candidate population (`selection.go` selects `ProviderID: resolution.Provider.ID, Status: active`), and the predicate is asked once per read through the same `EndpointStatusCountsByProvider` roll-up the provider list already uses. "Active" is the status, not the moment: a rate-limited active endpoint keeps its provider active because the runtime skip is a rotation, not a configuration, while disabled and errored endpoints do not. `active=false` and an absent parameter both narrow nothing, so the default read is unchanged; `yes`, `1`, and `TRUE` are `VALIDATION_ERROR` rather than silently unfiltered, which is the same refusal the usage status filter already gives and the reason it was made a closed set. A deployment that wires no counter refuses the narrowed read by name instead of answering the whole catalog under a parameter that promised the opposite, and the plain read keeps working without the seam. What is deliberately not in this change: the panel's picker (the caller side, `app-ui`) and a virtual no-auth connection like the reference's, which this port's router does not have — `no_auth` without an endpoint is measured as unservable here, so it is not counted as active.

*Changelog 2026-09-24: §7.5, §7.7, §7.15, and §8 re-state the failover order the data plane now implements (draft `028-FALLBACK-CREDENTIAL-PARITY.md` F1-F4, owner decision: the reference's semantics for all four). The order a failed request walks is credential-first: the provider's next healthy endpoint or key is tried inside the same request before the next combo member is, which is the account loop the reference runs (`chat.js` `excludeConnectionIds`) and this port never had. The first upstream failure parks a key for a class-specific window instead of the fixed three-strike circuit: 401/402/403/404 for 2 minutes, 429 exponentially 2s to a 5m cap driven by consecutive failures and reset by a served call, 5xx/network/timeout for 30s, and a request-scoped 4xx parks nothing because the request, not the credential, is the cause; healthy keys inside one endpoint rotate least-recently-used first so an idle second key is no longer dead weight. A request-scoped 4xx also stops the chain and answers as `UPSTREAM_REJECTED` (400), a new §8 code the panel's closed enum now carries, because the same body would be refused identically by every other member. When every member fails the client receives the first failure's status with the last failure's message, and only a member that actually reached an upstream call may own the recorded identity or the usage row, so a stale combo member (a provider whose endpoints were deleted, a model that no longer resolves) can no longer name itself in the client's error or the accounting; deleting such a provider is refused by name while a combo still references it. Deliberate deviations from the reference, recorded in the draft: parking is per key, not per key-and-model pair (`modelLock_${model}`), the failure-text rules are not ported, and media/embeddings keep their own single-call error shape because they run no chain.*

*Changelog 2026-09-24: §7.5 and §7.14 add the per-provider credential rotation policy (draft `030-CREDENTIAL-ROTATION-POLICY.md`), the reference's own switch read from `auth.js:139` (`providerOverride.fallbackStrategy || settings.fallbackStrategy || "fill-first"`) and edited per provider from its Connections card and globally from its profile page. Measured before the change: this port had no `fill-first` mode at all, and the panel's `routing.sticky_limit` was stored and validated but never read by the data plane, whose selector took its limit from the `DATA_PLANE_STICKY_LIMIT` environment knob, so the only rotation switch that actually reached the router was the combo's. `routing.fallback_strategy` (`fill-first` | `round-robin`, default `fill-first`) is now the global default and `routing.provider_strategies` overrides it per provider id (whole-map writes, absent entry inherits). `fill-first` serves priority order and starts every request from the first usable endpoint, picking the first healthy key by priority; `round-robin` keeps the sticky endpoint cursor and rotates keys least-recently-used first. The policy is consulted per selection, so it governs every request through a provider and not only combo members, and a policy that cannot be read degrades to `fill-first`, the same priority-order degradation the cursor already had. The environment knob is removed: the document is the single source, and it was never declared in `.env.example`.*

*Changelog 2026-09-24: §7.15 adds `POST /api/v1/systemone`, the System One (Jev) decision route, and §7.15's data-plane table records it (draft `029-OPENCODE-PROVIDER-PARITY.md` F6). The reference serves a decision model as `kind: "systemone"` on a native endpoint rather than as a chat model (`registry/opencode.js:31`, `systemoneConfig` on the entry, `src/sse/handlers/systemone.js`, `open-sse/handlers/systemoneCore.js`), because the payload is the provider's own vocabulary: `state` plus a `questions` map, forwarded untouched, with no chat translation layer. Measured before the change: this port had no such route, and `opencode/jev-1.13-free` resolved onto the chat wire and was served as a chat completion, so a chat body was sent to an endpoint that answers a decision payload. The route is gateway-key authenticated like every §7.15 route and follows the media plane's shape rather than the chat plane's: one call, no combo chain, one usage row and one log row under the router's request id. Its payload is validated at the boundary (`state` present, `questions` a non-empty object, the model naming a provider that declares a `systemone` endpoint) and forwarded verbatim, because the reference forwards it verbatim and a gateway that reshaped it would answer a different question than the one it was asked. The identity headers the reference sends with it (`x-opencode-client`, `User-Agent`) travel with the entry's own block, and the session header the Zen lanes require is written per call, which is what the reference's `generateSessionId()` does there. A model declaring `kind: systemone` is no longer routable through the chat plane: the resolver refuses it by name with `MODEL_NOT_FOUND` and the kind in the message, so the wrong-body failure cannot come back. What is deliberately not ported: the reference's credential-fallback loop for this route, because the free lane needs no credential and a keyed decision model is served by the same selector the media plane uses, which already walks the provider's accounts.*

