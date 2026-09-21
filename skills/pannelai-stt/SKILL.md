---
name: pannelai-stt
description: Speech-to-text through pannelAI. Covers POST /api/v1/audio/transcriptions as a multipart upload, the model and language fields, and the verbatim upstream answer. Use when a client needs an audio file transcribed by the gateway.
---

# pannelAI: Speech-to-Text

One route: `POST /api/v1/audio/transcriptions`, a multipart upload, gateway-key auth.

```bash
curl -s $PANNELAI_URL/api/v1/audio/transcriptions \
  -H "Authorization: Bearer $PANNELAI_KEY" \
  -F "file=@clip.mp3" \
  -F "model=deepgram/nova-3" \
  -F "language=id"
```

The upload is multipart, not JSON, so `-F` rather than `-d`. The file part carries the audio bytes and
their content type.

## Fields

| Field | Required | Notes |
|---|---|---|
| `file` | yes | The audio. |
| `model` | yes | `provider/model`. The provider must declare STT support. |
| `language` | no | The spoken language, as a code. |
| `prompt` | no | Context to bias the transcription. |
| `response_format` | no | The shape to ask for, where the provider supports more than one. |
| `temperature` | no | Decoding temperature, where the provider supports it. |

`language`, `prompt`, `response_format`, and `temperature` are forwarded to OpenAI-compatible providers,
which is where the reference forwards them. A provider-specific adapter reads the subset it declares, so
a field the provider ignores is not an error.

## The answer

The gateway forwards the upstream's answer verbatim rather than re-encoding it, because re-encoding would
drop whichever fields the caller asked for with `response_format`:

- a JSON answer arrives as `application/json`, usually `{"text": "..."}`
- anything else arrives as `text/plain`

A client should therefore read the content type before parsing, and treat a plain-text body as the
transcript itself.

## Provider notes

The gateway normalizes each provider's own answer to the same `{"text": "..."}` shape where the adapter
has to reshape it, so a client reads one field whichever provider served it. A provider whose wire format
has no adapter answers `PROVIDER_NOT_ROUTABLE` (400) naming the format. That is an honest gap rather than
an outage: report it to the operator instead of retrying.

The gateway authenticates the key before it reads any body, so a large upload is never parsed, or spilled
to disk, for a caller without a key. A caller that sends a 25 MB file with a bad key gets a 401 without
the upload being read.

## Failures

`UNAUTHORIZED` (401). `MODEL_NOT_FOUND` (404) for a bare model id or a provider that is not in the
registry. `PROVIDER_NOT_ROUTABLE` (400) when the provider declares no STT support, or declares a format
this gateway has no adapter for. `VALIDATION_ERROR` (400) for a missing file or model, or a provider that
needs a model and declares no default. `NO_PROVIDER_AVAILABLE` (503) when no endpoint can serve the
provider. `UPSTREAM_ERROR` (502) and `UPSTREAM_TIMEOUT` (504) when the provider itself refused or stalled.
