-- Bulk work needs a name for a set of accounts that is not their provider: the
-- operator marks "the free-tier batch I parked for this event" and acts on it again
-- without re-selecting rows across pages.
--
-- Why jsonb and not a text[]: the tables already carry codec columns written the same
-- way (`capabilities`, `fallback_chain` in 000006), so the row decoder keeps one shape
-- rather than a second driver type. Why GIN and not an expression index: the question is
-- "every row that holds tag X", containment, not one value at a fixed position. The
-- `account` index in 000014 is an expression index over a single key and cannot be
-- reused for this.
--
-- The default is the empty array, not NULL: an untagged row is a real state the router
-- reads on every selection, and a nullable collection would push a NULL check into every
-- containment query. Tags are bounded in the aggregate (16 per row, 48 characters each),
-- which is where an over-long tag is refused rather than silently cut.
--
-- Both tables get the column because a bulk action addresses keys and accounts with the
-- same selector, and a tag that only exists on the endpoint cannot name the key it was
-- meant to group.

ALTER TABLE upstream_endpoints
    ADD COLUMN IF NOT EXISTS tags jsonb NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE upstream_keys
    ADD COLUMN IF NOT EXISTS tags jsonb NOT NULL DEFAULT '[]'::jsonb;

CREATE INDEX IF NOT EXISTS idx_upstream_endpoints_tags ON upstream_endpoints USING gin (tags);

CREATE INDEX IF NOT EXISTS idx_upstream_keys_tags ON upstream_keys USING gin (tags);
