-- app-bot schema, first migration.
-- The gateway owns its own tables; this schema never joins into app-serv.

CREATE SCHEMA IF NOT EXISTS bot;

CREATE TABLE IF NOT EXISTS bot.plans (
    code            TEXT PRIMARY KEY,
    display_name    TEXT NOT NULL,
    price_idr       INTEGER NOT NULL CHECK (price_idr >= 0),
    monthly_credits INTEGER NOT NULL CHECK (monthly_credits >= 0),
    image_cost      INTEGER NOT NULL CHECK (image_cost > 0),
    video_cost      INTEGER NOT NULL CHECK (video_cost >= 0),
    max_concurrency INTEGER NOT NULL CHECK (max_concurrency > 0),
    video_enabled   BOOLEAN NOT NULL DEFAULT false,
    daily_jobs      INTEGER NOT NULL CHECK (daily_jobs > 0),
    queue_priority  INTEGER NOT NULL DEFAULT 100,
    active          BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE IF NOT EXISTS bot.users (
    id               BIGSERIAL PRIMARY KEY,
    telegram_user_id BIGINT NOT NULL UNIQUE,
    username         TEXT NOT NULL DEFAULT '',
    first_name       TEXT NOT NULL DEFAULT '',
    language_code    TEXT NOT NULL DEFAULT '',
    plan_code        TEXT NOT NULL DEFAULT 'free' REFERENCES bot.plans(code),
    credits_balance  INTEGER NOT NULL DEFAULT 0 CHECK (credits_balance >= 0),
    referred_by      BIGINT REFERENCES bot.users(id),
    status           TEXT NOT NULL DEFAULT 'active',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS bot.subscriptions (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES bot.users(id),
    plan_code       TEXT NOT NULL REFERENCES bot.plans(code),
    status          TEXT NOT NULL,
    period_start    TIMESTAMPTZ NOT NULL,
    period_end      TIMESTAMPTZ NOT NULL,
    credits_granted INTEGER NOT NULL,
    payment_id      BIGINT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS subscriptions_user_period_idx
    ON bot.subscriptions (user_id, period_end DESC);

CREATE TABLE IF NOT EXISTS bot.credit_ledger (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES bot.users(id),
    delta        INTEGER NOT NULL,
    reason       TEXT NOT NULL,
    reference_id TEXT NOT NULL DEFAULT '',
    balance_after INTEGER NOT NULL CHECK (balance_after >= 0),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS credit_ledger_user_idx
    ON bot.credit_ledger (user_id, id DESC);
CREATE UNIQUE INDEX IF NOT EXISTS credit_ledger_once_idx
    ON bot.credit_ledger (user_id, reason, reference_id)
    WHERE reason IN ('referral_grant', 'subscription_grant', 'topup_purchase');

CREATE TABLE IF NOT EXISTS bot.generation_jobs (
    id                  UUID PRIMARY KEY,
    user_id             BIGINT NOT NULL REFERENCES bot.users(id),
    kind                TEXT NOT NULL CHECK (kind IN ('image', 'video')),
    status              TEXT NOT NULL,
    model               TEXT NOT NULL,
    master_prompt       JSONB NOT NULL,
    compiled_prompt     TEXT NOT NULL,
    builder             TEXT NOT NULL DEFAULT 'model',
    builder_model       TEXT NOT NULL DEFAULT '',
    provider_task_id    TEXT NOT NULL DEFAULT '',
    result_url          TEXT NOT NULL DEFAULT '',
    cost_credits        INTEGER NOT NULL CHECK (cost_credits > 0),
    error_code          TEXT NOT NULL DEFAULT '',
    telegram_message_id BIGINT NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at        TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS generation_jobs_user_idx
    ON bot.generation_jobs (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS generation_jobs_active_idx
    ON bot.generation_jobs (status, updated_at)
    WHERE status IN ('queued', 'submitted', 'running');

CREATE TABLE IF NOT EXISTS bot.referral_campaigns (
    id              BIGSERIAL PRIMARY KEY,
    code            TEXT NOT NULL UNIQUE CHECK (code ~ '^[A-Z0-9_-]{6,24}$'),
    name            TEXT NOT NULL,
    grant_credits   INTEGER NOT NULL CHECK (grant_credits >= 0),
    grant_plan      TEXT NOT NULL REFERENCES bot.plans(code),
    max_redemptions INTEGER NOT NULL CHECK (max_redemptions > 0),
    redeemed_count  INTEGER NOT NULL DEFAULT 0 CHECK (redeemed_count >= 0),
    starts_at       TIMESTAMPTZ NOT NULL,
    ends_at         TIMESTAMPTZ NOT NULL,
    status          TEXT NOT NULL DEFAULT 'active',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (redeemed_count <= max_redemptions)
);

CREATE TABLE IF NOT EXISTS bot.referral_redemptions (
    id          BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES bot.referral_campaigns(id),
    user_id     BIGINT NOT NULL REFERENCES bot.users(id),
    ip_hash     TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (campaign_id, user_id)
);

CREATE TABLE IF NOT EXISTS bot.payments (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES bot.users(id),
    order_id        TEXT NOT NULL UNIQUE,
    plan_code       TEXT NOT NULL REFERENCES bot.plans(code),
    amount_idr      INTEGER NOT NULL CHECK (amount_idr > 0),
    status          TEXT NOT NULL,
    payment_method  TEXT NOT NULL DEFAULT 'qris',
    checkout_url    TEXT NOT NULL DEFAULT '',
    provider_ref    TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT NOT NULL UNIQUE,
    paid_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS payments_pending_idx
    ON bot.payments (status, created_at)
    WHERE status IN ('created', 'pending');

CREATE TABLE IF NOT EXISTS bot.admin_audit (
    id         BIGSERIAL PRIMARY KEY,
    actor      TEXT NOT NULL,
    action     TEXT NOT NULL,
    target     TEXT NOT NULL,
    detail     JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
