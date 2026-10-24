-- Reverse of 000023: the operator's deadline goes, the decision stays.
--
-- Every account an operator parked remains `disabled` in `status`, which is the column
-- that carries the switch itself, so a rollback does not silently re-open anything that
-- was closed. What is lost is the promise that a parked account comes back on its own:
-- after the drop, a disable is permanent until an operator reverses it, the state this
-- table had before the column existed.

ALTER TABLE upstream_keys
    DROP COLUMN IF EXISTS disabled_until;

ALTER TABLE upstream_endpoints
    DROP COLUMN IF EXISTS disabled_until;
