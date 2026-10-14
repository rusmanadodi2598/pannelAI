-- Reverse of 000019: the selection timestamp and the per-class runs go.

-- Dropping them costs no credential and no usage row. The router returns to
-- ordering keys by the last time one SUCCEEDED, which is the behaviour that made
-- a burst land on one key, and every ladder step returns to the shared total that
-- is still stored. A rollback therefore loses precision, not availability.

ALTER TABLE upstream_keys
    DROP COLUMN IF EXISTS last_error_class,
    DROP COLUMN IF EXISTS consecutive_billing,
    DROP COLUMN IF EXISTS consecutive_ratelimit,
    DROP COLUMN IF EXISTS consecutive_auth,
    DROP COLUMN IF EXISTS last_selected_at;
