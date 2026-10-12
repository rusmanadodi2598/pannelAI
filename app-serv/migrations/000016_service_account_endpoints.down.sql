-- Reverse of 000016: the service-account columns go and the auth_type CHECK
-- returns to the three values 000005 declared.
--
-- The narrow CHECK is re-added as a validating constraint, so a rollback with
-- service_account rows still in the table FAILS rather than deleting them: the
-- accounts are the operator's data, and a migration that silently dropped them
-- to make its own reverse succeed would be a destructive change wearing a
-- schema change's clothes. An operator rolling this back decides what happens
-- to those endpoints first (delete them, or keep the migration).

ALTER TABLE upstream_endpoints
    DROP CONSTRAINT IF EXISTS upstream_endpoints_auth_check;

ALTER TABLE upstream_endpoints
    ADD CONSTRAINT upstream_endpoints_auth_check
        CHECK (auth_type IN ('api_key', 'oauth', 'no_auth'));

ALTER TABLE upstream_endpoints
    DROP COLUMN IF EXISTS sa_project_id,
    DROP COLUMN IF EXISTS sa_location;
