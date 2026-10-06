-- 000015 replaces the OAuth email index 000014 built.
--
-- FindOAuthEndpoint matches the account identity as `lower(account->>'email') = $2`
-- (endpoint_batch.go), because the column may hold any letter case while the
-- argument arrives canonical from domain.NormalizeEmail. An index over the bare
-- `account->>'email'` cannot answer that expression, so the dedup read 000014 was
-- added for still walked the table: the index existed, and nothing used it.
--
-- The index keeps its name. Nothing in the schema refers to an index by name, and a
-- name that does not move keeps a database that already ran 000014 converging on the
-- same shape as a fresh one, which is what the ledger per version cannot promise on
-- its own.
--
-- The lock window 000014 takes on `request_logs` and `usage_records` is deliberately
-- left alone. Those two builds are plain CREATE INDEX inside the runner's transaction,
-- so a populated database blocks their writes for the duration of the build. At this
-- deployment's size that is a boot that takes seconds longer, and the alternative is
-- an online path the boot runner cannot hold: CREATE INDEX CONCURRENTLY is refused
-- inside a transaction, and run outside one it waits on the other replica's
-- pg_advisory_lock transaction while that replica waits on it, which deadlocks a
-- simultaneous start. If those tables ever grow past that trade-off, the fix is an
-- operator-run rebuild under CONCURRENTLY, not a change to this runner.
--
-- Every statement is idempotent, as the runner requires (migrations/apply_test.go's
-- TestApply_IsIdempotent).

DROP INDEX IF EXISTS idx_upstream_endpoints_account_email;

CREATE INDEX IF NOT EXISTS idx_upstream_endpoints_account_email
    ON upstream_endpoints ((lower(account->>'email')));
