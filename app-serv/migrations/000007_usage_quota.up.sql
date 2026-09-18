-- P1 schema: usage records and quota windows (SPEC-API-001 §6, §7.12).
-- Indexed by ts because every read is a range over a time window, and by
-- (provider_id, ts) / (endpoint_id, ts) because the group-by dimensions the
-- summary endpoint offers are exactly those two.

CREATE TABLE IF NOT EXISTS usage_records (
    id                  text        PRIMARY KEY,
    request_id          text        NOT NULL,
    ts                  timestamptz NOT NULL,
    endpoint_id         text,
    provider_id         text        NOT NULL,
    gateway_key_id      text,
    model               text        NOT NULL,
    combo               text,
    tokens_in           bigint      NOT NULL DEFAULT 0,
    tokens_out          bigint      NOT NULL DEFAULT 0,
    tokens_cache_read   bigint      NOT NULL DEFAULT 0,
    tokens_cache_write  bigint      NOT NULL DEFAULT 0,
    cost_usd            numeric(20, 8) NOT NULL DEFAULT 0,
    latency_ms          integer     NOT NULL DEFAULT 0,
    status              text        NOT NULL,
    error_code          text
);

CREATE INDEX IF NOT EXISTS idx_usage_records_ts ON usage_records (ts DESC);
CREATE INDEX IF NOT EXISTS idx_usage_records_provider_ts ON usage_records (provider_id, ts DESC);
CREATE INDEX IF NOT EXISTS idx_usage_records_endpoint_ts ON usage_records (endpoint_id, ts DESC);
CREATE INDEX IF NOT EXISTS idx_usage_records_model_ts ON usage_records (model, ts DESC);
CREATE INDEX IF NOT EXISTS idx_usage_records_gateway_key_ts ON usage_records (gateway_key_id, ts DESC);
CREATE INDEX IF NOT EXISTS idx_usage_records_request ON usage_records (request_id);

-- Quota counters are cached in Redis and flushed here; the row is the durable
-- record, not the hot path. `source` separates a number this gateway counted
-- from one the provider reported.
-- `window` is a reserved word in PostgreSQL, so the column is quoted
-- everywhere it appears. Quoting it keeps the API's own field name
-- (SPEC-API-001 §7.12 returns `window`) instead of renaming the column to suit
-- the parser and then mapping it back in every query.
CREATE TABLE IF NOT EXISTS quota_windows (
    endpoint_id text        NOT NULL,
    "window"    text        NOT NULL,
    used_units  bigint      NOT NULL DEFAULT 0,
    limit_units bigint,
    resets_at   timestamptz,
    source      text        NOT NULL DEFAULT 'computed',
    updated_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (endpoint_id, "window"),
    CONSTRAINT quota_windows_window_check
        CHECK ("window" IN ('5h', 'daily', 'weekly', 'monthly')),
    CONSTRAINT quota_windows_source_check
        CHECK (source IN ('computed', 'reported'))
);

-- Budget caps live on the endpoint; the router skips an exhausted endpoint.
CREATE TABLE IF NOT EXISTS quota_caps (
    endpoint_id       text        PRIMARY KEY,
    monthly_cost_usd  numeric(20, 8),
    monthly_tokens    bigint,
    updated_at        timestamptz NOT NULL DEFAULT now()
);