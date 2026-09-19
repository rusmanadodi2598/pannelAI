-- Down for 000011: intentionally empty.
-- The migration repairs an ownership defect rather than introducing a schema
-- change, so the state it replaces is the defect itself: restoring the old
-- owner would re-break the routes the migration fixes. Ownership is also not
-- recoverable — the pre-fix owner is not recorded anywhere to return to.
SELECT 1;
