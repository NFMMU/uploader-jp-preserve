-- Move rows that still mean "not examined yet" to unchecked. This runs outside
-- a transaction (statement-level autocommit) so it does not hold the ACCESS
-- EXCLUSIVE lock taken by the DDL migration. The new CHECK already allows
-- unchecked and enforces it for concurrent inserts and updates.
--
-- pending is an old-only value: the new application never writes it, so every
-- pending row is converted regardless of cutover or policy history.
--
-- unknown can also be written by the new version after the cutover, so only
-- pre-cutover rows without a post-cutover policy-history entry are converted.
-- atlas:txmode none

UPDATE file_objects AS f
SET redistribution_policy = 'unchecked'
FROM redistribution_unchecked_cutover AS c
WHERE f.redistribution_policy = 'pending'
  OR (
    f.redistribution_policy = 'unknown'
    AND f.created_at < c.cutover_at
    AND NOT EXISTS (
      SELECT 1
      FROM policies AS p
      WHERE p.file_object_id = f.id
        AND p.created_at >= c.cutover_at
    )
  );
