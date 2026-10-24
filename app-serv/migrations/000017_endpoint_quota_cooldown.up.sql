-- P3 quota cooldown: the column that lets a vendor's 429 hold the poller off the
-- account it just throttled (SPEC-API-001 §7.12, draft 050 T1).
--
-- Why a new column and not `rate_limited_until`, which already exists on this
-- table and already has a reader. That column is an input to `Available()`, which
-- decides whether an account may SERVE traffic. Nothing writes a deadline into it
-- today, and the reason is not an oversight: a vendor's 429 arrives on ONE key of
-- an endpoint that holds 1..N keys (§7.5), and the key that was throttled is
-- already parked by the health model. Filling the endpoint column would withhold
-- the whole account, its healthy keys included, from the router, because a
-- request carries one credential and cannot be re-served by a sibling key at that
-- point. The published-quota poller needs the opposite trade: an account the
-- vendor is throttling is a wasted provider call, and a wasted call costs nothing
-- but budget. Two questions, two columns.
--
-- The window is therefore write-once-expire-by-itself: a 429 stamps it, nothing
-- clears it early, and `Available()` never reads it. A success does not clear it
-- either, because a served request is the gateway's own traffic and says nothing
-- about whether the vendor will answer a usage read.

ALTER TABLE upstream_endpoints
    ADD COLUMN IF NOT EXISTS quota_cooldown_until timestamptz;
