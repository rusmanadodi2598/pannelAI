-- P3 index completion: the three lookups the request path makes that were
-- scanning (AGENTS.md §1.7: a new indexed lookup column ships with a migration
-- adding the index; this adds the indexes for columns that already had readers).
--
-- Why these three and not others. Each was found from a filter in the code, not
-- from a guess about the schema:
--
--   request_logs.gateway_key_id   handler/quota and the log screen narrow to one
--                                 key (`log.go`'s `$6 = '' OR gateway_key_id = $6`),
--                                 and the page then orders by ts.
--   usage_records.status          the usage reads filter `status` over a time
--                                 window, the one dimension the 000007 indexes
--                                 (ts, provider_id+ts) do not cover.
--   upstream_endpoints account    FindOAuthEndpoint dedupes an OAuth login by
--                                 `account->>'email'` / `account->>'workspace_id'`
--                                 on every connect. A jsonb path with no
--                                 expression index is a sequential scan of every
--                                 account, on the path a login takes.
--
-- The `($n = '' OR column = $n)` shape is what limits how well the first two are
-- used: the disjunction keeps a generic plan from committing to the index. The
-- index still serves the branch that matters and costs nothing when it does not;
-- restructuring those predicates into conditionally built SQL is a separate,
-- larger change and is deliberately not attempted here.
--
-- Every statement is idempotent, as the runner requires (migrations/apply_test.go's
-- TestApply_IsIdempotent).

CREATE INDEX IF NOT EXISTS idx_request_logs_gateway_key_ts
    ON request_logs (gateway_key_id, ts DESC);

CREATE INDEX IF NOT EXISTS idx_usage_records_status_ts
    ON usage_records (status, ts DESC);

CREATE INDEX IF NOT EXISTS idx_upstream_endpoints_account_email
    ON upstream_endpoints ((account->>'email'));

CREATE INDEX IF NOT EXISTS idx_upstream_endpoints_account_workspace
    ON upstream_endpoints ((account->>'workspace_id'));
