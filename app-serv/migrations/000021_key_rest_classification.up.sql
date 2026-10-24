-- P3 failure classification: what a park's rest was and how it was arrived at.
--
-- A 429 is two failures and only the vendor can say which: a few minutes of load,
-- or an allowance spent until the date its own body prints. Both parked the key
-- for five minutes, so a spent allowance re-entered rotation long before its real
-- reset and burned a walk leg every cycle to be refused again.
--
-- `signature_id` names the signature that recognised the refusal, and
-- `reset_source` says which statement produced the rest: `header` for a
-- Retry-After, `body` for a date the vendor stated in its own text, `default` for
-- the class ladder. Neither is read by the router: they are how an operator tells
-- a park the vendor warranted from one the gateway guessed. Both are nullable and
-- both are written only by the health write, so a row parked by an older binary
-- simply carries neither and every existing query is unaffected.
--
-- No index: they are read beside `last_error_class` on the endpoint page the
-- selection already loads, bounded by `MaxEndpointsPerProvider`, and every write
-- addresses the row by its primary key `(id, endpoint_id)` (§1.7).

ALTER TABLE upstream_keys
    ADD COLUMN IF NOT EXISTS reset_source text,
    ADD COLUMN IF NOT EXISTS signature_id text;
