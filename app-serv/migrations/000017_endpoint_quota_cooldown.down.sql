-- Reverse of 000017: the quota cooldown column goes.
--
-- Discarding it loses only deadlines that were still counting down, and every one
-- of them expires within the window on its own, so a rollback costs the poller at
-- most one wasted provider call per account. No row is deleted and no other column
-- is touched: the endpoints, their keys, and their parity state survive intact.

ALTER TABLE upstream_endpoints
    DROP COLUMN IF EXISTS quota_cooldown_until;
