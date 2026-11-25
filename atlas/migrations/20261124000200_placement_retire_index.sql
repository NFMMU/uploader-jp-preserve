-- atlas:txmode none
-- Build the marker index without blocking writes to placements.
CREATE INDEX CONCURRENTLY IF NOT EXISTS placements_retire_requested_idx
  ON placements (retire_requested_at)
  WHERE retire_requested_at IS NOT NULL;
