-- A request id is a caller's trace value, not a unique audit key. Giving the
-- table a server-generated id as its primary key means a client that reuses an
-- id creates a second row instead of being silently dropped by ON CONFLICT.
--
-- Existing rows keep their request id as the new id: it was unique under the old
-- primary key, so the backfill cannot collide. The request id stays indexed for
-- the list filter and the detail lookup.

ALTER TABLE request_logs
    ADD COLUMN IF NOT EXISTS id text;

UPDATE request_logs SET id = request_id WHERE id IS NULL;

ALTER TABLE request_logs
    ALTER COLUMN id SET NOT NULL;

ALTER TABLE request_logs
    DROP CONSTRAINT IF EXISTS request_logs_pkey;

ALTER TABLE request_logs
    ADD CONSTRAINT request_logs_pkey PRIMARY KEY (id);

CREATE INDEX IF NOT EXISTS idx_request_logs_request_id ON request_logs (request_id);
