table "t_tasks" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "task_type" {
    type = varchar(32)
    null = false
  }
  column "file_object_id" {
    type = uuid
    null = false
  }
  column "source_node_id" {
    type = uuid
    null = true
  }
  column "target_node_id" {
    type = uuid
    null = true
  }
  column "status" {
    type = varchar(32)
    null = false
    default = "queued"
  }
  column "priority" {
    type = smallint
    null = false
    default = 3
  }
  column "attempt_count" {
    type = integer
    null = false
    default = 0
  }
  column "last_error" {
    type = text
    null = true
  }
  column "lease_until" {
    type = timestamptz
    null = true
  }
  column "lease_token" {
    type = uuid
    null = true
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
  foreign_key "tasks_file_object_id_fkey" {
    columns = [column.file_object_id]
    ref_columns = [table.m_file_objects.column.id]
    on_update = NO_ACTION
    on_delete = CASCADE
  }
  foreign_key "tasks_source_node_id_fkey" {
    columns = [column.source_node_id]
    ref_columns = [table.m_nodes.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  foreign_key "tasks_target_node_id_fkey" {
    columns = [column.target_node_id]
    ref_columns = [table.m_nodes.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  index "tasks_status_priority_created_idx" {
    columns = [column.status, column.priority, column.created_at]
  }
  index "tasks_lease_token_key" {
    unique = true
    columns = [column.lease_token]
  }
  index "tasks_source_node_status_idx" {
    columns = [column.source_node_id, column.status]
  }
  check "tasks_priority_check" {
    expr = "priority BETWEEN 0 AND 5"
  }
  check "tasks_attempt_count_check" {
    expr = "attempt_count >= 0"
  }
}