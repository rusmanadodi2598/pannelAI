-- Reverse of 000020: the account's failure run and its selection stamp go.

-- Dropping them returns a dead OAuth account to the state draft 050 described:
-- selected again on every request that rotation hands it, refused, and retried by
-- the next request. No credential and no usage row is lost, because the token
-- lives in `oauth` and the reason it failed in `last_error`, and both stay. The
-- endpoint park itself survives a rollback in `rate_limited_until`, which this
-- migration never touches.

ALTER TABLE upstream_endpoints
    DROP COLUMN IF EXISTS last_selected_at,
    DROP COLUMN IF EXISTS consecutive_errors;
