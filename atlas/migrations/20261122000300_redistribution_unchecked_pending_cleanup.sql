-- The final CHECK in 20261122000200 rejects any new pending writes. This
-- statement only catches pending rows written by the old application in the
-- short window between the backfill and that CHECK. It runs outside a
-- transaction so it does not hold the ACCESS EXCLUSIVE lock.
-- atlas:txmode none

UPDATE file_objects
SET redistribution_policy = 'unchecked'
WHERE redistribution_policy = 'pending';
