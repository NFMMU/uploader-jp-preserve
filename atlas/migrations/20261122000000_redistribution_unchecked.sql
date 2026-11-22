-- Replace the old "unknown"/"pending" defaults with an explicit unchecked
-- state. Existing rows that still meant "not examined yet" move to unchecked;
-- "unknown" remains available for a reviewed file whose terms are unclear.
BEGIN;

ALTER TABLE file_objects
  DROP CONSTRAINT IF EXISTS file_objects_redistribution_policy_check;

UPDATE file_objects
  SET redistribution_policy = 'unchecked'
  WHERE redistribution_policy IN ('unknown', 'pending');

ALTER TABLE file_objects
  ALTER COLUMN redistribution_policy SET DEFAULT 'unchecked';

ALTER TABLE file_objects
  ADD CONSTRAINT file_objects_redistribution_policy_check
  CHECK (redistribution_policy IN ('allowed', 'restricted', 'in_progress', 'unchecked', 'unknown'));

COMMIT;
