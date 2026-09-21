---
name: pannelai-image
description: Text-to-image through pannelAI. Covers POST /api/v1/images/generations, the provider/model string, the prompt and size fields, and the normalized OpenAI images answer. Use when a client needs a generated image from the gateway.
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

## Video is registered but not served

`POST /api/v1/videos/generations` exists and answers `PROVIDER_NOT_ROUTABLE`. No provider in the registry
declares video support yet, so the route refuses by name instead of dialing an upstream with a payload it
cannot interpret. Do not build a client against it until the operator's registry says otherwise.

## Failures

`UNAUTHORIZED` (401) when the key is missing or revoked. `MODEL_NOT_FOUND` (404) for a bare model id or a
provider that is not in the registry. `PROVIDER_NOT_ROUTABLE` (400) when the provider declares no image
support, or declares a format this gateway has no adapter for. `VALIDATION_ERROR` (400) for a missing
`prompt`, or for a provider that needs a model and declares no default. `NO_PROVIDER_AVAILABLE` (503) when
every endpoint for that provider is unhealthy or out of budget. `UPSTREAM_ERROR` (502) and
`UPSTREAM_TIMEOUT` (504) when the provider itself refused or stalled.
