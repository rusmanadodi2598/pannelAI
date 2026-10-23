-- Plan rows. Prices and capacities are PROPOSED in SPEC-BOT-001 §7.1 and live
-- here so a price change is a data change, not a code change.

INSERT INTO bot.plans
    (code, display_name, price_idr, monthly_credits, image_cost, video_cost, max_concurrency, video_enabled, daily_jobs, queue_priority)
VALUES
    ('free',       'Free',       0,      10,   1, 0, 1, false,  3, 100),
    ('basic',      'Basic',      49000,  100,  1, 8, 1, true,  10,  50),
    ('premium',    'Premium',    149000, 350,  1, 6, 2, true,  30,  20),
    ('enterprise', 'Enterprise', 499000, 1200, 1, 5, 4, true, 100,  10)
ON CONFLICT (code) DO NOTHING;
