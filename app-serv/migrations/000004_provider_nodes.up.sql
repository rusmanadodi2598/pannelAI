-- P1 schema: custom provider nodes (SPEC-API-001 §7.4, §6).
-- A provider node is a user-defined endpoint the embedded registry does not
-- ship: an OpenAI-compatible or Anthropic-compatible base URL. Its id is
-- prefixed so the registry loader can synthesize the same runtime provider a
-- built-in entry would get, without a second code path downstream.

CREATE TABLE IF NOT EXISTS provider_nodes (
    id         text        PRIMARY KEY,
    type       text        NOT NULL,
    name       text        NOT NULL,
    prefix     text        NOT NULL,
    api_type   text        NOT NULL DEFAULT '',
    base_url   text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT provider_nodes_type_check
        CHECK (type IN ('openai-compatible', 'anthropic-compatible'))
);

-- A prefix is the model-string namespace ("mycorp/gpt-4o"), so it must be
-- unique across custom nodes: two nodes claiming one prefix would make model
-- resolution ambiguous.
CREATE UNIQUE INDEX IF NOT EXISTS idx_provider_nodes_prefix ON provider_nodes (prefix);
CREATE INDEX IF NOT EXISTS idx_provider_nodes_type ON provider_nodes (type);