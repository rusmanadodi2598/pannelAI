-- Rollback for 000012: drop the connection-parity columns and their indexes.
--
-- The order reverses the up file: the constraint first, then the two indexes,
-- then the columns. Dropping the columns would take the constraint with them, but
-- naming each step keeps the two files readable side by side and makes a partial
-- rollback possible when only one index is unwanted.
--
-- Dropping the columns discards whatever routing order, default models, proxy
-- bindings, use counts, and last errors the deployment had recorded. That is the
-- intent of a rollback — the fields do not exist in the prior schema — and it is
-- stated here so the data loss is a decision rather than a surprise.

ALTER TABLE upstream_endpoints
    DROP CONSTRAINT IF EXISTS upstream_endpoints_consecutive_use_check;

DROP INDEX IF EXISTS idx_upstream_endpoints_proxy_pool;
DROP INDEX IF EXISTS idx_upstream_endpoints_global_priority;

ALTER TABLE upstream_endpoints
    DROP COLUMN IF EXISTS proxy_pool_id,
    DROP COLUMN IF EXISTS error_code,
    DROP COLUMN IF EXISTS last_error_at,
    DROP COLUMN IF EXISTS last_error,
    DROP COLUMN IF EXISTS consecutive_use_count,
    DROP COLUMN IF EXISTS default_model,
    DROP COLUMN IF EXISTS global_priority;
