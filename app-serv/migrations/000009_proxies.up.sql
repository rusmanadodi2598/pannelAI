-- P2 schema: saved outbound proxy candidates (SPEC-API-001 §6, §7.11).
-- The password is stored sealed (AES-256-GCM, the same sealer endpoints use)
-- and is write-only on the wire; the status document carries the last
-- connectivity test so the panel can show when a candidate was last proven.

CREATE TABLE IF NOT EXISTS proxies (
    id                 text        PRIMARY KEY,
    label              text        NOT NULL,
    protocol           text        NOT NULL,
    host               text        NOT NULL,
    port               integer     NOT NULL,
    username           text        NOT NULL DEFAULT '',
    password_encrypted text        NOT NULL DEFAULT '',
    enabled            boolean     NOT NULL DEFAULT true,
    status             jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT proxies_protocol_check CHECK (protocol IN ('http', 'https', 'socks5')),
    CONSTRAINT proxies_port_check CHECK (port BETWEEN 1 AND 65535)
);

-- The list orders by label, and the panel reads the whole pool, so the label
-- carries the index rather than the id.
CREATE INDEX IF NOT EXISTS idx_proxies_label ON proxies (label);
