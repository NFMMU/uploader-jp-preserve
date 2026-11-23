-- Move rows that still mean "not examined yet" to unchecked. This runs outside
-- a transaction (statement-level autocommit) so it does not hold the ACCESS
-- EXCLUSIVE lock taken by the DDL migration. The new CHECK already allows
-- unchecked and enforces it for concurrent inserts and updates.
--
-- Only rows that existed before the migration cutover are converted. Rows
-- created by the new application version after the cutover already use the new
-- meaning of unknown and must not be rewritten. A policy-history row written
-- after the cutover also marks the file as reviewed by the new version.
-- atlas:txmode none

UPDATE file_objects AS f
SET redistribution_policy = 'unchecked'
FROM redistribution_unchecked_cutover AS c
WHERE f.redistribution_policy IN ('unknown', 'pending')
  AND f.created_at < c.cutover_at
  AND NOT EXISTS (
    SELECT 1
    FROM policies AS p
    WHERE p.file_object_id = f.id
      AND p.created_at >= c.cutover_at
  );
