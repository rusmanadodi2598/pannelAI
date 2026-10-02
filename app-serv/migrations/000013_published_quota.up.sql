-- P1 quota tracker: the cache of what a provider publishes about ITSELF
-- (SPEC-API-001 §7.12, draft 017 §4.3).
--
-- The screen must show the provider's own numbers, but fetching them on read is
-- forbidden: a quota screen over a few hundred accounts would fan out to a few
-- hundred provider calls, which is the N+1 shape AGENTS.md §1.7 blocks outright.
-- So a background worker polls on a schedule and writes the answer here, and the
-- screen reads this cache in one statement. These two tables are that cache: the
-- state row says WHEN an endpoint is next due and what the last answer said, and
-- the window rows are the buckets in that answer.
--
-- Why this is not `quota_windows` (000007). That table's `"window"` column is
-- CHECK-constrained to the four kinds this gateway accounts for — 5h, daily,
-- weekly, monthly — because those are OUR buckets. A provider's buckets are named
-- in the provider's own words ("Claude & GPT (Weekly)", "5-hour limit",
-- "weekly limit"), an unbounded vocabulary that no CHECK set can enumerate and
-- that must not be squashed into four values just to reuse a table: the squash
-- would lose the distinction between two buckets that share a cadence. `label`
-- here is stored verbatim and is the primary key's second column.
--
-- Every statement is idempotent: Apply re-runs the whole file if a previous boot
-- died partway through, and the schema ledger records it only once
-- (migrations/apply_test.go's TestApply_IsIdempotent).

-- One row per endpoint: the worker's scheduling record and the last answer's
-- envelope. `next_attempt_at` is the only column the sweep cannot live without,
-- and it defaults to now() so a newly created endpoint is due immediately rather
-- than after an interval nobody set.
CREATE TABLE IF NOT EXISTS quota_published_state (
    endpoint_id          text        PRIMARY KEY REFERENCES upstream_endpoints (id) ON DELETE CASCADE,
    provider_id          text        NOT NULL,
    plan                 text,
    message              text,
    fetched_at           timestamptz,
    last_attempt_at      timestamptz,
    next_attempt_at      timestamptz NOT NULL DEFAULT now(),
    consecutive_failures integer     NOT NULL DEFAULT 0,
    -- The failure count is a run length, so a negative value is a bug in the
    -- writer rather than a state worth representing — the same rule 000012 pins
    -- on consecutive_use_count. It is also what a backoff interval is computed
    -- from, so one negative row would make every later interval shorter.
    CONSTRAINT quota_published_state_failures_check
        CHECK (consecutive_failures >= 0)
);

-- The sweep's ordering key: "endpoints due by now, oldest first, LIMIT n" reads
-- this index as a range scan and stops. Without it the sweep would sort the whole
-- table on every tick, so the index is load-bearing rather than decorative
-- (AGENTS.md §1.7: an indexed lookup column ships with its index).
CREATE INDEX IF NOT EXISTS idx_quota_published_state_due
    ON quota_published_state (next_attempt_at);

-- One row per bucket the provider published, keyed by the provider's own label.
-- The label is in the key because it is the only thing that distinguishes two
-- buckets of the same cadence; a provider that renames a bucket is handled by the
-- repository, which deletes labels absent from the newest answer rather than
-- letting a renamed total survive beside its replacement.
--
-- `total` is NULLABLE ON PURPOSE, and this is the one column a later reader will
-- be tempted to "simplify" into NOT NULL DEFAULT 0. Do not. NULL and 0 are two
-- different facts a provider states: NULL means the provider published no ceiling
-- at all (unlimited — nothing to divide by, so the panel draws no bar), while 0
-- means a ceiling that exists and is fully spent. Collapsing the two into 0 would
-- render an unlimited allowance as an exhausted one, which is the single most
-- misleading thing this screen could do. The distinction is the same one
-- quota_windows.limit_units holds (000007) for the counters this gateway counts.
--
-- There is deliberately NO percentage column. Percent-of-used-over-total is a
-- display rule, and the panel owns display rules: storing the quotient would make
-- a change of that rule (rounding, clamping, weighting) an UPDATE over every row
-- plus a backfill, when it is only a change to the read side. Same reasoning as
-- UsageTotals.ErrorRate, which is derived rather than stored (domain/usage_totals.go).
CREATE TABLE IF NOT EXISTS quota_published_window (
    endpoint_id text           NOT NULL REFERENCES upstream_endpoints (id) ON DELETE CASCADE,
    label       text           NOT NULL,
    used        numeric(20, 6) NOT NULL,
    total       numeric(20, 6),
    unlimited   boolean        NOT NULL DEFAULT false,
    -- A money balance is a third state, not a synonym for `unlimited`: a prepaid
    -- wallet has a finite amount of cash and no periodic ceiling. Folding it into
    -- `unlimited` would drop the currency, and folding it into a normal window
    -- would draw a "share spent" bar over a number that was never a share.
    is_credit_balance boolean   NOT NULL DEFAULT false,
    recurring   boolean        NOT NULL DEFAULT false,
    unit        text,
    resets_at   timestamptz,
    fetched_at  timestamptz    NOT NULL,
    PRIMARY KEY (endpoint_id, label)
);

-- TTL pruning scans by the instant the answer was fetched, so the sweep that
-- drops stale rows for an endpoint that stopped being polled is a range scan
-- rather than a full-table filter.
CREATE INDEX IF NOT EXISTS idx_quota_published_window_fetched
    ON quota_published_window (fetched_at);
