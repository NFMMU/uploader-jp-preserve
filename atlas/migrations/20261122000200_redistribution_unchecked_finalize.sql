-- Once the backfill has moved pending rows, replace the interim CHECK with the
-- final value set. Only DDL runs here; the replacement is NOT VALID so adding
-- it does not scan the table. The final CHECK rejects any new pending writes,
-- and 20261122000400 validates it separately.
ALTER TABLE file_objects
  DROP CONSTRAINT IF EXISTS file_objects_redistribution_policy_check;

ALTER TABLE file_objects
  ADD CONSTRAINT file_objects_redistribution_policy_check
  CHECK (redistribution_policy IN ('allowed', 'restricted', 'in_progress', 'unchecked', 'unknown'))
  NOT VALID;

DROP TABLE redistribution_unchecked_cutover;
