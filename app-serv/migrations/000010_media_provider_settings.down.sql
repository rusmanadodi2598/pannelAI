-- Down for 000010: the per-provider media overrides (SPEC-API-001 §7.10).
-- Dropping the table loses only operator overrides; the embedded registry
-- defaults remain, so the media routes keep working with their built-in
-- base URLs.

DROP TABLE IF EXISTS media_provider_settings;
