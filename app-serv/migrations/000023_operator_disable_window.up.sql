-- An operator parking an account and the gateway parking one are different decisions, and
-- until now only the gateway's had a deadline: `status = 'disabled'` was permanent until
-- someone PATCHed it back, so a free-tier account switched off for its 24-hour reset had
-- to be switched on by hand.
--
-- This is the operator's deadline, and it is a separate column for the reason 000017
-- states about `quota_cooldown_until`: one meaning per column. `disabled_until` is the
-- opposite of that column, because it IS an input to `Available()` and is meant to be.
-- `quota_cooldown_until` holds an account off the poller; this one holds it out of
-- rotation, and conflating them would make a vendor's throttle look like an operator's
-- switch the moment either is read by the other's path.
--
-- NULL means "no window": a disable with no deadline is permanent, the state every row
-- already had before this column, so no backfill is needed and no existing row changes
-- meaning. The window is evaluated by the wall clock when the row is read and nothing
-- clears it, so expired parks need no sweeper and no worker: the row stays `disabled` for
-- audit until the next successful request puts it back, exactly how a health park already
-- behaves.
--
-- Both tables carry it because a bulk disable addresses keys and accounts with one
-- selector, and a key parked with no deadline of its own would re-enter rotation the
-- moment its endpoint did.
--
-- No index: nothing is looked up BY this deadline. It is compared against the clock on a
-- row selection already reached by primary key or by the provider listing, which is
-- bounded by MaxEndpointsPerProvider (§1.7).

ALTER TABLE upstream_endpoints
    ADD COLUMN IF NOT EXISTS disabled_until timestamptz;

ALTER TABLE upstream_keys
    ADD COLUMN IF NOT EXISTS disabled_until timestamptz;
