-- Let authors replace files: track which new file supersedes an old one so
-- the old file can be physically deleted once the replacement is verified.
BEGIN;

ALTER TABLE file_objects ADD COLUMN replaced_by_file_object_id uuid;
ALTER TABLE file_objects ADD COLUMN replaced_by_user_id uuid;

ALTER TABLE file_objects
  ADD CONSTRAINT file_objects_replaced_by_file_object_id_fkey
  FOREIGN KEY (replaced_by_file_object_id) REFERENCES file_objects (id)
  ON UPDATE NO ACTION ON DELETE NO ACTION;

ALTER TABLE file_objects
  ADD CONSTRAINT file_objects_replaced_by_user_id_fkey
  FOREIGN KEY (replaced_by_user_id) REFERENCES users (id)
  ON UPDATE NO ACTION ON DELETE SET NULL;

CREATE INDEX file_objects_replaced_by_idx ON file_objects (replaced_by_file_object_id)
  WHERE replaced_by_file_object_id IS NOT NULL;

COMMIT;
