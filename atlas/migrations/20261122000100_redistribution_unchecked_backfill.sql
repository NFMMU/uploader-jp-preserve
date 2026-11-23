-- Move rows that still mean "not examined yet" to unchecked. This runs outside
-- a transaction (statement-level autocommit) so it does not hold the ACCESS
-- EXCLUSIVE lock taken by the DDL migration. The new CHECK already allows
-- unchecked and enforces it for concurrent inserts and updates.
-- atlas:txmode none

UPDATE file_objects
  SET redistribution_policy = 'unchecked'
  WHERE redistribution_policy IN ('unknown', 'pending');
