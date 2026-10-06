-- Reverse of 000015, taken literally: the expression index goes and the bare one 000014
-- built returns, so the schema ends where 000014 left it. The returned index serves the
-- dedup read as little as it did before, which is the point of rolling this one back.

DROP INDEX IF EXISTS idx_upstream_endpoints_account_email;

CREATE INDEX IF NOT EXISTS idx_upstream_endpoints_account_email
    ON upstream_endpoints ((account->>'email'));
