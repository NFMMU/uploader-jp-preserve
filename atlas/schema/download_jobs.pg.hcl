table "download_jobs" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "file_object_id" {
    type = uuid
    null = false
  }
  column "selected_node_id" {
    type = uuid
    null = true
  }
  column "status" {
    type = varchar(16)
    null = false
    default = "queued"
  }
  column "access_token_hash" {
    type = varchar(64)
    null = false
  }
  column "lease_token" {
    type = uuid
    null = true
  }
  column "lease_until" {
    type = timestamptz
    null = true
  }
  column "attempt_count" {
    type = integer
    null = false
    default = 0
  }
  column "size_bytes" {
    type = bigint
    null = true
  }
  column "sha256" {
    type = varchar(64)
    null = true
  }
  column "last_error" {
    type = text
    null = true
  }
  column "expires_at" {
    type = timestamptz
    null = false
    default = sql("now() + interval '30 minutes'")
  }
  column "created_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }
  column "updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  foreign_key "download_jobs_file_object_id_fkey" {
    columns = [column.file_object_id]
    ref_columns = [table.file_objects.column.id]
    on_update = NO_ACTION
    on_delete = CASCADE
  }
  foreign_key "download_jobs_selected_node_id_fkey" {
    columns = [column.selected_node_id]
    ref_columns = [table.nodes.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  index "download_jobs_status_created_idx" {
    columns = [column.status, column.created_at]
  }
  index "download_jobs_file_idx" {
    columns = [column.file_object_id]
  }
  index "download_jobs_expiry_idx" {
    columns = [column.expires_at]
  }
  check "download_jobs_status_check" {
    expr = "status IN ('queued','running','ready','failed','expired')"
  }
  check "download_jobs_attempt_count_check" {
    expr = "attempt_count >= 0"
  }
}
