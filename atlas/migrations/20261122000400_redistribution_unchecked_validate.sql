-- Validate the replacement constraint in its own migration transaction.
-- VALIDATE CONSTRAINT takes SHARE UPDATE EXCLUSIVE, which permits normal reads
-- and writes while it scans the table.
ALTER TABLE file_objects
  VALIDATE CONSTRAINT file_objects_redistribution_policy_check;
