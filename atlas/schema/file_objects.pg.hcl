table "file_objects" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "artifact_id" {
    type = uuid
    null = false
  }
  column "original_filename" {
    type = varchar(500)
    null = false
  }
  column "size_bytes" {
    type = bigint
    null = false
    default = 0
  }
  column "sha256" {
    type = varchar(64)
    null = true
  }
  column "media_type" {
    type = varchar(255)
    null = true
  }
  column "source_url" {
    type = text
    null = true
  }
  column "source_password_encrypted" {
    type = text
    null = true
  }
  column "preservation_status" {
    type = varchar(32)
    null = false
    default = "uncollected"
  }
  column "redistribution_policy" {
    type = varchar(32)
    null = false
    default = "unknown"
  }
  column "representative" {
    type = uuid
    null = true
  }
  column "public_download_enabled" {
    type = boolean
    null = false
    default = false
  }
  column "created_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }
  column "deletion_requested_at" {
    type = timestamptz
    null = true
  }
  column "deleted_at" {
    type = timestamptz
    null = true
  }
  primary_key {
    columns = [column.id]
  }
  foreign_key "file_objects_artifact_id_fkey" {
    columns = [column.artifact_id]
    ref_columns = [table.artifacts.column.id]
    on_update = NO_ACTION
    on_delete = CASCADE
  }
  foreign_key "file_objects_representative_fkey" {
    columns = [column.representative]
    ref_columns = [table.users.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  index "file_objects_artifact_id_idx" {
    columns = [column.artifact_id]
  }
  index "file_objects_redistribution_policy_idx" {
    columns = [column.redistribution_policy]
  }
  index "file_objects_deleted_at_idx" {
    columns = [column.deleted_at]
  }
  check "file_objects_preservation_status_check" {
    expr = "preservation_status IN ('uncollected', 'queued', 'stored', 'verified', 'degraded', 'missing', 'error')"
  }
  check "file_objects_redistribution_policy_check" {
    expr = "redistribution_policy IN ('allowed', 'restricted', 'in_progress', 'pending', 'unknown')"
  }
  check "file_objects_size_bytes_check" {
    expr = "size_bytes >= 0"
  }
}