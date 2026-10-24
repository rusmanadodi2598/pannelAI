-- P3 OAuth dead-letter retry: the window that decides when a buried account is
-- tried again (SPEC-API-001 §7.4, §10; draft 050 T2 decision ii).
--
-- Why a window and not simply sweeping `error` accounts too. The worker's sweep
-- selects accounts whose token is due, and a dead-lettered token is expired, so it
-- is due on EVERY tick. Including those accounts unfiltered re-ran the whole
-- five-attempt budget, with its sleeps, against a vendor that had already answered
-- invalid_grant, and rewrote the dead-letter each round (measured: `grant calls = 6`
-- where the pinned budget is 5). The account is not healed by trying harder; it is
-- healed by a token that works, which only a re-authorization or a vendor-side
-- recovery produces.
--
-- So the grave gets visiting hours, and they widen: an hour after the fifth
-- failure, then two, four, up to a day. A transient cause (the vendor's token
-- endpoint down for the whole five-attempt burst) is found again within an hour
-- without an operator noticing, and a revoked grant is asked once a day instead of
-- once a tick.
--
-- `oauth_retry_rounds` is the ladder's step, stored rather than inferred, because
-- the window alone cannot say how wide the next one should be: a deadline in the
-- past looks the same whether it is the first one or the tenth.
--
-- No index: the sweep reads a provider's OAuth endpoints through the same listing
-- the status route serves, bounded by the page it already uses, and the window is
-- compared in Go against the instant that read already carries (§1.7).

ALTER TABLE upstream_endpoints
    ADD COLUMN IF NOT EXISTS oauth_retry_at timestamptz,
    ADD COLUMN IF NOT EXISTS oauth_retry_rounds integer NOT NULL DEFAULT 0;
