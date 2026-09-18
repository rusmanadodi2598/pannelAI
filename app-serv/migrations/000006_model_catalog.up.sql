-- P1 schema: combos, model aliases, custom models, disabled models
-- (SPEC-API-001 §6, §7.6, §7.7).
-- A combo is an ordered model list with a strategy, addressed by name in the
-- model string. models is jsonb ([{ref, priority}]) because the list is
-- always read and written whole: it is part of the aggregate, not a table of
-- children, and a child table would let a combo be loaded in a partially
-- updated state that the strategy can never produce.

CREATE TABLE IF NOT EXISTS combos (
    id           text        PRIMARY KEY,
    name         text        NOT NULL,
    strategy     text        NOT NULL DEFAULT 'fallback',
    sticky_limit integer     NOT NULL DEFAULT 1,
    judge_model  text,
    models       jsonb       NOT NULL DEFAULT '[]'::jsonb,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT combos_strategy_check
        CHECK (strategy IN ('fallback', 'round_robin', 'fusion')),
    CONSTRAINT combos_sticky_limit_check CHECK (sticky_limit >= 1)
);

-- A combo name is a model string a client sends, so it must resolve to exactly
-- one combo.
CREATE UNIQUE INDEX IF NOT EXISTS idx_combos_name ON combos (name);

-- alias -> "provider/model" or a combo name. PUT replaces the whole set, so
-- this table is small and fully rewritten; alias is the natural key.
CREATE TABLE IF NOT EXISTS model_aliases (
    alias      text        PRIMARY KEY,
    target     text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- A user-added model, referenced by (provider_id, model_id) like every
-- registry model, so the catalog merge has one shape.
CREATE TABLE IF NOT EXISTS models_custom (
    id           text        PRIMARY KEY,
    provider_id  text        NOT NULL,
    model_id     text        NOT NULL,
    display_name text        NOT NULL,
    capabilities jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_models_custom_provider_model
    ON models_custom (provider_id, model_id);

-- Disabled models are hidden from the catalog and refused by routing. The pair
-- is the key: a row exists or it does not.
CREATE TABLE IF NOT EXISTS models_disabled (
    provider_id text        NOT NULL,
    model_id    text        NOT NULL,
    disabled_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (provider_id, model_id)
);