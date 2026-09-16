-- P0 schema: gateway keys (SPEC-API-001 §6)
-- A gateway key is stored only as a SHA-256 digest plus a masking hint; the
-- plaintext is shown to the caller exactly once at creation time (§4).

CREATE TABLE IF NOT EXISTS gateway_keys (
    id            text       PRIMARY KEY,
    name          text       NOT NULL,
    value_hash    text       NOT NULL,
    key_hint      text       NOT NULL,
    status        text       NOT NULL DEFAULT 'active',
    last_used_at  timestamptz,
    request_count bigint     NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    revoked_at    timestamptz
);

-- Lookups by digest (the presented secret is hashed and compared) and by name.
CREATE INDEX IF NOT EXISTS idx_gateway_keys_value_hash ON gateway_keys (value_hash);
CREATE INDEX IF NOT EXISTS idx_gateway_keys_name ON gateway_keys (name);
