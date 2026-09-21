---
name: pannelai-tts
description: Text-to-speech through pannelai. Covers POST /api/v1/audio/speech, the voice catalog at GET /api/v1/audio/voices, the bytes-versus-base64 response choice, and the per-provider adapters. Use when a client needs spoken audio from the gateway.
---

# pannelAI: Text-to-Speech

```bash
curl -s $PANNELAI_URL/api/v1/audio/speech \
  -H "Authorization: Bearer $PANNELAI_KEY" \
  -H "content-type: application/json" \
  -d '{"model": "openai/gpt-4o-mini-tts", "input": "Ready.", "voice": "alloy"}' \
  --output speech.mp3
```

The answer is audio bytes with an `audio/...` content type, so a client writes the body to a file rather
than parsing it as JSON.

## Fields

| Field | Required | Notes |
|---|---|---|
| `model` | yes | `provider/model`. The provider must declare TTS support. |
| `input` | yes | The text to speak. |
| `voice` | yes | A voice id from the provider's catalog. |
| `response_format` | no | The audio format to ask for. |
| `speed`, `language` | no | Passed through to providers that accept them. |

## Ask for base64 instead of bytes

Add `?response_format=json` to the query string and the same audio comes back as JSON:

```json
{ "audio": "<base64>", "format": "mp3" }
```

Use this when the client cannot write a binary body, for example inside a tool call that has to return
text. Without the query parameter the response is raw bytes.

## The voice catalog

```bash
curl -s "$PANNELAI_URL/api/v1/audio/voices?provider=openai" \
  -H "Authorization: Bearer $PANNELAI_KEY"
```

`provider` is required: the catalog is per provider, and listing every provider's would answer a question
the client did not ask. The answer is the OpenAI list shape, `{"object":"list","data":[...]}`, with each
voice carrying `id`, and `name`, `lang`, and `gender` where the registry declares them. A provider that
declares no voice catalog answers `PROVIDER_NOT_ROUTABLE` (400) naming that fact, rather than an empty
list, so an empty list never has to be interpreted.

Reading the catalog makes no upstream call, so it costs no quota. Use it to discover a valid `voice`
rather than guessing one.

## Provider notes

The gateway sends each provider its own request shape and normalizes the answer back to bytes or base64,
so a client does not branch per provider. Two consequences worth knowing:

- Some providers return a fixed output format, and asking for another through `response_format` is refused
  rather than silently converted.
- A provider whose wire format has no adapter answers `PROVIDER_NOT_ROUTABLE` (400) naming the format,
  which is an honest gap rather than an outage. Tell the operator; do not retry.

The `voice` field is forwarded, not checked against the catalog here, so a voice the provider does not
know comes back as `UPSTREAM_ERROR` (502) rather than a validation refusal.

## Failures

`UNAUTHORIZED` (401). `MODEL_NOT_FOUND` (404) for a bare model id or a provider that is not in the
registry. `PROVIDER_NOT_ROUTABLE` (400) when the provider declares no TTS support, declares a format this
gateway has no adapter for, or declares no voice catalog. `VALIDATION_ERROR` (400) for a missing field, or
a provider that needs a model and declares no default. `NO_PROVIDER_AVAILABLE` (503) when no endpoint can
serve the provider. `UPSTREAM_ERROR` (502) and `UPSTREAM_TIMEOUT` (504) when the provider itself refused
or stalled.
