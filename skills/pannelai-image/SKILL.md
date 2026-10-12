---
name: pannelai-image
description: Text-to-image and async video generation through pannelAI. Covers POST /api/v1/images/generations, the provider/model string, the prompt and size fields, the normalized OpenAI images answer, and the two video job routes. Use when a client needs a generated image or video from the gateway.
---

# pannelAI: Image Generation

One route: `POST /api/v1/images/generations`, gateway-key auth, OpenAI images shape.

```bash
curl -s $PANNELAI_URL/api/v1/images/generations \
  -H "Authorization: Bearer $PANNELAI_KEY" \
  -H "content-type: application/json" \
  -d '{
    "model": "openai/gpt-image-1",
    "prompt": "A square logo mark, two flat coral shapes, no text",
    "size": "1024x1024"
  }'
```

## The model string

`model` is `provider/model`, and the provider must declare image support. Unlike the chat plane, the
media plane has no aliases and no combos: the route addresses a provider's own service, so a bare model id
answers `MODEL_NOT_FOUND` rather than being guessed at. The model is optional; leaving it out uses the
provider's configured default, and a provider that declares no default needs the model spelled out.

## Fields

| Field | Required | Notes |
|---|---|---|
| `prompt` | yes | The generation instruction. |
| `model` | no | `provider/model`. Falls back to the provider's configured default. |
| `n` | no | Number of images. |
| `size` | no | For example `1024x1024`. |
| `response_format` | no | Ask for a URL or for base64 inline data. |
| `quality`, `style` | no | Passed through to providers that accept them. |

## The answer

The gateway normalizes every provider's answer to the OpenAI images shape:

```json
{ "created": 1758400000, "data": [ { "url": "https://..." } ] }
```

A provider that returns inline bytes instead of a URL answers `data[0].b64_json` in the same envelope, so
a client reads one shape whichever provider served it.

## Video is an async job

A video generation outlives one request, so it is two routes rather than one. `POST
/api/v1/videos/generations` creates the job and answers it, and `GET /api/v1/videos/{task_id}?provider=<id>`
reads it. The creation body is read for `model` and `prompt` only; every other member travels to the
provider as written, so a provider's own knobs (a duration, a resolution) reach it.

```bash
curl -s $PANNELAI_URL/api/v1/videos/generations \
  -H "Authorization: Bearer $PANNELAI_KEY" \
  -H "content-type: application/json" \
  -d '{"model": "xai/grok-imagine-video", "prompt": "A paper boat on a puddle", "duration": 8}'
```

```json
{ "task_id": "req-1", "status": "queued" }
```

The `X-PannelAI-Endpoint-Id` response header names the account the job was created on; send it back as
`?endpoint=` so the polls reach that same account. `provider` is required on the poll, because a poll
carries no model to resolve it from.

`status` is `queued`, `in_progress`, `succeeded`, or `failed`. `url` carries the asset once the job
succeeded and `error` the provider's own message once it failed. Poll until one of those two terminal
states: a state the gateway has no mapping for is reported as the provider spelled it, never as a finished
job.

## Failures

`UNAUTHORIZED` (401) when the key is missing or revoked. `MODEL_NOT_FOUND` (404) for a bare model id or a
provider that is not in the registry. `PROVIDER_NOT_ROUTABLE` (400) when the provider declares no image
support, or declares a format this gateway has no adapter for. `VALIDATION_ERROR` (400) for a missing
`prompt`, or for a provider that needs a model and declares no default. `NO_PROVIDER_AVAILABLE` (503) when
every endpoint for that provider is unhealthy or out of budget. `UPSTREAM_ERROR` (502) and
`UPSTREAM_TIMEOUT` (504) when the provider itself refused or stalled.
