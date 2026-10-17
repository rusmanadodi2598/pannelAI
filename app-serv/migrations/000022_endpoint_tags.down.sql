-- Reverse of 000022: the tag columns and the two containment indexes go.
--
-- Nothing else is touched. Tags are a label an operator adds on top of rows that already
-- exist, so discarding them removes no account, no key, and no health state: the rows
-- return to the untagged shape they had before the migration, and the only loss is the
-- grouping a bulk action would have read.

DROP INDEX IF EXISTS idx_upstream_keys_tags;

DROP INDEX IF EXISTS idx_upstream_endpoints_tags;

ALTER TABLE upstream_keys
    DROP COLUMN IF EXISTS tags;

ALTER TABLE upstream_endpoints
    DROP COLUMN IF EXISTS tags;
