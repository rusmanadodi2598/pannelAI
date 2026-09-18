-- P1 schema: upstream endpoints and their keys (SPEC-API-001 §6, §7.5).
-- An upstream endpoint is one configured account at a provider. Routing picks
-- the endpoint by priority, then a healthy key inside it. Multi-key per
-- endpoint is the capability the reference lacks (§11.2), and it is what makes
-- a provider multi-account without a second table.

CREATE TABLE IF NOT EXISTS upstream_endpoints (
    id                  text        PRIMARY KEY,
    provider_id         text        NOT NULL,
    label               text        NOT NULL,
    auth_type           text        NOT NULL,
    priority            integer     NOT NULL DEFAULT 1,
    status              text        NOT NULL DEFAULT 'active',
    oauth               jsonb,
    account             jsonb       NOT NULL DEFAULT '{}'::jsonb,
    test_status         jsonb,
    rate_limited_until  timestamptz,
    last_used_at        timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT upstream_endpoints_auth_check
        CHECK (auth_type IN ('api_key', 'oauth', 'no_auth')),
    CONSTRAINT upstream_endpoints_status_check
        CHECK (status IN ('active', 'disabled', 'error'))
);

-- Priority is scoped per provider: the router orders candidates within one
-- provider, and two providers may both start at 1.
CREATE UNIQUE INDEX IF NOT EXISTS idx_upstream_endpoints_provider_label
    ON upstream_endpoints (provider_id, label);
CREATE INDEX IF NOT EXISTS idx_upstream_endpoints_provider_priority
    ON upstream_endpoints (provider_id, priority);
CREATE INDEX IF NOT EXISTS idx_upstream_endpoints_status
    ON upstream_endpoints (status);

-- One API key credential under an endpoint. The value is stored as AES-GCM
-- ciphertext (ENCRYPTION_KEY); key_hint is the only form ever read back.
CREATE TABLE IF NOT EXISTS upstream_keys (
    id                  text        PRIMARY KEY,
    endpoint_id         text        NOT NULL REFERENCES upstream_endpoints (id) ON DELETE CASCADE,
    label               text        NOT NULL,
    value_encrypted     text        NOT NULL,
    key_hint            text        NOT NULL,
    priority            integer     NOT NULL DEFAULT 1,
    status              text        NOT NULL DEFAULT 'active',
    last_used_at        timestamptz,
    last_error          text,
    consecutive_errors  integer     NOT NULL DEFAULT 0,
    rate_limited_until  timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT upstream_keys_status_check
        CHECK (status IN ('active', 'disabled', 'error'))
);

-- Selection reads the healthy keys of one endpoint in priority order; the
-- endpoint_id prefix makes that a single index range scan.
CREATE INDEX IF NOT EXISTS idx_upstream_keys_endpoint_priority
    ON upstream_keys (endpoint_id, priority);
CREATE INDEX IF NOT EXISTS idx_upstream_keys_status ON upstream_keys (status);