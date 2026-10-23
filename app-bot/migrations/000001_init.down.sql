-- Rollback of the first migration. Tables drop in reverse dependency order;
-- the schema and schema_migrations stay, so the runner can record the state.

DROP TABLE IF EXISTS bot.admin_audit;
DROP TABLE IF EXISTS bot.payments;
DROP TABLE IF EXISTS bot.referral_redemptions;
DROP TABLE IF EXISTS bot.referral_campaigns;
DROP TABLE IF EXISTS bot.generation_jobs;
DROP TABLE IF EXISTS bot.credit_ledger;
DROP TABLE IF EXISTS bot.subscriptions;
DROP TABLE IF EXISTS bot.users;
DROP TABLE IF EXISTS bot.plans;
