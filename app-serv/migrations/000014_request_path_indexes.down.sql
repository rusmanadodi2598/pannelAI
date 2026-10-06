-- Reverse of 000014: the four indexes are dropped, and nothing else goes with
-- them. Indexes carry no data of their own, so a rollback costs query time and
-- not a row.

DROP INDEX IF EXISTS idx_upstream_endpoints_account_workspace;
DROP INDEX IF EXISTS idx_upstream_endpoints_account_email;
DROP INDEX IF EXISTS idx_usage_records_status_ts;
DROP INDEX IF EXISTS idx_request_logs_gateway_key_ts;
