---
name: pannelai
description: Entry point for pannelAI, a self-hosted AI gateway that speaks OpenAI, Anthropic, and OpenAI Responses wire formats for chat, image generation, text-to-speech, speech-to-text, embeddings, and web search. Use when the user mentions pannelAI, a gateway base URL, or wants one key in front of several providers. Covers setup, gateway-key auth, model discovery, and links to one skill per capability.
---

# pannelAI

A self-hosted gateway in front of the providers an operator configured. A client holds one gateway key
and calls one base URL; the gateway resolves the model, picks a healthy upstream endpoint, translates the
wire format, and records usage, quota, and a request log row.

## Setup

```bash
export PANNELAI_URL="http://localhost:8080"   # the gateway's own address, no /api/v1 suffix here
export PANNELAI_KEY="sk-..."                  # created once on the panel's Endpoint & Key screen
```

Every data-plane call is `${PANNELAI_URL}/api/v1/...` with `Authorization: Bearer ${PANNELAI_KEY}`.

Check the gateway is up, and what build it is:

```bash
curl -s $PANNELAI_URL/api/v1/health      # liveness plus database and Redis reachability, no key needed
curl -s $PANNELAI_URL/api/v1/version     # build version, commit, registry revision, no key needed
```

`GET /api/v1/health` answers `{"status":"ok","checks":{...},"version":...,"commit":...,"timestamp":...}`
with HTTP 200, or `"status":"degraded"` with HTTP 503 when a dependency did not answer. If it is not `ok`,
nothing below it will work, and the fault is the gateway's own infrastructure rather than a provider.

## Discover models

```bash
curl -s -H "Authorization: Bearer $PANNELAI_KEY" $PANNELAI_URL/api/v1/models
```

The answer is the OpenAI list shape, `{"object":"list","data":[...]}`, holding every model the gateway
can route right now plus every combo the operator built. A combo carries `owned_by: "combo"`. Use
`data[].id` as the `model` field of a request.

A `model` string resolves in this order, and the first match wins:

1. a combo name,
2. an alias,
3. `provider/model`, split on the first slash so a model id that itself contains a slash
   (`openrouter/openai/gpt-4o-mini-tts`) still resolves to its provider,
4. otherwise the call is refused with `MODEL_NOT_FOUND`.

## Capability skills

Each capability has its own document. Fetch the one you need:

| Capability | Data-plane route | Skill |
|---|---|---|
| Chat and code generation, streamed or not | `POST /api/v1/chat/completions`, `/messages`, `/responses` | `https://raw.githubusercontent.com/rusmanadodi2598/pannelAI/refs/heads/main/skills/pannelai-chat/SKILL.md` |
| Image generation | `POST /api/v1/images/generations` | `https://raw.githubusercontent.com/rusmanadodi2598/pannelAI/refs/heads/main/skills/pannelai-image/SKILL.md` |
| Text-to-speech | `POST /api/v1/audio/speech` | `https://raw.githubusercontent.com/rusmanadodi2598/pannelAI/refs/heads/main/skills/pannelai-tts/SKILL.md` |
| Speech-to-text | `POST /api/v1/audio/transcriptions` | `https://raw.githubusercontent.com/rusmanadodi2598/pannelAI/refs/heads/main/skills/pannelai-stt/SKILL.md` |
| Embeddings | `POST /api/v1/embeddings` | `https://raw.githubusercontent.com/rusmanadodi2598/pannelAI/refs/heads/main/skills/pannelai-embeddings/SKILL.md` |
| Web search | `POST /api/v1/search` | `https://raw.githubusercontent.com/rusmanadodi2598/pannelAI/refs/heads/main/skills/pannelai-web-search/SKILL.md` |

## Failures

A data-plane failure answers with an HTTP status and a JSON body carrying `error.code`, `error.message`,
and `error.type`, all in English:

```json
{ "error": { "message": "gateway key is missing or invalid", "type": "invalid_request_error", "code": "UNAUTHORIZED" } }
```

The codes a client should branch on:

| Code | HTTP | What to do |
|---|---|---|
| `UNAUTHORIZED` | 401 | The gateway key is missing, revoked, or wrong. Ask the operator for a new one. |
| `MODEL_NOT_FOUND` | 404 | The `model` string matched no combo, alias, or provider. Re-read `/models`. |
| `VALIDATION_ERROR` | 400 | The body did not match the route's contract. |
| `RATE_LIMITED` | 429 | This key is over its limit, or the gateway's login limiter tripped. Back off and retry. |
| `NO_PROVIDER_AVAILABLE` | 503 | Every endpoint and key for that model is unhealthy or out of quota. Try another model, or tell the operator. |
| `PROVIDER_NOT_ROUTABLE` | 400 | The provider is registered but the gateway has no adapter for its wire format. Try another provider. |
| `UPSTREAM_ERROR` | 502 | The provider refused or failed. Retrying may help. |
| `UPSTREAM_TIMEOUT` | 504 | The provider did not answer inside the gateway's timeout. |
| `INTERNAL_ERROR` | 500 | A gateway fault. The gateway logs it with a request id, so quote the time and route when reporting it. |

A retry is worth attempting on `429`, `502`, and `504` only. On `400`, `401`, `404`, and `503` the same
request will fail the same way until something changes, so retrying spends the caller's time and the
operator's quota for nothing.
