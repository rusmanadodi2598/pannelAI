-- P1 parity: the five connection fields a reference "connection" carries and an
-- endpoint did not (draft 017 §4.1b, SPEC-API-001 §7.5).
--
-- The endpoint stays the mutation boundary and the connection model is not
-- imported: the reference keeps one row per credential, while this port keeps one
-- endpoint with 1..N keys (§7.5). What is added here is the capability, not the
-- shape — the five fields an operator needs to express, on an endpoint, what a
-- reference connection can express.
--
--   global_priority        order across providers, for the case where several
--                          providers each have a healthy account
--   default_model          the model a request uses when it names none
--   consecutive_use_count  the current run of served calls; a rotation input
--   last_error / _at / _code  the last failure that was NOT a connectivity test,
--                          so a rejection during routing is visible on the screen
--   proxy_pool_id          the stored proxy pool this endpoint egresses through
--
-- Two columns carry rules rather than data, and both are enforced above the
-- database: `last_error` is scrubbed of credential material before it is stored
-- (domain.RecordUpstreamError, OWASP A09), and `proxy_pool_id` is validated
-- against the proxies table by the service, so a dangling id is refused rather
-- than stored.
--
-- Every statement is idempotent: Apply runs the whole file on every boot and the
-- schema ledger only records it once, but a partially applied file must be
-- re-runnable (migrations/apply_test.go's TestApply_IsIdempotent).

ALTER TABLE upstream_endpoints
    ADD COLUMN IF NOT EXISTS global_priority       integer,
    ADD COLUMN IF NOT EXISTS default_model         text        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS consecutive_use_count integer     NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_error            text,
    ADD COLUMN IF NOT EXISTS last_error_at         timestamptz,
    ADD COLUMN IF NOT EXISTS error_code            text,
    ADD COLUMN IF NOT EXISTS proxy_pool_id         text;

-- The order across providers is a sort key the router reads, and the proxy
-- binding is a lookup, so both get an index (AGENTS.md §1.7: a new indexed
-- lookup column ships with its index).
CREATE INDEX IF NOT EXISTS idx_upstream_endpoints_global_priority
    ON upstream_endpoints (global_priority);
CREATE INDEX IF NOT EXISTS idx_upstream_endpoints_proxy_pool
    ON upstream_endpoints (proxy_pool_id);

-- The counter is a run length, so a negative value is a bug rather than data.
-- It is added through a catalog check so re-running the file is safe.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'upstream_endpoints_consecutive_use_check'
    ) THEN
        ALTER TABLE upstream_endpoints
            ADD CONSTRAINT upstream_endpoints_consecutive_use_check
            CHECK (consecutive_use_count >= 0);
    END IF;
END
$$;
