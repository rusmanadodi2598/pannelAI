CREATE TABLE IF NOT EXISTS panel_auth (
    id            smallint PRIMARY KEY CHECK (id = 1),
    password_hash text,
    updated_at    timestamptz NOT NULL DEFAULT now()
);

INSERT INTO panel_auth (id)
VALUES (1)
ON CONFLICT (id) DO NOTHING;
