---
name: pannelai-chat
description: Chat and code generation through pannelAI, in the OpenAI chat completions, Anthropic messages, or OpenAI Responses wire format, streamed or not. Covers the model field, streaming, combo routing, and the vision adapter. Use when a client needs an LLM answer from the gateway.
---

# pannelAI: Chat

Three routes, one capability. Pick the route that matches the wire format the client already speaks;
the gateway translates to whatever the resolved provider expects.

| Route | Wire format |
|---|---|
| `POST /api/v1/chat/completions` | OpenAI chat completions |
| `POST /api/v1/messages` | Anthropic messages |
| `POST /api/v1/responses` | OpenAI Responses |

All three take `Authorization: Bearer $PANNELAI_KEY` and a `model` field resolved as described in the
entry skill (combo name, alias, or `provider/model`).

## OpenAI format

```bash
curl -s $PANNELAI_URL/api/v1/chat/completions \
  -H "Authorization: Bearer $PANNELAI_KEY" \
  -H "content-type: application/json" \
  -d '{
    "model": "openai/gpt-4o-mini",
    "messages": [{"role": "user", "content": "Reply with one word: ready"}]
  }'
```

The answer is the OpenAI chat completion shape: `choices[0].message.content`, with a `usage` block
carrying `prompt_tokens`, `completion_tokens`, and `total_tokens` when the provider reported accounting.

Add `"stream": true` for server-sent events. The gateway passes the upstream's SSE frames through in the
client's own format, so a client parses `data:` lines exactly as it would against the provider directly.
The stream ends with `data: [DONE]`, which is the marker a client waits for to release the connection. A
streamed answer carries its token accounting in a final frame before that marker when the provider
reported it, so a client that wants usage from a stream must read to the end rather than stopping at the
first content frame.

## Anthropic format

```bash
curl -s $PANNELAI_URL/api/v1/messages \
  -H "Authorization: Bearer $PANNELAI_KEY" \
  -H "content-type: application/json" \
  -d '{
    "model": "claude/sonnet",
    "max_tokens": 256,
    "messages": [{"role": "user", "content": "Reply with one word: ready"}]
  }'
```

The answer carries `content[0].text` and `usage.input_tokens` / `usage.output_tokens`, the Anthropic
shape. `POST /api/v1/messages/count_tokens` counts a request's tokens without sending it upstream.

## Routing the model chooses

- **Combos.** A `model` naming a combo makes the gateway pick the member. A `fallback` combo tries its
  members in priority order until one answers; `round_robin` rotates between them; `fusion` asks several
  and has a judge model write the final answer. A client sends one request either way and does not need to
  know which member served it.
  Failover happens only for a failure another member could plausibly survive: `UPSTREAM_ERROR`,
  `UPSTREAM_TIMEOUT`, `RATE_LIMITED`, and `NO_PROVIDER_AVAILABLE`. A `VALIDATION_ERROR` or
  `MODEL_NOT_FOUND` stops at the first member, because every other member would reject the same body.
  When every member has failed, the answer carries the last member's error, so the code names what
  actually went wrong rather than a generic one.
- **Vision adapter.** When a request carries image content and the resolved model cannot read images, the
  gateway prepends the operator's configured vision models to the member order and the first one that
  answers handles the image. The answer's identity is reported as the model the client asked for, not the
  adapter that served it, so a client sends the same body it would send to a vision-capable model and does
  not need to know an adapter ran.

## Failures

Branch on `error.code`: `UNAUTHORIZED` (401), `MODEL_NOT_FOUND` (404), `VALIDATION_ERROR` (400),
`RATE_LIMITED` (429), `NO_PROVIDER_AVAILABLE` (503), `PROVIDER_NOT_ROUTABLE` (400), `UPSTREAM_ERROR`
(502), `UPSTREAM_TIMEOUT` (504), `INTERNAL_ERROR` (500). The entry skill lists what each one means and
which are worth a retry.
