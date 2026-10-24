-- Track deletion requests separately from physical deletion completion.
-- A file remains auditable until all node agents report successful deletion.
ALTER TABLE file_objects
  ADD COLUMN deletion_requested_at timestamptz,
  ADD COLUMN deleted_at timestamptz;

CREATE INDEX file_objects_deleted_at_idx ON file_objects (deleted_at);
