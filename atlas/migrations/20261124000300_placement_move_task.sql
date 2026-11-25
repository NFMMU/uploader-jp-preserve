-- Link a rebalance target placement to the replication task that fills it, so
-- a terminal failure can be recognised without relying on timestamps or on the
-- storage key (which may be reused from an earlier failed copy).
ALTER TABLE placements ADD COLUMN move_task_id uuid;
