-- P2 auth: the service-account credential a Google provider needs (SPEC-API-001
-- §7.5, §8.1).
--
-- Vertex AI authenticates with a service-account JSON rather than a static key
-- or an OAuth flow: the gateway signs a JWT with the account's private key and
-- exchanges it for a bearer token before the call. The JSON itself is a secret
-- and lives where every other credential lives, sealed in upstream_keys; what
-- an endpoint was missing is the two non-secret values that address the call,
-- the project and the location, which is what these columns carry.
--
-- The auth_type CHECK is widened rather than replaced by a second constraint:
-- one constraint stating the closed set is what keeps a typo'd auth_type out of
-- the table, and a second one would let the two disagree. It is dropped and
-- re-added as a pair, which is what makes the file re-runnable: Apply runs the
-- whole file again if a previous boot died partway through, and ADD CONSTRAINT
-- alone is not idempotent (migrations/apply_test.go's TestApply_IsIdempotent).
-- The drop and the add both take the table's exclusive lock and the add
-- revalidates every row; at this table's size that is a boot that takes
-- milliseconds longer, and the alternative is a constraint the file cannot
-- promise is the one it declared.
--
-- Both columns are NOT NULL with an empty default, so a row created before this
-- migration reads back as empty rather than failing the scan, the same rule
-- 000012's nullable parity columns follow from the other side.

ALTER TABLE upstream_endpoints
    DROP CONSTRAINT IF EXISTS upstream_endpoints_auth_check;

ALTER TABLE upstream_endpoints
    ADD CONSTRAINT upstream_endpoints_auth_check
        CHECK (auth_type IN ('api_key', 'oauth', 'no_auth', 'service_account'));

ALTER TABLE upstream_endpoints
    ADD COLUMN IF NOT EXISTS sa_project_id text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS sa_location   text NOT NULL DEFAULT '';
