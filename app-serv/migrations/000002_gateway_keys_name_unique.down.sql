-- Rollback for 000002: restores the non-unique index and drops the constraint.

DROP INDEX IF EXISTS uniq_gateway_keys_name;
CREATE INDEX IF NOT EXISTS idx_gateway_keys_name ON gateway_keys (name);