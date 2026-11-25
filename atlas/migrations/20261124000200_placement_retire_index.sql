-- atlas:txmode none
-- Build the marker index without blocking writes to placements.
-- If a failed concurrent build leaves an invalid index behind, drop it and
-- re-apply this migration:
--   DROP INDEX CONCURRENTLY IF EXISTS placements_retire_requested_idx;
CREATE INDEX CONCURRENTLY placements_retire_requested_idx
  ON placements (retire_requested_at)
  WHERE retire_requested_at IS NOT NULL;
