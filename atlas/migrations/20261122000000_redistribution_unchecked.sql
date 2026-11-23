-- Switch the redistribution_policy constraint to allow unchecked. Only DDL is
-- done here so the ACCESS EXCLUSIVE lock is held for a catalog update, not
-- while existing rows are rewritten. The new CHECK is NOT VALID, so adding it
-- does not scan file_objects.
--
-- pending is kept in this interim CHECK: NOT VALID still enforces the CHECK on
-- updated rows, and pending rows must remain updatable until the backfill in
-- 20261122000100 finishes. The final CHECK removes pending afterwards.
ALTER TABLE file_objects
  DROP CONSTRAINT IF EXISTS file_objects_redistribution_policy_check;

ALTER TABLE file_objects
  ALTER COLUMN redistribution_policy SET DEFAULT 'unchecked';

ALTER TABLE file_objects
  ADD CONSTRAINT file_objects_redistribution_policy_check
  CHECK (redistribution_policy IN ('allowed', 'restricted', 'in_progress', 'unchecked', 'unknown', 'pending'))
  NOT VALID;
