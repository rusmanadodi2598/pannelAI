---
name: pannelai-embeddings
description: Embeddings through pannelAI. Covers POST /api/v1/embeddings, the provider/model string, the input field, and the OpenAI embeddings answer with its usage block. Use when a client needs vectors for RAG or semantic search from the gateway.
---

# pannelAI: Embeddings

One route: `POST /api/v1/embeddings`, gateway-key auth, OpenAI embeddings shape.

```bash
curl -s $PANNELAI_URL/api/v1/embeddings \
  -H "Authorization: Bearer $PANNELAI_KEY" \
  -H "content-type: application/json" \
  -d '{"model": "openai/text-embedding-3-small", "input": ["first document", "second document"]}'
```

## Fields

| Field | Required | Notes |
|---|---|---|
| `model` | yes | `provider/model`. The provider must declare embedding support. |
| `input` | yes | One string, or an array of strings. |
| `encoding_format` | no | `float` or `base64`, where the provider supports the choice. |
| `dimensions` | no | Ask for a shorter vector, where the model supports it. |
| `user` | no | An identifier passed through for the provider's own accounting. |

## The answer

```json
{
  "object": "list",
  "data": [ { "object": "embedding", "index": 0, "embedding": [0.01, -0.02] } ],
  "model": "text-embedding-3-small",
  "usage": { "prompt_tokens": 4, "total_tokens": 4 }
}
```

`data` is ordered to match the `input` array, so `data[i].embedding` belongs to `input[i]`. Read the
vector length from the first element rather than assuming it: it depends on the model the operator
configured.

## Model strings

The embeddings plane resolves `model` the same way chat does, so `provider/model` and an alias both work.
A combo does not: a combo is an ordered list for chat failover, and honouring its first member would embed
with a model the client did not name, so the call is refused with `VALIDATION_ERROR` rather than silently
dereferenced.

A model that exists but belongs to a provider without embedding support is refused by name, which is why
`GET /api/v1/models` is worth reading first: it lists what the gateway can actually route.

## Failures

`UNAUTHORIZED` (401), `MODEL_NOT_FOUND` (404) when nothing matches the model string,
`VALIDATION_ERROR` (400) for a combo, an unknown provider, or a provider without embedding support,
`NO_PROVIDER_AVAILABLE` (503) when every endpoint for that provider is unhealthy or out of budget,
`UPSTREAM_ERROR` (502), `UPSTREAM_TIMEOUT` (504).
