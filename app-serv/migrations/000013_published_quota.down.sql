-- Reverse of 000013: the published-quota cache is dropped, and its indexes go
-- with the tables rather than needing separate statements.
--
-- No table in this pair is referenced by another migration, so nothing depends
-- on them existing; dropping in child-then-parent order is for readability, since
-- both carry an ON DELETE CASCADE to upstream_endpoints rather than one to the
-- other.

DROP INDEX IF EXISTS idx_quota_published_window_fetched;
DROP TABLE IF EXISTS quota_published_window;
DROP INDEX IF EXISTS idx_quota_published_state_due;
DROP TABLE IF EXISTS quota_published_state;
