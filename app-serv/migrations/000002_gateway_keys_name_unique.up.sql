-- P0 correction: gateway key names are unique (SPEC-API-001 §7.3, §8 CONFLICT).
--
-- 000001 created a plain index on name, so a second key with the same name was
-- accepted by PostgreSQL while the repository contract and the API promise
-- domain.ErrGatewayKeyExists. The unique index is what makes that promise true.
--
-- It is a separate migration rather than an edit to 000001 so that a database
-- where the table already exists still receives the constraint.

CREATE UNIQUE INDEX IF NOT EXISTS uniq_gateway_keys_name ON gateway_keys (name);

-- The plain index from 000001 is redundant once the unique index exists.
DROP INDEX IF EXISTS idx_gateway_keys_name;