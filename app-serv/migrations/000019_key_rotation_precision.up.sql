-- P3 rotation precision: the columns that let a key be recorded at the moment it
-- is CHOSEN, not only when it succeeds (SPEC-API-001 §7.5).
--
-- Why new columns and not the `last_used_at` that already exists and already has a
-- reader. That column is stamped by `RecordSuccess` alone, which is the whole
-- reason a burst does not spread: N concurrent selections read one identical
-- `last_used_at`, the least-recently-used rule answers the same key for all of
-- them, and nothing in the row records that the key was just spent. A key whose
-- selection then failed with a target-shaped error (DNS, TLS, an upstream 404) is
-- not parked either, because that class implicates no credential, so its
-- timestamp stays the oldest in the set and it wins every subsequent selection
-- forever.
--
-- `last_selected_at` is written by the selection itself and answers the ordering
-- question the rotation cursor would answer if Redis could. The three streak
-- columns replace one shared `consecutive_errors` counter as the ladder's step:
-- a run of 429s was raising the auth window, because `WindowFor` keyed the
-- ladder on the class but the step on a counter every class incremented. The
-- total stays for display, so a reader that predates this migration still sees
-- how many failures a key has taken.
--
-- `last_error_class` names which of those runs the last failure advanced. The
-- health write increments the matching column inside the database rather than
-- writing the aggregate's own count, so two concurrent failures of one class
-- count twice, and the statement has to know the class to pick the column. It is
-- also the fact the panel cannot show today: `last_error` carries the upstream's
-- sentence, which says "too many requests" or "invalid key" only if the vendor
-- chose to say so in words an operator can read.
--
-- No index: both are read only inside the endpoint page the selection already
-- loads, which `MaxEndpointsPerProvider` bounds at 100, and every write addresses
-- the row by its primary key `(id, endpoint_id)` (§1.7).

ALTER TABLE upstream_keys
    ADD COLUMN IF NOT EXISTS last_selected_at timestamptz,
    ADD COLUMN IF NOT EXISTS consecutive_auth integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS consecutive_ratelimit integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS consecutive_billing integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_error_class text;
