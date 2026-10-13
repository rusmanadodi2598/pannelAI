-- Reverse of 000018: the retry window and its ladder step go.
--
-- Dropping them discards only the schedule, never an account: a dead-lettered
-- endpoint keeps the `error` status and the reason it got there, and the operator's
-- refresh call still reaches it. What a rollback loses is the automatic visit, so
-- those accounts wait for a human again, which is exactly the state draft 050 T2
-- described before the fix.

ALTER TABLE upstream_endpoints
    DROP COLUMN IF EXISTS oauth_retry_rounds,
    DROP COLUMN IF EXISTS oauth_retry_at;
