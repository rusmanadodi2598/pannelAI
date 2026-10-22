-- Reverse of 000024. This restore can fail once the new insert path has stored
-- two rows with one request id: the old primary key cannot hold both. The down
-- migration is manual (README), so resolve the duplicates before running it.

ALTER TABLE request_logs
    DROP CONSTRAINT IF EXISTS request_logs_pkey;

ALTER TABLE request_logs
    ADD CONSTRAINT request_logs_pkey PRIMARY KEY (request_id);

DROP INDEX IF EXISTS idx_request_logs_request_id;

ALTER TABLE request_logs
    DROP COLUMN IF EXISTS id;
