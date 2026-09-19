-- Re-owns the P2 tables to the role the rest of the schema belongs to.
-- A migration runs as whatever role boots the gateway, so a boot with a
-- superuser DSN creates tables the superuser owns and the app role cannot
-- read. Measured live (G19, 2026-09-19): `PATCH /media-providers/{id}` and
-- every §7.11 route answered 500 `permission denied for table
-- media_provider_settings` while all other tables answered normally, because
-- `media_provider_settings` and `proxies` were created by a superuser boot
-- (migrations 000009/000010) while 000001-000008 ran as the app role.
--
-- The anchor is `gateway_keys`, the table created by the first migration:
-- that states one rule — the P2 tables belong to the same role as the P1
-- tables — instead of naming a role that differs per deployment.
--
-- A role that does not own a table cannot re-own it, and that is exactly the
-- app-role boot this migration is meant to survive: failing here would turn
-- two broken routes into a boot that never comes up. So a refusal is caught
-- and reported as a warning carrying the statement to run, and the boot
-- proceeds with the rest of the schema applied.

DO $$
DECLARE
    app_role text;
    target   text;
    owner    text;
BEGIN
    SELECT pg_get_userbyid(relowner) INTO app_role
    FROM pg_class WHERE oid = 'gateway_keys'::regclass;

    IF app_role IS NULL THEN
        RAISE EXCEPTION 'gateway_keys has no owner to anchor the P2 tables to';
    END IF;

    FOREACH target IN ARRAY ARRAY['media_provider_settings', 'proxies'] LOOP
        SELECT pg_get_userbyid(relowner) INTO owner
        FROM pg_class WHERE oid = target::regclass;

        IF owner = app_role THEN
            CONTINUE;
        END IF;

        BEGIN
            EXECUTE format('ALTER TABLE %I OWNER TO %I', target, app_role);
        EXCEPTION WHEN insufficient_privilege THEN
            RAISE WARNING
                'table % is owned by % and % may not re-own it; run as a superuser: ALTER TABLE % OWNER TO %',
                target, owner, current_user, target, app_role;
        END;
    END LOOP;
END $$;
