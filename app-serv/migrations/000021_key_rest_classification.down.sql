-- Reverse of 000021: the two diagnostics a park carries go.
--
-- Only the provenance is lost. The park itself, its length, the class that earned
-- it and the vendor's own sentence all stay in `rate_limited_until`,
-- `last_error_class` and `last_error`, so a rollback costs an operator the answer
-- to "why this long" and nothing else.

ALTER TABLE upstream_keys
    DROP COLUMN IF EXISTS reset_source,
    DROP COLUMN IF EXISTS signature_id;
