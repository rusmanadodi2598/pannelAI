-- P1 schema: request logs, the console ring buffer's durable sibling, and
-- typed settings (SPEC-API-001 §6, §7.13, §7.14).
-- Bodies are stored only when capture is enabled in settings.logging; the
-- retention job deletes rows older than retention_days.

CREATE TABLE IF NOT EXISTS request_logs (
    request_id    text        PRIMARY KEY,
    ts            timestamptz NOT NULL,
    gateway_key_id text,
    endpoint_id   text,
    provider_id   text,
    model         text,
    status        text        NOT NULL,
    latency_ms    integer     NOT NULL DEFAULT 0,
    request_body  text,
    response_body text,
    error         text
);

CREATE INDEX IF NOT EXISTS idx_request_logs_ts ON request_logs (ts DESC);
CREATE INDEX IF NOT EXISTS idx_request_logs_status ON request_logs (status, ts DESC);
CREATE INDEX IF NOT EXISTS idx_request_logs_endpoint ON request_logs (endpoint_id, ts DESC);
CREATE INDEX IF NOT EXISTS idx_request_logs_model ON request_logs (model, ts DESC);

-- One row per settings key, value typed jsonb, defaults merged at read
-- (SPEC-API-001 §7.14). A single row of one wide jsonb document was the
-- alternative; separate keys make a partial PATCH a single-row upsert and stop
-- two writers from clobbering each other's subtree.
CREATE TABLE IF NOT EXISTS settings (
    key        text        PRIMARY KEY,
    value      jsonb       NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);