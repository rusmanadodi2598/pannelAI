-- Rollback for 000001 (destructive by design: drops the P0 credential table).
-- Only run when tearing down a dev database; never apply blindly in production.

DROP TABLE IF EXISTS gateway_keys;
