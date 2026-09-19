-- P2 schema: per-provider media overrides (SPEC-API-001 §7.10).
-- The embedded registry carries a media base URL and default model per kind.
-- An operator override lives here so a self-hosted provider can be pointed at
-- its own host, or a different default model chosen, without editing the
-- registry. Keyed by (provider_id, kind) because one provider may serve
-- several kinds from different hosts, and a base URL that fits embeddings
-- rarely fits speech.
--
-- The row is an override, not a copy: an empty base_url means "use the
-- registry's", which is why the columns default to '' rather than NULL — the
-- service reads one representation, not two.

CREATE TABLE IF NOT EXISTS media_provider_settings (
    provider_id   text        NOT NULL,
    kind          text        NOT NULL,
    base_url      text        NOT NULL DEFAULT '',
    default_model text        NOT NULL DEFAULT '',
    updated_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (provider_id, kind),
    CONSTRAINT media_provider_settings_kind_check
        CHECK (kind IN ('tts', 'stt', 'embedding', 'image', 'video', 'search'))
);
