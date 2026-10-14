-- P3 account circuit breaker: the columns that let a KEYLESS endpoint be parked.
--
-- `rate_limited_until` already exists on this table and `Available()` already
-- reads it, but nothing on the health path writes it, so an endpoint carries a
-- deadline nobody sets. Draft 050 T1 kept it that way on purpose for a KEYED
-- endpoint: a vendor 429 arrives on one of an endpoint's 1..N keys, and filling
-- the endpoint column would withhold the whole account, its healthy keys included.
-- That reasoning does not reach an endpoint with no keys at all.
--
-- An OAuth or no-auth endpoint has exactly one credential, which lives on the
-- endpoint row rather than in `upstream_keys`, so the failure IS account-level and
-- `selection_health` writes no breaker for it: its guard required a key id. A dead
-- OAuth token therefore stayed in rotation indefinitely and every request that
-- landed on it paid a full upstream round trip to be refused. Parking the account
-- is the correct trade there and evicts nobody unfairly, because there is no
-- sibling key to evict.
--
-- `consecutive_errors` is that park's ladder step, for the same reason 000019 gives
-- the keys one: the park length has to grow with the account's own run, and the
-- endpoint's `consecutive_use_count` is a use counter, not a failure counter, so it
-- cannot be reused. `last_selected_at` mirrors the key column: an account the
-- rotation cursor could not reach falls back to ordering by when it was last
-- chosen rather than when it last succeeded.

ALTER TABLE upstream_endpoints
    ADD COLUMN IF NOT EXISTS consecutive_errors integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_selected_at timestamptz;
