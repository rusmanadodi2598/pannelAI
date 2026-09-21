---
name: pannelai-web-search
description: Web search through pannelAI. Covers POST /api/v1/search, how the provider is named, the query and max_results fields, and the normalized results envelope with its cost and timing. Use when a client needs live web results from the gateway.
---

# pannelAI: Web Search

One route: `POST /api/v1/search`, gateway-key auth.

```bash
curl -s $PANNELAI_URL/api/v1/search \
  -H "Authorization: Bearer $PANNELAI_KEY" \
  -H "content-type: application/json" \
  -d '{"provider": "brave-search", "query": "golang generics", "max_results": 5}'
```

## Naming the provider

Search is the one media kind that does not need a model: its providers read a query and a result count,
so there is no `provider/model` string to resolve. Name the provider one of two ways:

- `provider` names it directly, which is the shorter path.
- `model` names it through its first segment, `provider/anything`.

Neither field present answers `VALIDATION_ERROR` with "provider (or model) is required". A provider that
is not in the registry answers `MODEL_NOT_FOUND` (404), and one that declares no search support answers
`PROVIDER_NOT_ROUTABLE` (400).

## Fields

| Field | Required | Notes |
|---|---|---|
| `query` | yes | 1 to 400 characters. |
| `provider` | no | The registry id of a search-capable provider. |
| `model` | no | Alternative to `provider`; its first segment names the provider. |
| `max_results` | no | A positive whole number. |
| `search_type`, `country`, `language` | no | Forwarded to providers that declare them. |

## The answer

```json
{
  "provider": "brave-search",
  "query": "golang generics",
  "results": [ { "title": "...", "url": "https://...", "snippet": "...", "position": 1 } ],
  "usage": { "queries_used": 1, "search_cost_usd": 0.003 },
  "metrics": { "response_time_ms": 412, "total_results_available": 5 }
}
```

The envelope is the same whichever provider served the call, because the gateway builds each provider's
own payload from the parameters its registry block declares. Search APIs disagree on names (`q` against
`query`, `num` against `count`), and that disagreement is the gateway's problem rather than the client's.

`usage.search_cost_usd` is the provider's declared cost per query, and `metrics.response_time_ms` is the
gateway's own measurement, so both are facts about the call that just happened rather than estimates.

## Failures

`UNAUTHORIZED` (401). `VALIDATION_ERROR` (400) for a missing `query`, a query over 400 characters, or
neither `provider` nor `model`. `MODEL_NOT_FOUND` (404) for a provider that is not in the registry.
`PROVIDER_NOT_ROUTABLE` (400) when the provider declares no search support or a format this gateway has no
adapter for. `NO_PROVIDER_AVAILABLE` (503) when no endpoint can serve the provider. `UPSTREAM_ERROR`
(502) and `UPSTREAM_TIMEOUT` (504) when the provider itself refused or stalled.
