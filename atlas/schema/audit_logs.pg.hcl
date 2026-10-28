table "audit_logs" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "actor_id" {
    type = uuid
    null = true
  }
  column "actor_label" {
    type = varchar(320)
    null = false
  }
  column "target_type" {
    type = varchar(64)
    null = false
  }
  column "target_id" {
    type = varchar(128)
    null = true
  }
  column "action" {
    type = varchar(100)
    null = false
  }
  column "result" {
    type = varchar(32)
    null = false
  }
  column "reason" {
    type = text
    null = true
  }
  column "metadata" {
    type = jsonb
    null = true
  }
  column "created_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  foreign_key "audit_logs_actor_id_fkey" {
    columns = [column.actor_id]
    ref_columns = [table.users.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  index "audit_logs_created_at_idx" {
    columns = [column.created_at]
  }
  index "audit_logs_target_idx" {
    columns = [column.target_type, column.target_id]
  }
}