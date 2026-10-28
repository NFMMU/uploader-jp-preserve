table "download_audit_logs" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "download_job_id" {
    type = uuid
    null = true
  }
  column "file_object_id" {
    type = uuid
    null = true
  }
  column "selected_node_id" {
    type = uuid
    null = true
  }
  column "action" {
    type = varchar(32)
    null = false
  }
  column "result" {
    type = varchar(16)
    null = false
  }
  column "reason" {
    type = text
    null = true
  }
  column "bytes_sent" {
    type = bigint
    null = false
    default = 0
  }
  column "duration_ms" {
    type = bigint
    null = false
    default = 0
  }
  column "created_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  foreign_key "download_audit_logs_download_job_id_fkey" {
    columns = [column.download_job_id]
    ref_columns = [table.download_jobs.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  foreign_key "download_audit_logs_file_object_id_fkey" {
    columns = [column.file_object_id]
    ref_columns = [table.file_objects.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  foreign_key "download_audit_logs_selected_node_id_fkey" {
    columns = [column.selected_node_id]
    ref_columns = [table.nodes.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  index "download_audit_logs_created_idx" {
    columns = [column.created_at]
  }
  index "download_audit_logs_job_idx" {
    columns = [column.download_job_id]
  }
}
